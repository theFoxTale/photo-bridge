package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/image/tiff"
)

func TestCachedThumbnailPNGRoundTrip(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)

	payload := pngBytes(t, 800, 200)
	var calls atomic.Int32
	var srcPath string
	download := func(_ context.Context, dest string) error {
		calls.Add(1)
		srcPath = dest
		return os.WriteFile(dest, payload, 0o644)
	}

	const deviceID = "device-1"
	const photoID = "100APPLE/IMG_0001.PNG"
	url, err := cachedThumbnail(context.Background(), deviceID, photoID, download)
	if err != nil {
		t.Fatalf("cachedThumbnail: %v", err)
	}

	img := decodeJPEGDataURL(t, url)
	if got := img.Bounds().Size(); got.X != 400 || got.Y != 100 {
		t.Fatalf("size = %v, want 400x100", got)
	}
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Fatalf("temp original %q still present: %v", srcPath, err)
	}

	cacheFile := filepath.Join(root, "PhotoBridge", "thumbnails", "device-1", "100APPLE_IMG_0001.PNG.jpg")
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("cache file: %v", err)
	}

	url2, err := cachedThumbnail(context.Background(), deviceID, photoID, download)
	if err != nil {
		t.Fatalf("cached read: %v", err)
	}
	if url2 != url {
		t.Fatal("cached thumbnail differs from the generated one")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("download calls = %d, want 1", got)
	}
}

func TestCachedThumbnailDoesNotUpscale(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	payload := pngBytes(t, 40, 50)
	download := func(_ context.Context, dest string) error {
		return os.WriteFile(dest, payload, 0o644)
	}

	url, err := cachedThumbnail(context.Background(), "device-1", "IMG_SMALL.PNG", download)
	if err != nil {
		t.Fatal(err)
	}
	img := decodeJPEGDataURL(t, url)
	if got := img.Bounds().Size(); got.X != 40 || got.Y != 50 {
		t.Fatalf("size = %v, want 40x50", got)
	}
}

func TestCachedThumbnailRemovesTempOnDecodeError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)

	var srcPath string
	download := func(_ context.Context, dest string) error {
		srcPath = dest
		return os.WriteFile(dest, []byte("not a png"), 0o644)
	}

	_, err := cachedThumbnail(context.Background(), "device-1", "100APPLE/IMG_BAD.PNG", download)
	if err == nil {
		t.Fatal("expected a decode error")
	}
	if _, statErr := os.Stat(srcPath); !os.IsNotExist(statErr) {
		t.Fatalf("temp original %q still present: %v", srcPath, statErr)
	}

	cacheDir := filepath.Join(root, "PhotoBridge", "thumbnails")
	entries, readErr := os.ReadDir(cacheDir)
	if readErr == nil && len(entries) != 0 {
		t.Fatalf("cache dir has entries after a failed decode: %d", len(entries))
	}
}

func TestCachedThumbnailSingleflight(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	payload := pngBytes(t, 500, 300)

	var calls atomic.Int32
	var arrived atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	var closeRelease sync.Once
	t.Cleanup(func() { closeRelease.Do(func() { close(release) }) })

	download := func(_ context.Context, dest string) error {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return os.WriteFile(dest, payload, 0o644)
	}

	const deviceID = "device-sf"
	const photoID = "100APPLE/IMG_0002.PNG"
	errCh := make(chan error, 2)
	call := func() {
		arrived.Add(1)
		_, err := cachedThumbnail(context.Background(), deviceID, photoID, download)
		errCh <- err
	}

	go call()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("download did not start")
	}
	go call()

	deadline := time.Now().Add(5 * time.Second)
	for arrived.Load() < 2 {
		if time.Now().After(deadline) {
			t.Fatal("second call did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Fatalf("downloads while in flight = %d, want 1", got)
	}

	closeRelease.Do(func() { close(release) })
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("call did not finish")
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("downloads = %d, want 1", got)
	}
}

func TestCachedThumbnailRejectsUnsupported(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	var calls atomic.Int32
	download := func(context.Context, string) error {
		calls.Add(1)
		return nil
	}

	_, err := cachedThumbnail(context.Background(), "device-1", "100APPLE/IMG_0001.AAE", download)
	if err == nil {
		t.Fatal("cachedThumbnail(AAE) succeeded")
	}
	if calls.Load() != 0 {
		t.Fatalf("download calls = %d, want 0", calls.Load())
	}
}

