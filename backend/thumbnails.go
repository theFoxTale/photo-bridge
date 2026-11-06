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
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/sync/singleflight"
)

const (
	thumbnailMaxEdge     = 400
	thumbnailJPEGQuality = 85

	// thumbnailCacheSuffix versions the cached JPEG. v2 is the first
	// generation whose pixels are already upright, so a WebView does not
	// have to honour EXIF. Older files are named .jpg and are left unused.
	thumbnailCacheSuffix = ".v2.jpg"

	ffmpegMissingWarning = "FFmpeg not found. Video thumbnails will be unavailable."
)

// ffmpegMissingLogged keeps the missing-ffmpeg warning to a single line.
var ffmpegMissingLogged atomic.Bool

// thumbnailFlights collapses concurrent previews of the same photo into one
// download. The frontend asks for every visible tile at once.
var thumbnailFlights singleflight.Group

// thumbnailCacheDir is the configured cache folder, or
// %APPDATA%/PhotoBridge/thumbnails when the config does not set one.
func thumbnailCacheDir() (string, error) {
	return ResolveCacheDir()
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

// thumbnailPath is cacheDir/sanitizedDevice/sanitizedPhoto.v2.jpg, and it is
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

	full := filepath.Join(root, deviceSeg, photoSeg+thumbnailCacheSuffix)
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
	if format != "jpeg" && format != "png" && format != "heic" && format != "dng" && !isVideoFormat(format) {
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

	var img image.Image
	var orientation int
	switch {
	case isVideoFormat(format):
		img, err = decodeVideoPoster(ctx, srcPath)
		orientation = 1
	case format == "dng":
		img, orientation, err = decodeDNGPreview(ctx, srcPath)
	default:
		img, orientation, err = decodeThumbnailSource(srcPath, format)
	}
	if err != nil {
		return "", err
	}
	data, err := encodeThumbnailJPEG(fitUpright(img, orientation, false))
	if err != nil {
		return "", err
	}
	if err := writeCacheAtomic(cachePath, data); err != nil {
		return "", err
	}
	return jpegDataURL(data), nil
}

// logFFmpegMissing records that the ffmpeg binary is not on PATH. Later
// videos fail the same way without repeating the warning.
func logFFmpegMissing() {
	if ffmpegMissingLogged.CompareAndSwap(false, true) {
		slog.Warn(ffmpegMissingWarning)
	}
}

func contextDone(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// decodeVideoPoster extracts one JPEG frame with ffmpeg and decodes that
// frame. The video file itself is never passed to an image decoder. The frame
// at one second is preferred; a shorter clip is tried from the start.
func decodeVideoPoster(ctx context.Context, srcPath string) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			logFFmpegMissing()
		}
		return nil, fmt.Errorf("ffmpeg not available: %w", err)
	}

	poster, err := os.CreateTemp("", "photobridge-poster-*.jpg")
	if err != nil {
		return nil, fmt.Errorf("create temporary frame: %w", err)
	}
	posterPath := poster.Name()
	if err := poster.Close(); err != nil {
		_ = os.Remove(posterPath)
		return nil, fmt.Errorf("create temporary frame: %w", err)
	}
	defer func() { _ = os.Remove(posterPath) }()

	err = extractVideoFrame(ctx, ffmpeg, srcPath, posterPath, "00:00:01")
	if err != nil {
		if contextDone(err) || errors.Is(err, exec.ErrNotFound) {
			return nil, err
		}
		if err2 := extractVideoFrame(ctx, ffmpeg, srcPath, posterPath, "00:00:00"); err2 != nil {
			return nil, err2
		}
	}
	return decodeJPEGFile(posterPath)
}

// extractVideoFrame runs:
//
//	ffmpeg -y -nostdin -i <src> -ss <timestamp> -vframes 1 -f image2 <dst>
//
// A missing binary is exec.ErrNotFound (the process never starts). A non-zero
// exit after ffmpeg has started is exec.ExitError, wrapped with ffmpeg's message.
func extractVideoFrame(ctx context.Context, ffmpegPath, srcPath, dstPath, timestamp string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := runFFmpeg(ctx, ffmpegPath,
		"-y", "-nostdin",
		"-i", srcPath,
		"-ss", timestamp,
		"-vframes", "1",
		"-f", "image2",
		dstPath,
	); err != nil {
		return fmt.Errorf("extract video frame at %s: %w", timestamp, err)
	}
	if !jpegFileReady(dstPath) {
		return fmt.Errorf("extract video frame at %s: ffmpeg wrote no JPEG", timestamp)
	}
	return nil
}

// decodeDNGPreview extracts one JPEG from a DNG with ffmpeg and decodes that
// JPEG. The DNG itself is never passed to an image decoder. The returned
// orientation is applied by fitUpright after the bitmap has been scaled.
func decodeDNGPreview(ctx context.Context, srcPath string) (image.Image, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 1, err
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			logFFmpegMissing()
		}
		return nil, 1, fmt.Errorf("ffmpeg not available: %w", err)
	}

	poster, err := os.CreateTemp("", "photobridge-dng-*.jpg")
	if err != nil {
		return nil, 1, fmt.Errorf("create temporary frame: %w", err)
	}
	posterPath := poster.Name()
	if err := poster.Close(); err != nil {
		_ = os.Remove(posterPath)
		return nil, 1, fmt.Errorf("create temporary frame: %w", err)
	}
	defer func() { _ = os.Remove(posterPath) }()

	if err := runFFmpeg(ctx, ffmpeg,
		"-y", "-nostdin",
		"-i", srcPath,
		"-vframes", "1",
		"-f", "image2",
		posterPath,
	); err != nil {
		return nil, 1, fmt.Errorf("extract DNG preview: %w", err)
	}
	if !jpegFileReady(posterPath) {
		return nil, 1, errors.New("extract DNG preview: ffmpeg wrote no JPEG")
	}
	// ffmpeg's autorotate writes an upright JPEG and drops the orientation
	// tag. A tag that is still present describes pixels that were not turned.
	return decodeOrientedJPEGFile(posterPath)
}

