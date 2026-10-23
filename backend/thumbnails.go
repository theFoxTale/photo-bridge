package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/sync/singleflight"
)

const (
	thumbnailMaxEdge     = 400
	thumbnailJPEGQuality = 85
)

// thumbnailFlights collapses concurrent previews of the same photo into one
// download. The frontend asks for every visible tile at once.
var thumbnailFlights singleflight.Group

// thumbnailCacheDir is %APPDATA%/PhotoBridge/thumbnails, or the user cache
// directory when APPDATA is unset.
func thumbnailCacheDir() (string, error) {
	root := strings.TrimSpace(os.Getenv("APPDATA"))
	if root == "" {
		var err error
		root, err = os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("thumbnail cache directory: %w", err)
		}
	}
	return filepath.Join(root, "PhotoBridge", "thumbnails"), nil
}

// sanitizeCacheSegment makes id safe as one path segment. Photo IDs are
// DCIM-relative paths, so a raw ID would create nested folders or leave the
// cache directory.
func sanitizeCacheSegment(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("must not be empty")
	}

	var b strings.Builder
	b.Grow(len(id))
	for _, r := range id {
		if r < 0x20 || strings.ContainsRune(`\/<>:"|?*`, r) {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}

	s := strings.TrimRight(b.String(), ". ")
	if s == "" || s == "." || s == ".." {
		return "", errors.New("invalid cache name")
	}
	if isReservedWindowsName(s) {
		s = "_" + s
	}
	return s, nil
}

// isReservedWindowsName reports device names Windows rejects as file names.
// The check uses the text before the first dot, which is the name Windows
// treats as reserved.
func isReservedWindowsName(name string) bool {
	base := name
	if i := strings.IndexByte(name, '.'); i >= 0 {
		base = name[:i]
	}
	switch strings.ToUpper(base) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}

// thumbnailPath is cacheDir/sanitizedDevice/sanitizedPhoto.jpg, and it is
// rejected when it would leave the cache directory.
func thumbnailPath(deviceID, photoID string) (string, error) {
	root, err := thumbnailCacheDir()
	if err != nil {
		return "", err
	}
	deviceSeg, err := sanitizeCacheSegment(deviceID)
	if err != nil {
		return "", fmt.Errorf("deviceID: %w", err)
	}
	photoSeg, err := sanitizeCacheSegment(photoID)
	if err != nil {
		return "", fmt.Errorf("photoID: %w", err)
	}

	full := filepath.Join(root, deviceSeg, photoSeg+".jpg")
	rel, err := filepath.Rel(root, full)
	if err != nil || !filepath.IsLocal(rel) {
		return "", errors.New("thumbnail path escapes cache directory")
	}
	return full, nil
}

// cachedThumbnail returns a JPEG data URL for photoID. A cache hit reads the
// stored file. A miss downloads the original, resizes it, and stores a JPEG.
// download writes the original bytes to destPath.
func cachedThumbnail(ctx context.Context, deviceID, photoID string, download func(ctx context.Context, destPath string) error) (string, error) {
	if ctx == nil {
		return "", errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateDeviceID(deviceID); err != nil {
		return "", err
	}
	if !validPhotoID(photoID) {
		return "", fmt.Errorf("invalid photoID %q", photoID)
	}
	format, ok := formatOf(photoID)
	if !ok {
		return "", fmt.Errorf("unsupported file type for %q", photoID)
	}
	if format != "jpeg" && format != "png" && format != "heic" {
		return "", fmt.Errorf("preview not available for %s files", format)
	}
	if download == nil {
		return "", errors.New("download function is required")
	}

	cachePath, err := thumbnailPath(deviceID, photoID)
	if err != nil {
		return "", err
	}
	if url, ok := readCachedThumbnail(cachePath); ok {
		return url, nil
	}

	key := deviceID + "\x00" + photoID
	v, err, _ := thumbnailFlights.Do(key, func() (any, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if url, ok := readCachedThumbnail(cachePath); ok {
			return url, nil
		}
		return generateThumbnail(ctx, cachePath, format, download)
	})
	if err != nil {
		return "", err
	}
	url, ok := v.(string)
	if !ok || url == "" {
		return "", errors.New("thumbnail generation returned no image")
	}
	return url, nil
}

func readCachedThumbnail(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 3 || data[0] != 0xFF || data[1] != 0xD8 {
		return "", false
	}
	return jpegDataURL(data), true
}

// generateThumbnail downloads the original to a temp file, decodes it, and
// writes the resized JPEG. The temp file is removed on every exit, including
// a decode failure.
func generateThumbnail(ctx context.Context, cachePath, format string, download func(ctx context.Context, destPath string) error) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp("", "photobridge-orig-*")
	if err != nil {
		return "", fmt.Errorf("create temporary file: %w", err)
	}
	srcPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(srcPath)
		return "", fmt.Errorf("create temporary file: %w", err)
	}
	defer func() { _ = os.Remove(srcPath) }()

	if err := download(ctx, srcPath); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	img, err := decodeThumbnailSource(srcPath, format)
	if err != nil {
		return "", err
	}
	data, err := encodeThumbnailJPEG(fitThumbnail(img))
	if err != nil {
		return "", err
	}
	if err := writeCacheAtomic(cachePath, data); err != nil {
		return "", err
	}
	return jpegDataURL(data), nil
}