func TestCachedThumbnailCancelledContext(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var calls atomic.Int32
	_, err := cachedThumbnail(ctx, "device-1", "IMG_0001.PNG", func(context.Context, string) error {
		calls.Add(1)
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("download calls = %d, want 0", calls.Load())
	}
}

func TestThumbnailPathStaysInCache(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)

	got, err := thumbnailPath(`..\..\Windows`, `100APPLE/..\IMG_0001.JPG`)
	if err != nil {
		t.Fatal(err)
	}
	cacheRoot, err := thumbnailCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(cacheRoot, got)
	if err != nil || !filepath.IsLocal(rel) {
		t.Fatalf("path %q escaped %q (rel %q, err %v)", got, cacheRoot, rel, err)
	}

	if _, err := thumbnailPath("..", "IMG_0001.JPG"); err == nil {
		t.Fatal("expected deviceID .. to be rejected")
	}
	if got, err := sanitizeCacheSegment("CON"); err != nil || got != "_CON" {
		t.Fatalf("sanitize CON = %q, %v", got, err)
	}
}

func TestCachedThumbnailVideoFrame(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	src := filepath.Join(t.TempDir(), "clip.mp4")
	writeTestVideo(t, src, "red", 800, 400, 2)

	var srcPath string
	download := func(_ context.Context, dest string) error {
		srcPath = dest
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	}

	url, err := cachedThumbnail(context.Background(), "device-1", "100APPLE/CLIP.MP4", download)
	if err != nil {
		t.Fatal(err)
	}
	img := decodeJPEGDataURL(t, url)
	if got := img.Bounds().Size(); got.X != 400 || got.Y != 200 {
		t.Fatalf("size = %v, want 400x200", got)
	}
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Fatalf("temp original %q still present: %v", srcPath, err)
	}
	cacheFile := filepath.Join(root, "PhotoBridge", "thumbnails", "device-1", "100APPLE_CLIP.MP4.jpg")
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("cache file: %v", err)
	}
}

func TestCachedThumbnailShortVideoFallsBackToStart(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	dir := t.TempDir()
	src := filepath.Join(dir, "short.mov")
	writeTestVideo(t, src, "blue", 200, 100, 0.2)

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	late := filepath.Join(dir, "late.jpg")
	if err := extractVideoFrame(context.Background(), ffmpeg, src, late, "00:00:01"); err == nil {
		t.Fatal("a 0.2s clip should not yield a frame at 1s")
	}

	download := func(_ context.Context, dest string) error {
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	}
	url, err := cachedThumbnail(context.Background(), "device-1", "100APPLE/SHORT.MOV", download)
	if err != nil {
		t.Fatal(err)
	}
	img := decodeJPEGDataURL(t, url)
	if got := img.Bounds().Size(); got.X != 200 || got.Y != 100 {
		t.Fatalf("size = %v, want 200x100", got)
	}
}

func TestCachedThumbnailVideoGarbageLeavesNoTemp(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	var srcPath string
	download := func(_ context.Context, dest string) error {
		srcPath = dest
		return os.WriteFile(dest, []byte("not a video"), 0o644)
	}
	_, err := cachedThumbnail(context.Background(), "device-1", "100APPLE/BAD.MP4", download)
	if err == nil {
		t.Fatal("expected ffmpeg to reject the file")
	}
	if _, statErr := os.Stat(srcPath); !os.IsNotExist(statErr) {
		t.Fatalf("temp original %q still present: %v", srcPath, statErr)
	}
	cacheDir := filepath.Join(root, "PhotoBridge", "thumbnails")
	entries, readErr := os.ReadDir(cacheDir)
	if readErr == nil && len(entries) != 0 {
		t.Fatalf("cache dir has entries after a failed video preview: %d", len(entries))
	}
}

func TestExtractVideoFrameLogsWhenBinaryMissing(t *testing.T) {
	ffmpegMissingLogged.Store(false)
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	err := extractVideoFrame(context.Background(), "ffmpeg-does-not-exist", "in.mov", filepath.Join(t.TempDir(), "out.jpg"), "00:00:01")
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if !strings.Contains(buf.String(), ffmpegMissingWarning) {
		t.Fatalf("log = %q", buf.String())
	}

	buf.Reset()
	_ = extractVideoFrame(context.Background(), "ffmpeg-does-not-exist", "in.mov", filepath.Join(t.TempDir(), "out.jpg"), "00:00:00")
	if buf.Len() != 0 {
		t.Fatalf("warning logged twice: %s", buf.String())
	}
}

