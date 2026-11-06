package backend

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

func TestFitPreviewBox(t *testing.T) {
	cases := []struct {
		name   string
		w, h   int
		dw, dh int
	}{
		{name: "already inside", w: 800, h: 600, dw: 800, dh: 600},
		{name: "exact box", w: 1920, h: 1080, dw: 1920, dh: 1080},
		{name: "landscape photo", w: 4032, h: 3024, dw: 1440, dh: 1080},
		{name: "portrait photo", w: 3024, h: 4032, dw: 810, dh: 1080},
		{name: "panorama", w: 8000, h: 1000, dw: 1920, dh: 240},
		{name: "tall strip", w: 100, h: 2000, dw: 54, dh: 1080},
		{name: "empty", w: 0, h: 100, dw: 0, dh: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dw, dh := fitPreviewBox(tc.w, tc.h)
			if dw != tc.dw || dh != tc.dh {
				t.Fatalf("fitPreviewBox(%d, %d) = %d x %d, want %d x %d", tc.w, tc.h, dw, dh, tc.dw, tc.dh)
			}
			if dw > previewMaxWidth || dh > previewMaxHeight {
				t.Fatalf("result %d x %d is outside 1920x1080", dw, dh)
			}
		})
	}
}

func TestPhotoPreviewFitsJPEGAndRemovesTempFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)

	payload := pngBytes(t, 2000, 1500)
	var srcPath string
	download := func(_ context.Context, dest string) error {
		srcPath = dest
		return os.WriteFile(dest, payload, 0o644)
	}
	thumbCalls := 0
	videoThumb := func(context.Context) (string, error) {
		thumbCalls++
		return "data:image/jpeg;base64,thumb", nil
	}

	url, err := PhotoPreview(context.Background(), "device-1", "100APPLE/IMG_0001.PNG", download, videoThumb)
	if err != nil {
		t.Fatal(err)
	}
	if thumbCalls != 0 {
		t.Fatalf("video thumbnail calls = %d, want 0 for a still", thumbCalls)
	}
	if srcPath == "" {
		t.Fatal("download was not called")
	}
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Fatalf("temp file %s still exists: %v", srcPath, err)
	}

	img := decodeJPEGDataURL(t, url)
	b := img.Bounds()
	if b.Dx() != 1440 || b.Dy() != 1080 {
		t.Fatalf("preview = %d x %d, want 1440 x 1080", b.Dx(), b.Dy())
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("preview wrote %d cache entries, want none", len(entries))
	}
}

func TestPhotoPreviewRotatesExifOrientation(t *testing.T) {
	payload := orientedJPEG(t, 2000, 1000, 6, nil)
	download := func(_ context.Context, dest string) error {
		return os.WriteFile(dest, payload, 0o644)
	}
	url, err := PhotoPreview(context.Background(), "device-1", "100APPLE/IMG_0001.JPG", download, nil)
	if err != nil {
		t.Fatal(err)
	}
	img := decodeJPEGDataURL(t, url)
	b := img.Bounds()
	if b.Dx() != 540 || b.Dy() != 1080 {
		t.Fatalf("preview = %d x %d, want 540 x 1080", b.Dx(), b.Dy())
	}
	const prefix = "data:image/jpeg;base64,"
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, prefix))
	if err != nil {
		t.Fatal(err)
	}
	block, err := jpegExif(raw)
	if err != nil {
		t.Fatal(err)
	}
	if block != nil {
		t.Fatal("preview JPEG still carries an EXIF orientation tag")
	}
}

func TestPhotoPreviewVideoUsesThumbnail(t *testing.T) {
	downloads := 0
	download := func(context.Context, string) error {
		downloads++
		return nil
	}
	url, err := PhotoPreview(
		context.Background(),
		"device-1",
		"100APPLE/CLIP.MP4",
		download,
		func(context.Context) (string, error) {
			return "data:image/jpeg;base64,poster", nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if downloads != 0 {
		t.Fatalf("downloads = %d, want 0 for a video", downloads)
	}
	if url != "data:image/jpeg;base64,poster" {
		t.Fatalf("url = %q", url)
	}
}

func TestPhotoPreviewCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := PhotoPreview(ctx, "device-1", "100APPLE/IMG_0001.JPG", func(context.Context, string) error {
		t.Fatal("download ran after cancel")
		return nil
	}, nil)
	if err == nil {
		t.Fatal("expected a canceled context to fail")
	}
}

func TestPhotoPreviewRejectsBadID(t *testing.T) {
	_, err := PhotoPreview(context.Background(), "device-1", `..\IMG_0001.JPG`, func(context.Context, string) error {
		t.Fatal("download ran for a bad id")
		return nil
	}, nil)
	if err == nil {
		t.Fatal("expected an invalid photo id to fail")
	}
}
