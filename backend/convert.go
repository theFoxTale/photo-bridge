package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"os/exec"
	"strconv"
)

// heicImage is a decoded HEIC photo.
type heicImage struct {
	img image.Image
	// rotations is the number of 90 degree counter-clockwise turns the file
	// asks for (the HEIF "irot" property). The decoder does not apply them.
	rotations int
	// exif is the raw EXIF block as stored in the file, nil when absent.
	exif []byte
}

type imageKind int

const (
	kindUnknown imageKind = iota
	kindJPEG
	kindHEIF
)

// sniffKind recognises a file by its content rather than its extension.
func sniffKind(head []byte) imageKind {
	switch {
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF:
		return kindJPEG
	case len(head) >= 12 && string(head[4:8]) == "ftyp":
		return kindHEIF
	default:
		return kindUnknown
	}
}

// ffmpegJPEGQuality maps the app's 1-100 slider onto ffmpeg's -q:v scale.
// 100 becomes 2 (excellent) and 1 becomes 31 (smallest).
func ffmpegJPEGQuality(quality int) (int, error) {
	if quality < 1 || quality > 100 {
		return 0, fmt.Errorf("JPG quality must be between 1 and 100, got %d", quality)
	}
	return 2 + (100-quality)*29/99, nil
}

// convertDNGToJPEG writes srcPath, a DNG, to dstPath as a JPEG.
// -map_metadata 0 keeps the source metadata. The DNG is not decoded in Go.
func convertDNGToJPEG(ctx context.Context, srcPath, dstPath string, quality int) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	qv, err := ffmpegJPEGQuality(quality)
	if err != nil {
		return err
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			logFFmpegMissing()
		}
		return fmt.Errorf("ffmpeg not available: %w", err)
	}

	err = runFFmpeg(ctx, ffmpeg,
		"-y", "-nostdin",
		"-i", srcPath,
		"-map_metadata", "0",
		"-q:v", strconv.Itoa(qv),
		dstPath,
	)
	if err != nil {
		_ = os.Remove(dstPath)
		return fmt.Errorf("convert DNG to JPG: %w", err)
	}
	if !jpegFileReady(dstPath) {
		_ = os.Remove(dstPath)
		return errors.New("convert DNG to JPG: ffmpeg wrote no JPEG")
	}
	return nil
}

// convertToJPEG writes srcPath (a HEIC or JPEG file) to dstPath as a JPEG,
// keeping the EXIF metadata. A JPEG source is copied byte for byte at quality
// 100 and re-encoded below that.
func convertToJPEG(srcPath, dstPath string, quality int) error {
	if quality < 1 || quality > 100 {
		return fmt.Errorf("JPG quality must be between 1 and 100, got %d", quality)
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open downloaded file: %w", err)
	}
	defer f.Close()

	head := make([]byte, 12)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("read downloaded file: %w", err)
	}

	switch sniffKind(head[:n]) {
	case kindJPEG:
		if quality == 100 {
			return copyFile(srcPath, dstPath)
		}
		return reencodeJPEG(f, dstPath, quality)
	case kindHEIF:
		return convertHEIC(f, dstPath, quality)
	default:
		return errors.New("file is not a HEIC or JPEG image")
	}
}

func reencodeJPEG(f *os.File, dstPath string, quality int) error {
	data, err := io.ReadAll(io.NewSectionReader(f, 0, 1<<62))
	if err != nil {
		return fmt.Errorf("read JPEG: %w", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode JPEG: %w", err)
	}
	rawExif, err := jpegExif(data)
	if err != nil {
		return err
	}
	// Pixels are kept as stored, so the original orientation tag stays valid.
	return writeJPEG(dstPath, img, quality, rawExif)
}

func convertHEIC(f *os.File, dstPath string, quality int) error {
	decoded, err := decodeHEIC(f)
	if err != nil {
		return err
	}
	rawExif, err := normalizeExif(decoded.exif)
	if err != nil {
		return err
	}

	img := decoded.img
	if turns := decoded.rotations % 4; turns != 0 {
		img = rotateCounterClockwise(img, turns)
		if rawExif != nil {
			// The pixels are upright now; a leftover orientation tag would
			// make viewers rotate the picture a second time.
			if err := setExifOrientation(rawExif, 1); err != nil {
				return err
			}
		}
	}
	return writeJPEG(dstPath, img, quality, rawExif)
}

// writeJPEG encodes img, adds rawExif (if any) and writes the result to dstPath.
func writeJPEG(dstPath string, img image.Image, quality int, rawExif []byte) error {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return fmt.Errorf("encode JPEG: %w", err)
	}
	data := buf.Bytes()
	if rawExif != nil {
		var err error
		if data, err = insertExif(data, rawExif); err != nil {
			return err
		}
	}
	if err := os.WriteFile(dstPath, data, 0o644); err != nil {
		_ = os.Remove(dstPath)
		return fmt.Errorf("write %s: %w", dstPath, err)
	}
	return nil
}

// rotateCounterClockwise turns img by turns*90 degrees counter-clockwise.
// turns must be 1, 2 or 3.
func rotateCounterClockwise(src image.Image, turns int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	dw, dh := w, h
	if turns%2 == 1 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))

	for dy := 0; dy < dh; dy++ {
		for dx := 0; dx < dw; dx++ {
			var sx, sy int
			switch turns {
			case 1:
				sx, sy = w-1-dy, dx
			case 2:
				sx, sy = w-1-dx, h-1-dy
			default:
				sx, sy = dy, h-1-dx
			}
			dst.Set(dx, dy, src.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return dst
}

// applyExifOrientation turns stored pixels into the orientation a viewer
// should show. 1 and any unknown value leave the image unchanged.
// 6 is 90 degrees clockwise, 8 is 270 degrees clockwise, and 3 is 180.
func applyExifOrientation(src image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return flipHorizontal(src)
	case 3:
		return rotateCounterClockwise(src, 2)
	case 4:
		return flipVertical(src)
	case 5:
		return transpose(src)
	case 6:
		return rotateCounterClockwise(src, 3)
	case 7:
		return transverse(src)
	case 8:
		return rotateCounterClockwise(src, 1)
	default:
		return src
	}
}

func flipHorizontal(src image.Image) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(b.Min.X+w-1-x, b.Min.Y+y))
		}
	}
	return dst
}

func flipVertical(src image.Image) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(b.Min.X+x, b.Min.Y+h-1-y))
		}
	}
	return dst
}

// transpose maps stored (x, y) onto visual (y, x). That is EXIF orientation 5.
func transpose(src image.Image) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < w; y++ {
		for x := 0; x < h; x++ {
			dst.Set(x, y, src.At(b.Min.X+y, b.Min.Y+x))
		}
	}
	return dst
}

// transverse is EXIF orientation 7: the stored top row is the visual right side.
func transverse(src image.Image) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < w; y++ {
		for x := 0; x < h; x++ {
			dst.Set(x, y, src.At(b.Min.X+w-1-y, b.Min.Y+h-1-x))
		}
	}
	return dst
}

// copyFile copies src to dst, replacing dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("copy to %s: %w", dst, err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}