func TestExtractVideoFrameExitErrorIsNotMissing(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffmpegMissingLogged.Store(false)
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	err = extractVideoFrame(context.Background(), ffmpeg, filepath.Join(t.TempDir(), "missing.mov"), filepath.Join(t.TempDir(), "out.jpg"), "00:00:01")
	if err == nil {
		t.Fatal("expected ffmpeg to fail")
	}
	if errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("err = %v, ffmpeg should have started", err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %T %v, want ExitError", err, err)
	}
	if strings.Contains(buf.String(), ffmpegMissingWarning) {
		t.Fatalf("log = %q", buf.String())
	}
}

func TestCachedThumbnailDNG(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	payload := testDNGBytes(t, 800, 400)

	var srcPath string
	download := func(_ context.Context, dest string) error {
		srcPath = dest
		return os.WriteFile(dest, payload, 0o644)
	}
	url, err := cachedThumbnail(context.Background(), "device-1", "100APPLE/IMG_0001.DNG", download)
	if err != nil {
		t.Fatal(err)
	}
	img := decodeJPEGDataURL(t, url)
	if got := img.Bounds().Size(); got.X != 400 || got.Y != 200 {
		t.Fatalf("size = %v, want 400x200", got)
	}
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Fatalf("temp original %q still present: %v", srcPath, err)
	}
	cacheFile := filepath.Join(root, "PhotoBridge", "thumbnails", "device-1", "100APPLE_IMG_0001.DNG.jpg")
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("cache file: %v", err)
	}
}

func TestCachedThumbnailCorruptDNG(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)

	var srcPath string
	download := func(_ context.Context, dest string) error {
		srcPath = dest
		return os.WriteFile(dest, []byte("not a dng"), 0o644)
	}
	_, err := cachedThumbnail(context.Background(), "device-1", "100APPLE/BAD.DNG", download)
	if err == nil {
		t.Fatal("expected a corrupt DNG to fail")
	}
	if _, statErr := os.Stat(srcPath); !os.IsNotExist(statErr) {
		t.Fatalf("temp original %q still present: %v", srcPath, statErr)
	}
	cacheDir := filepath.Join(root, "PhotoBridge", "thumbnails")
	entries, readErr := os.ReadDir(cacheDir)
	if readErr == nil && len(entries) != 0 {
		t.Fatalf("cache dir has entries after a failed DNG preview: %d", len(entries))
	}
}

func TestFFmpegJPEGQuality(t *testing.T) {
	best, err := ffmpegJPEGQuality(100)
	if err != nil || best != 2 {
		t.Fatalf("quality 100 = %d, %v, want 2", best, err)
	}
	worst, err := ffmpegJPEGQuality(1)
	if err != nil || worst != 31 {
		t.Fatalf("quality 1 = %d, %v, want 31", worst, err)
	}
	if _, err := ffmpegJPEGQuality(0); err == nil {
		t.Fatal("quality 0 was accepted")
	}
	if _, err := ffmpegJPEGQuality(101); err == nil {
		t.Fatal("quality 101 was accepted")
	}
}

func writeTestVideo(t *testing.T, path, color string, w, h int, seconds float64) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := fmt.Sprintf("color=c=%s:s=%dx%d:r=25:d=%s", color, w, h, strconv.FormatFloat(seconds, 'f', -1, 64))
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", spec, "-pix_fmt", "yuv420p", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg fixture: %v\n%s", err, out)
	}
}

func testDNGBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 180, G: uint8(x), B: uint8(y), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, img, &tiff.Options{Compression: tiff.Uncompressed}); err != nil {
		t.Fatalf("encode tiff: %v", err)
	}
	return buf.Bytes()
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func decodeJPEGDataURL(t *testing.T, url string) image.Image {
	t.Helper()
	const prefix = "data:image/jpeg;base64,"
	if !strings.HasPrefix(url, prefix) {
		t.Fatalf("data URL prefix = %q", url[:min(len(url), len(prefix)+8)])
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, prefix))
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode jpeg: %v", err)
	}
	return img
}
