package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"os"
)

const (
	previewMaxWidth    = 1920
	previewMaxHeight   = 1080
	previewJPEGQuality = 90
)

// PhotoPreview returns a JPEG data URL for a lightbox view of one photo.
// Stills are downloaded, fitted inside 1920x1080, and encoded in memory.
// The original temp file is removed before returning, and the preview itself
// is not written to the thumbnail cache. Videos reuse videoThumb, the
// existing 400px poster, because the lightbox has no video player.
func PhotoPreview(
	ctx context.Context,
	deviceID, photoID string,
	download func(ctx context.Context, destPath string) error,
	videoThumb func(ctx context.Context) (string, error),
) (string, error) {
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

	if isVideoFormat(format) {
		if videoThumb == nil {
			return "", errors.New("thumbnail function is required")
		}
		return videoThumb(ctx)
	}
	if download == nil {
		return "", errors.New("download function is required")
	}
	return generatePreview(ctx, format, download)
}

// generatePreview downloads the original to a temp file and removes that file
// on every exit, including a decode failure. Nothing is written to the cache.
func generatePreview(ctx context.Context, format string, download func(ctx context.Context, destPath string) error) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp("", "photobridge-preview-*")
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
	switch format {
	case "dng":
		img, orientation, err = decodeDNGPreview(ctx, srcPath)
	default:
		img, orientation, err = decodeThumbnailSource(srcPath, format)
	}
	if err != nil {
		return "", err
	}

	data, err := encodePreviewJPEG(fitUpright(img, orientation, true))
	if err != nil {
		return "", err
	}
	return jpegDataURL(data), nil
}

// fitPreviewBox is the largest size that fits inside 1920x1080.
func fitPreviewBox(w, h int) (int, int) {
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	if w <= previewMaxWidth && h <= previewMaxHeight {
		return w, h
	}
	// Width is the tighter limit when w/1920 >= h/1080.
	if int64(w)*previewMaxHeight >= int64(h)*previewMaxWidth {
		dh := h * previewMaxWidth / w
		if dh < 1 {
			dh = 1
		}
		if dh > previewMaxHeight {
			dh = previewMaxHeight
		}
		return previewMaxWidth, dh
	}
	dw := w * previewMaxHeight / h
	if dw < 1 {
		dw = 1
	}
	if dw > previewMaxWidth {
		dw = previewMaxWidth
	}
	return dw, previewMaxHeight
}

func encodePreviewJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: previewJPEGQuality}); err != nil {
		return nil, fmt.Errorf("encode preview: %w", err)
	}
	if buf.Len() == 0 {
		return nil, errors.New("encode preview: empty image")
	}
	return buf.Bytes(), nil
}
