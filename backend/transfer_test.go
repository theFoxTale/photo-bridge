package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// testExif is a minimal little-endian EXIF block whose IFD0 holds one tag:
// Orientation = 6.
func testExif() []byte {
	return []byte{
		'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, // TIFF header, IFD0 at 8
		0x01, 0x00, // one entry
		0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, // Orientation = 6
		0x00, 0x00, 0x00, 0x00, // no next IFD
	}
}

func testJPEG(t *testing.T, withExif bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 5), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	if withExif {
		var err error
		if data, err = insertExif(data, testExif()); err != nil {
			t.Fatal(err)
		}
	}
	return data
}

// fakeLibrary serves in-memory photos. Every file is JPEG data, whatever
// format the photo declares; the converter recognises files by content.
type fakeLibrary struct {
	mu            sync.Mutex
	photos        map[string]Photo
	data          map[string][]byte
	downloadHook  func(ctx context.Context) error
	downloadCalls int
}

func newFakeLibrary() *fakeLibrary {
	return &fakeLibrary{photos: map[string]Photo{}, data: map[string][]byte{}}
}

func (l *fakeLibrary) add(id, filename, format string, data []byte) {
	l.photos[id] = Photo{
		ID: id, Filename: filename, Format: format, Size: int64(len(data)),
		Date: "2026-03-04T05:06:07Z",
	}
	l.data[id] = data
}

func (l *fakeLibrary) Albums(context.Context, string) ([]Album, error) { return nil, errors.New("unused") }
func (l *fakeLibrary) Photos(context.Context, string, string) ([]Photo, error) {
	return nil, errors.New("unused")
}
func (l *fakeLibrary) Thumbnail(context.Context, string, string) (string, error) {
	return "", errors.New("unused")
}

func (l *fakeLibrary) Describe(_ context.Context, _, photoID string) (Photo, error) {
	p, ok := l.photos[photoID]
	if !ok {
		return Photo{}, fmt.Errorf("photo %q not found", photoID)
	}
	return p, nil
}

func (l *fakeLibrary) Download(ctx context.Context, _, photoID, destPath string, onBytes func(int64)) error {
	l.mu.Lock()
	l.downloadCalls++
	hook := l.downloadHook
	l.mu.Unlock()
	if hook != nil {
		if err := hook(ctx); err != nil {
			return err
		}
	}
	data, ok := l.data[photoID]
	if !ok {
		return fmt.Errorf("photo %q not found", photoID)
	}
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return err
	}
	if onBytes != nil {
		onBytes(int64(len(data)))
	}
	return nil
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func hasName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func TestValidateTransfer(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok := TransferOptions{SaveOriginal: true, JPGQuality: 90, DestPath: dir}

	tests := []struct {
		name     string
		deviceID string
		ids      []string
		opts     func(TransferOptions) TransferOptions
		wantErr  string
	}{
		{"valid", "dev", []string{"a"}, nil, ""},
		{"empty device", " ", []string{"a"}, nil, "deviceID"},
		{"no photos", "dev", nil, nil, "no photos"},
		{"blank photo id", "dev", []string{"a", " "}, nil, "photo IDs"},
		{"nothing to save", "dev", []string{"a"}, func(o TransferOptions) TransferOptions { o.SaveOriginal = false; return o }, "at least one format"},
		{"quality zero", "dev", []string{"a"}, func(o TransferOptions) TransferOptions { o.JPGQuality = 0; return o }, "between 1 and 100"},
		{"quality too high", "dev", []string{"a"}, func(o TransferOptions) TransferOptions { o.JPGQuality = 101; return o }, "between 1 and 100"},
		{"no destination", "dev", []string{"a"}, func(o TransferOptions) TransferOptions { o.DestPath = " "; return o }, "destination folder"},
		{"missing destination", "dev", []string{"a"}, func(o TransferOptions) TransferOptions { o.DestPath = filepath.Join(dir, "nope"); return o }, "does not exist"},
		{"destination is a file", "dev", []string{"a"}, func(o TransferOptions) TransferOptions { o.DestPath = file; return o }, "not a folder"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := ok
			if tc.opts != nil {
				opts = tc.opts(opts)
			}
			_, _, err := validateTransfer(tc.deviceID, tc.ids, opts)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}

	ids, _, err := validateTransfer("dev", []string{"a", "b", "a"}, ok)
	if err != nil || len(ids) != 2 {
		t.Fatalf("duplicates were not removed: %v, %v", ids, err)
	}
}

// ---------------------------------------------------------------------------
// EXIF and image helpers
// ---------------------------------------------------------------------------

func TestExifRoundTripThroughJPEG(t *testing.T) {
	data := testJPEG(t, true)

	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("JPEG with inserted EXIF does not decode: %v", err)
	}
	got, err := jpegExif(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, testExif()) {
		t.Fatalf("EXIF changed in the round trip:\n got % x\nwant % x", got, testExif())
	}

	none, err := jpegExif(testJPEG(t, false))
	if err != nil || none != nil {
		t.Fatalf("jpegExif of a JPEG without EXIF = %v, %v; want nil, nil", none, err)
	}
}