func decodeThumbnailSource(path, format string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open downloaded file: %w", err)
	}
	defer f.Close()

	var img image.Image
	switch format {
	case "jpeg":
		img, err = jpeg.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("decode JPEG: %w", err)
		}
	case "png":
		img, err = png.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("decode PNG: %w", err)
		}
	case "heic":
		decoded, decErr := decodeHEIC(f)
		if decErr != nil {
			return nil, decErr
		}
		img = decoded.img
		if turns := decoded.rotations % 4; turns != 0 {
			img = rotateCounterClockwise(img, turns)
		}
	default:
		return nil, fmt.Errorf("preview not available for %s files", format)
	}
	if img == nil {
		return nil, errors.New("decoded image is empty")
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, errors.New("decoded image has no pixels")
	}
	return img, nil
}

// fitThumbnail scales src so both sides are at most thumbnailMaxEdge pixels.
// Smaller images are returned unchanged.
func fitThumbnail(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := fittedSize(w, h, thumbnailMaxEdge)
	if dw == w && dh == h {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

// fittedSize preserves the aspect ratio. The longer side becomes maxEdge
// when the source is larger; neither side grows when it is already smaller.
func fittedSize(w, h, maxEdge int) (int, int) {
	if w <= 0 || h <= 0 || maxEdge <= 0 {
		return 0, 0
	}
	if w <= maxEdge && h <= maxEdge {
		return w, h
	}
	if w >= h {
		dh := h * maxEdge / w
		if dh < 1 {
			dh = 1
		}
		return maxEdge, dh
	}
	dw := w * maxEdge / h
	if dw < 1 {
		dw = 1
	}
	return dw, maxEdge
}

func encodeThumbnailJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: thumbnailJPEGQuality}); err != nil {
		return nil, fmt.Errorf("encode thumbnail: %w", err)
	}
	if buf.Len() == 0 {
		return nil, errors.New("encode thumbnail: empty image")
	}
	return buf.Bytes(), nil
}

// writeCacheAtomic writes data to a temp file in the cache folder and renames
// it into place, so a reader never opens a half-written JPEG.
func writeCacheAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create thumbnail cache: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".thumb-*.jpg")
	if err != nil {
		return fmt.Errorf("create thumbnail cache: %w", err)
	}
	tmpName := tmp.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write thumbnail: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write thumbnail: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		// Windows refuses to rename over an existing file. A lost race can
		// leave a previous JPEG in place; replace it with this one.
		_ = os.Remove(path)
		if err2 := os.Rename(tmpName, path); err2 != nil {
			return fmt.Errorf("write thumbnail: %w", err2)
		}
	}
	success = true
	return nil
}

func jpegDataURL(data []byte) string {
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
}
