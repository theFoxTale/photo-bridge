package backend

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
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