// runFFmpeg runs ffmpeg with args. A missing binary is exec.ErrNotFound and
// is logged once. A non-zero exit after ffmpeg has started is exec.ExitError.
func runFFmpeg(ctx context.Context, ffmpegPath string, args ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	hideFFmpegWindow(cmd)
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			logFFmpegMissing()
		}
		detail := strings.TrimSpace(stderr.String())
		if len(detail) > 512 {
			detail = detail[:512]
		}
		if detail != "" {
			return fmt.Errorf("%w (%s)", err, detail)
		}
		return err
	}
	return nil
}

func jpegFileReady(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var magic [3]byte
	n, err := f.Read(magic[:])
	return err == nil && n == 3 && magic[0] == 0xFF && magic[1] == 0xD8
}

func decodeJPEGFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open extracted frame: %w", err)
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode extracted frame: %w", err)
	}
	if img == nil {
		return nil, errors.New("extracted frame is empty")
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, errors.New("extracted frame has no pixels")
	}
	return img, nil
}

// decodeOrientedJPEGFile decodes a JPEG and reports its EXIF orientation.
// Video posters stay on decodeJPEGFile, which does not read EXIF. The pixels
// are left as stored so the caller can scale them before turning them.
func decodeOrientedJPEGFile(path string) (image.Image, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 1, fmt.Errorf("open extracted frame: %w", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 1, fmt.Errorf("decode extracted frame: %w", err)
	}
	if img == nil {
		return nil, 1, errors.New("extracted frame is empty")
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, 1, errors.New("extracted frame has no pixels")
	}
	return img, jpegOrientation(data), nil
}

func validImage(img image.Image) (image.Image, error) {
	if img == nil {
		return nil, errors.New("decoded image is empty")
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, errors.New("decoded image has no pixels")
	}
	return img, nil
}

func decodeThumbnailSource(path, format string) (image.Image, int, error) {
	switch format {
	case "jpeg", "png":
		return decodeOrientedStill(path, format)
	case "heic":
		return decodeOrientedHEIC(path)
	default:
		return nil, 1, fmt.Errorf("preview not available for %s files", format)
	}
}

// decodeOrientedStill decodes a JPEG or PNG and reports its EXIF orientation.
// Pixels stay in stored order. The JPEG written afterwards has no EXIF
// segment, so the tag cannot turn the picture a second time.
func decodeOrientedStill(path, format string) (image.Image, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 1, fmt.Errorf("open downloaded file: %w", err)
	}
	var img image.Image
	var orientation int
	switch format {
	case "jpeg":
		img, err = jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, 1, fmt.Errorf("decode JPEG: %w", err)
		}
		orientation = jpegOrientation(data)
	case "png":
		img, err = png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, 1, fmt.Errorf("decode PNG: %w", err)
		}
		orientation = pngOrientation(data)
	default:
		return nil, 1, fmt.Errorf("preview not available for %s files", format)
	}
	img, err = validImage(img)
	if err != nil {
		return nil, 1, err
	}
	return img, orientation, nil
}

// decodeOrientedHEIC reports the HEIF irot value as an EXIF orientation.
// EXIF is used only when irot is 0, because a file that stores both describes
// one turn, not two. The pixels are not turned here.
func decodeOrientedHEIC(path string) (image.Image, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 1, fmt.Errorf("open downloaded file: %w", err)
	}
	defer f.Close()

	decoded, err := decodeHEIC(f)
	if err != nil {
		return nil, 1, err
	}
	img, err := validImage(decoded.img)
	if err != nil {
		return nil, 1, err
	}
	if turns := decoded.rotations % 4; turns != 0 {
		return img, orientationFromIrot(turns), nil
	}
	return img, exifBlockOrientation(decoded.exif), nil
}

// fitUpright scales src so the visual picture fits the thumbnail or the
// lightbox box, then applies orientation. The turn happens after the scale,
// so a 12 megapixel photo is not rotated pixel by pixel.
func fitUpright(src image.Image, orientation int, preview bool) image.Image {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	vw, vh := visualSize(sw, sh, orientation)
	if vw <= 0 || vh <= 0 {
		return src
	}
	var fw, fh int
	if preview {
		fw, fh = fitPreviewBox(vw, vh)
	} else {
		fw, fh = fittedSize(vw, vh, thumbnailMaxEdge)
	}
	if fw <= 0 || fh <= 0 {
		return applyExifOrientation(src, orientation)
	}
	pw, ph := fw, fh
	if orientation >= 5 && orientation <= 8 {
		pw, ph = fh, fw
	}
	scaled := src
	if pw != sw || ph != sh {
		dst := image.NewRGBA(image.Rect(0, 0, pw, ph))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
		scaled = dst
	}
	return applyExifOrientation(scaled, orientation)
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