func TestSetExifOrientation(t *testing.T) {
	raw := testExif()
	if err := setExifOrientation(raw, 1); err != nil {
		t.Fatal(err)
	}
	// 8 byte TIFF header + 2 byte count + 8 bytes (tag, type, count) = value at 18.
	if raw[18] != 1 || raw[19] != 0 {
		t.Fatalf("orientation bytes = % x, want 01 00", raw[18:20])
	}
	if err := setExifOrientation([]byte("garbage"), 1); err == nil {
		t.Fatal("expected an error for corrupt EXIF")
	}
}

func TestNormalizeExifStripsPrefix(t *testing.T) {
	prefixed := append(append([]byte{}, exifAPP1Prefix...), testExif()...)
	got, err := normalizeExif(prefixed)
	if err != nil || !bytes.Equal(got, testExif()) {
		t.Fatalf("normalizeExif = % x, %v", got, err)
	}
	if got, err := normalizeExif(nil); got != nil || err != nil {
		t.Fatalf("normalizeExif(nil) = %v, %v", got, err)
	}
}

func TestRotateCounterClockwise(t *testing.T) {
	// 3 wide, 2 high:  A B C
	//                  D E F
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	names := "ABCDEF"
	for i := 0; i < 6; i++ {
		src.Set(i%3, i/3, color.RGBA{R: uint8(i + 1), A: 255})
	}
	grid := func(img *image.RGBA) string {
		var sb strings.Builder
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				sb.WriteByte(names[img.RGBAAt(x, y).R-1])
			}
			sb.WriteByte('/')
		}
		return sb.String()
	}

	tests := map[int]string{
		1: "CF/BE/AD/", // 90 degrees counter-clockwise
		2: "FED/CBA/",
		3: "DA/EB/FC/", // 270 degrees counter-clockwise
	}
	for turns, want := range tests {
		if got := grid(rotateCounterClockwise(src, turns)); got != want {
			t.Errorf("turns=%d: got %s, want %s", turns, got, want)
		}
	}
}

func TestConvertToJPEG(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	original := testJPEG(t, true)
	if err := os.WriteFile(src, original, 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("quality 100 copies the file", func(t *testing.T) {
		dst := filepath.Join(dir, "copy.jpg")
		if err := convertToJPEG(src, dst, 100); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(readFile(t, dst), original) {
			t.Fatal("a JPEG at quality 100 must be copied unchanged")
		}
	})

	t.Run("lower quality re-encodes and keeps EXIF", func(t *testing.T) {
		dst := filepath.Join(dir, "small.jpg")
		if err := convertToJPEG(src, dst, 50); err != nil {
			t.Fatal(err)
		}
		out := readFile(t, dst)
		if bytes.Equal(out, original) {
			t.Fatal("expected a re-encoded file")
		}
		if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
			t.Fatalf("output does not decode: %v", err)
		}
		got, err := jpegExif(out)
		if err != nil || !bytes.Equal(got, testExif()) {
			t.Fatalf("EXIF lost in re-encode: % x, %v", got, err)
		}
	})

	t.Run("rejects unknown data and bad quality", func(t *testing.T) {
		junk := filepath.Join(dir, "junk.bin")
		if err := os.WriteFile(junk, []byte("this is not an image at all"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := convertToJPEG(junk, filepath.Join(dir, "x.jpg"), 90); err == nil {
			t.Error("expected an error for non-image data")
		}
		if err := convertToJPEG(src, filepath.Join(dir, "x.jpg"), 0); err == nil {
			t.Error("expected an error for quality 0")
		}
	})
}

// ---------------------------------------------------------------------------
// RunTransfer
// ---------------------------------------------------------------------------

func baseOptions(dir string) TransferOptions {
	return TransferOptions{SaveOriginal: true, SaveJPG: true, JPGQuality: 100, DestPath: dir}
}

func TestRunTransferOriginalAndJPG(t *testing.T) {
	dir := t.TempDir()
	lib := newFakeLibrary()
	exifJPEG := testJPEG(t, true)
	lib.add("100APPLE/IMG_0001.HEIC", "IMG_0001.HEIC", "heic", exifJPEG)

	var mu sync.Mutex
	var events []TransferProgress
	emit := func(p TransferProgress) {
		mu.Lock()
		events = append(events, p)
		mu.Unlock()
	}

	res, err := RunTransfer(context.Background(), lib, "dev", []string{"100APPLE/IMG_0001.HEIC"}, baseOptions(dir), emit)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Succeeded) != 1 || len(res.Failed) != 0 || res.Cancelled {
		t.Fatalf("result = %+v", res)
	}
	if res.BytesCopied != int64(len(exifJPEG)) {
		t.Errorf("BytesCopied = %d, want %d", res.BytesCopied, len(exifJPEG))
	}

	if !bytes.Equal(readFile(t, filepath.Join(dir, "IMG_0001.HEIC")), exifJPEG) {
		t.Error("the original must be saved unchanged")
	}
	jpg := readFile(t, filepath.Join(dir, "IMG_0001.jpg"))
	if got, err := jpegExif(jpg); err != nil || !bytes.Equal(got, testExif()) {
		t.Errorf("EXIF missing from the JPG: % x, %v", got, err)
	}

	info, err := os.Stat(filepath.Join(dir, "IMG_0001.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC); !info.ModTime().Equal(want) {
		t.Errorf("modtime = %v, want %v", info.ModTime(), want)
	}

	if names := listDir(t, dir); len(names) != 2 {
		t.Errorf("destination holds %v, want exactly the two output files (no temp leftovers)", names)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 {
		t.Fatal("no progress events")
	}
	last := events[len(events)-1]
	if last.Completed != 1 || last.Total != 1 || last.BytesCopied != int64(len(exifJPEG)) {
		t.Errorf("final progress = %+v", last)
	}
}

func TestRunTransferIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	lib := newFakeLibrary()
	lib.add("a/IMG_1.HEIC", "IMG_1.HEIC", "heic", testJPEG(t, true))
	lib.add("a/IMG_2.JPG", "IMG_2.JPG", "jpeg", testJPEG(t, false))
	ids := []string{"a/IMG_1.HEIC", "a/IMG_2.JPG"}

	if _, err := RunTransfer(context.Background(), lib, "dev", ids, baseOptions(dir), nil); err != nil {
		t.Fatal(err)
	}
	before := lib.downloadCalls

	res, err := RunTransfer(context.Background(), lib, "dev", ids, baseOptions(dir), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Succeeded) != 2 || len(res.Failed) != 0 {
		t.Fatalf("second run result = %+v", res)
	}
	if lib.downloadCalls != before || res.BytesCopied != 0 {
		t.Errorf("second run downloaded again: calls %d -> %d, bytes %d", before, lib.downloadCalls, res.BytesCopied)
	}
}

func TestRunTransferNameCollision(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "IMG_1.HEIC"), []byte("something else"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib := newFakeLibrary()
	data := testJPEG(t, false)
	lib.add("a/IMG_1.HEIC", "IMG_1.HEIC", "heic", data)
	opts := TransferOptions{SaveOriginal: true, JPGQuality: 90, DestPath: dir}

	for run := 1; run <= 2; run++ {
		res, err := RunTransfer(context.Background(), lib, "dev", []string{"a/IMG_1.HEIC"}, opts, nil)
		if err != nil || len(res.Succeeded) != 1 {
			t.Fatalf("run %d: %+v, %v", run, res, err)
		}
	}

	if got := string(readFile(t, filepath.Join(dir, "IMG_1.HEIC"))); got != "something else" {
		t.Error("an existing file with a different size was overwritten")
	}
	if !bytes.Equal(readFile(t, filepath.Join(dir, "IMG_1 (1).HEIC")), data) {
		t.Error("the photo was not saved under a numbered name")
	}
	if names := listDir(t, dir); len(names) != 2 {
		t.Errorf("destination holds %v; the re-run must not create IMG_1 (2).HEIC", names)
	}
}

func TestRunTransferSameNameInOneRun(t *testing.T) {
	dir := t.TempDir()
	lib := newFakeLibrary()
	lib.add("100APPLE/IMG_1.JPG", "IMG_1.JPG", "jpeg", testJPEG(t, false))
	lib.add("101APPLE/IMG_1.JPG", "IMG_1.JPG", "jpeg", append(testJPEG(t, false), 0))
	opts := TransferOptions{SaveOriginal: true, JPGQuality: 90, DestPath: dir}

	res, err := RunTransfer(context.Background(), lib, "dev", []string{"100APPLE/IMG_1.JPG", "101APPLE/IMG_1.JPG"}, opts, nil)
	if err != nil || len(res.Succeeded) != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
	if names := listDir(t, dir); len(names) != 2 {
		t.Errorf("destination holds %v, want two distinct files", names)
	}
}

func TestRunTransferJPEGOriginalIsNotDuplicated(t *testing.T) {
	dir := t.TempDir()
	lib := newFakeLibrary()
	lib.add("a/IMG_1.JPG", "IMG_1.JPG", "jpeg", testJPEG(t, true))

	res, err := RunTransfer(context.Background(), lib, "dev", []string{"a/IMG_1.JPG"}, baseOptions(dir), nil)
	if err != nil || len(res.Succeeded) != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	if names := listDir(t, dir); len(names) != 1 || !hasName(names, "IMG_1.JPG") {
		t.Errorf("destination holds %v, want only IMG_1.JPG", names)
	}
}

func TestRunTransferReportsUnsupportedAndMissing(t *testing.T) {
	dir := t.TempDir()
	lib := newFakeLibrary()
	lib.add("a/IMG_1.DNG", "IMG_1.DNG", "dng", []byte("raw data"))
	lib.add("a/IMG_2.HEIC", "IMG_2.HEIC", "heic", testJPEG(t, false))
	opts := TransferOptions{SaveJPG: true, JPGQuality: 90, DestPath: dir}

	res, err := RunTransfer(context.Background(), lib, "dev", []string{"a/IMG_1.DNG", "a/IMG_2.HEIC", "a/missing.HEIC"}, opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Succeeded) != 1 || res.Succeeded[0] != "a/IMG_2.HEIC" {
		t.Errorf("Succeeded = %v", res.Succeeded)
	}
	if len(res.Failed) != 2 {
		t.Fatalf("Failed = %+v, want 2 entries", res.Failed)
	}
	reasons := map[string]string{}
	for _, f := range res.Failed {
		reasons[f.PhotoID] = f.Reason
	}
	if !strings.Contains(reasons["a/IMG_1.DNG"], "not supported for DNG") {
		t.Errorf("DNG reason = %q", reasons["a/IMG_1.DNG"])
	}
	if !strings.Contains(reasons["a/missing.HEIC"], "not found") {
		t.Errorf("missing photo reason = %q", reasons["a/missing.HEIC"])
	}
	if names := listDir(t, dir); len(names) != 1 || !hasName(names, "IMG_2.jpg") {
		t.Errorf("destination holds %v, want only IMG_2.jpg", names)
	}
}

func TestRunTransferCancel(t *testing.T) {
	dir := t.TempDir()
	lib := newFakeLibrary()
	ids := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("a/IMG_%d.JPG", i)
		lib.add(id, fmt.Sprintf("IMG_%d.JPG", i), "jpeg", testJPEG(t, false))
		ids = append(ids, id)
	}

	started := make(chan struct{}, 10)
	lib.downloadHook = func(ctx context.Context) error {
		started <- struct{}{}
		<-ctx.Done() // a download that never finishes on its own
		return ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	type outcome struct {
		res TransferResult
		err error
	}
	finished := make(chan outcome, 1)
	go func() {
		res, err := RunTransfer(ctx, lib, "dev", ids, TransferOptions{SaveOriginal: true, JPGQuality: 90, DestPath: dir}, nil)
		finished <- outcome{res, err}
	}()

	<-started
	cancel()

	select {
	case out := <-finished:
		if out.err != nil {
			t.Fatal(out.err)
		}
		if !out.res.Cancelled {
			t.Error("Cancelled = false")
		}
		if len(out.res.Succeeded) != 0 || len(out.res.Failed) != 0 {
			t.Errorf("abandoned photos must not be reported: %+v", out.res)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunTransfer did not return after cancel")
	}

	if names := listDir(t, dir); len(names) != 0 {
		t.Errorf("cancelled transfer left %v behind", names)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := map[string]string{
		"IMG_0001.HEIC": "IMG_0001.HEIC",
		`a:b*c?.jpg`:    "a_b_c_.jpg",
		"  name.jpg. ":  "name.jpg",
		"..":            "",
		"":              "",
	}
	for in, want := range tests {
		if got := sanitizeFilename(in); got != want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}
