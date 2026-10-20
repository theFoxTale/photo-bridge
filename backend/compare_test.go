package backend

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var compareBase = time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)

func compareDate(minutes int) time.Time { return compareBase.Add(time.Duration(minutes) * time.Minute) }

// distinctJPEG returns valid JPEG data whose length depends on pad.
func distinctJPEG(t *testing.T, pad int) []byte {
	t.Helper()
	return append(testJPEG(t, false), bytes.Repeat([]byte{0}, pad)...)
}

func writeLocal(t *testing.T, path string, data []byte, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func photoIDs(photos []Photo) []string {
	ids := make([]string, 0, len(photos))
	for _, p := range photos {
		ids = append(ids, p.ID)
	}
	return ids
}

func containsID(photos []Photo, id string) bool {
	for _, p := range photos {
		if p.ID == id {
			return true
		}
	}
	return false
}

func TestRunCompareClassifies(t *testing.T) {
	dir := t.TempDir()
	lib := newFakeLibrary()

	same := distinctJPEG(t, 1)
	lib.addListed("100/IMG_A.JPG", "IMG_A.JPG", "jpeg", compareDate(1).Format(time.RFC3339), same)
	writeLocal(t, filepath.Join(dir, "sub", "IMG_A.JPG"), same, compareDate(1)) // found recursively

	lib.addListed("100/IMG_B.JPG", "IMG_B.JPG", "jpeg", compareDate(2).Format(time.RFC3339), distinctJPEG(t, 2))

	devC := distinctJPEG(t, 3)
	lib.addListed("100/IMG_C.JPG", "IMG_C.JPG", "jpeg", compareDate(3).Format(time.RFC3339), devC)
	localC := append([]byte(nil), devC...)
	localC[len(localC)-1] = 0xFF // same size, different content
	writeLocal(t, filepath.Join(dir, "IMG_C.JPG"), localC, compareDate(3))

	lib.addListed("100/IMG_H.HEIC", "IMG_H.HEIC", "heic", compareDate(4).Format(time.RFC3339), distinctJPEG(t, 4))
	writeLocal(t, filepath.Join(dir, "IMG_H.jpg"), distinctJPEG(t, 44), compareDate(4)) // converted copy
	lib.addListed("100/MOV_1.MOV", "MOV_1.MOV", "mov", compareDate(5).Format(time.RFC3339), []byte("video"))

	writeLocal(t, filepath.Join(dir, "only_here.png"), distinctJPEG(t, 6), compareDate(6))
	writeLocal(t, filepath.Join(dir, "notes.txt"), []byte("not an image"), compareDate(7))
	writeLocal(t, filepath.Join(dir, ".hidden", "secret.jpg"), distinctJPEG(t, 8), compareDate(8))

	var mu sync.Mutex
	var events []CompareProgress
	emit := func(p CompareProgress) {
		mu.Lock()
		events = append(events, p)
		mu.Unlock()
	}

	res, err := RunCompare(context.Background(), lib, nil, "dev", dir, emit)
	if err != nil {
		t.Fatal(err)
	}

	if got := photoIDs(res.AlreadyOnPC); len(got) != 1 || got[0] != "100/IMG_A.JPG" {
		t.Errorf("AlreadyOnPC = %v", got)
	}
	if !containsID(res.NewOnDevice, "100/IMG_B.JPG") || !containsID(res.NewOnDevice, "100/IMG_H.HEIC") {
		t.Errorf("NewOnDevice = %v, want IMG_B and IMG_H", photoIDs(res.NewOnDevice))
	}
	if containsID(res.NewOnDevice, "100/MOV_1.MOV") {
		t.Error("videos must not be compared")
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0].Photo.ID != "100/IMG_C.JPG" ||
		res.Conflicts[0].Reason != "content differs" ||
		res.Conflicts[0].LocalPath != filepath.Join(dir, "IMG_C.JPG") {
		t.Errorf("Conflicts = %+v", res.Conflicts)
	}
	if len(res.MissingOnPC) != 1 || res.MissingOnPC[0].Filename != "only_here.png" {
		t.Errorf("MissingOnPC = %+v, want only only_here.png (not the converted IMG_H.jpg, .txt or hidden files)", res.MissingOnPC)
	}
	if _, err := time.Parse(time.RFC3339, res.ScannedAt); err != nil {
		t.Errorf("ScannedAt = %q", res.ScannedAt)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 || events[len(events)-1].Phase != PhaseDone {
		t.Fatalf("progress events = %+v, want a final %q event", events, PhaseDone)
	}
	// IMG_A and IMG_C each need one device hash and one local hash.
	if last := events[len(events)-1]; last.Total != 4 || last.Scanned != 4 {
		t.Errorf("final progress = %+v, want 4 of 4", last)
	}
}

func TestRunCompareHashesOnlyCandidatesAndUsesCache(t *testing.T) {
	dir := t.TempDir()
	lib := newFakeLibrary()
	same := distinctJPEG(t, 1)
	lib.addListed("100/IMG_A.JPG", "IMG_A.JPG", "jpeg", compareDate(1).Format(time.RFC3339), same)
	writeLocal(t, filepath.Join(dir, "IMG_A.JPG"), same, compareDate(1))
	for i := 2; i <= 6; i++ { // new photos: never hashed
		id := "100/IMG_" + string(rune('A'+i-1)) + ".JPG"
		lib.addListed(id, filepath.Base(id), "jpeg", compareDate(i).Format(time.RFC3339), distinctJPEG(t, i))
	}

	var hashed atomic.Int32
	lib.hashHook = func(context.Context, string) error {
		hashed.Add(1)
		return nil
	}

	cache, err := OpenHashCache(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()

	for run := 1; run <= 2; run++ {
		res, err := RunCompare(context.Background(), lib, cache, "dev", dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.AlreadyOnPC) != 1 || len(res.NewOnDevice) != 5 {
			t.Fatalf("run %d: %d already, %d new", run, len(res.AlreadyOnPC), len(res.NewOnDevice))
		}
	}
	if got := hashed.Load(); got != 1 {
		t.Errorf("device hashed %d times over two runs, want 1 (one candidate, then cached)", got)
	}

	// A changed local file must be hashed again.
	writeLocal(t, filepath.Join(dir, "IMG_A.JPG"), append(same[:len(same):len(same)], 0), compareDate(1))
	res, err := RunCompare(context.Background(), lib, cache, "dev", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.AlreadyOnPC) != 0 || len(res.NewOnDevice) != 6 {
		t.Errorf("after the edit: %d already, %d new; the size changed so nothing matches", len(res.AlreadyOnPC), len(res.NewOnDevice))
	}
}

func TestRunCompareReportsUnreadableDeviceFile(t *testing.T) {
	dir := t.TempDir()
	lib := newFakeLibrary()
	data := distinctJPEG(t, 1)
	lib.addListed("100/IMG_A.JPG", "IMG_A.JPG", "jpeg", compareDate(1).Format(time.RFC3339), data)
	writeLocal(t, filepath.Join(dir, "IMG_A.JPG"), data, compareDate(1))
	lib.hashHook = func(context.Context, string) error { return errors.New("phone locked") }

	res, err := RunCompare(context.Background(), lib, nil, "dev", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Conflicts) != 1 || !strings.Contains(res.Conflicts[0].Reason, "phone locked") {
		t.Fatalf("Conflicts = %+v, want one that explains the read failure", res.Conflicts)
	}
}

func TestRunCompareCancel(t *testing.T) {
	dir := t.TempDir()
	lib := newFakeLibrary()
	data := distinctJPEG(t, 1)
	lib.addListed("100/IMG_A.JPG", "IMG_A.JPG", "jpeg", compareDate(1).Format(time.RFC3339), data)
	writeLocal(t, filepath.Join(dir, "IMG_A.JPG"), data, compareDate(1))

	started := make(chan struct{}, 1)
	lib.hashHook = func(ctx context.Context, _ string) error {
		started <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() {
		_, err := RunCompare(ctx, lib, nil, "dev", dir, nil)
		finished <- err
	}()

	<-started
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, ErrCompareCancelled) {
			t.Fatalf("err = %v, want ErrCompareCancelled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunCompare did not return after cancel")
	}
}

func TestRunCompareValidation(t *testing.T) {
	lib := newFakeLibrary()
	file := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, device, path, want string
	}{
		{"empty device", " ", t.TempDir(), "deviceID"},
		{"empty folder", "dev", " ", "choose a folder"},
		{"missing folder", "dev", filepath.Join(t.TempDir(), "nope"), "does not exist"},
		{"file instead of folder", "dev", file, "not a folder"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunCompare(context.Background(), lib, nil, tc.device, tc.path, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestRunCompareEmptyEverything(t *testing.T) {
	lib := newFakeLibrary()
	lib.listing = []Photo{} // an empty camera roll
	res, err := RunCompare(context.Background(), lib, nil, "dev", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.NewOnDevice == nil || res.AlreadyOnPC == nil || res.Conflicts == nil || res.MissingOnPC == nil {
		t.Errorf("lists must be empty, not nil (they are sent to JSON): %+v", res)
	}
}

func TestHashCache(t *testing.T) {
	cache, err := OpenHashCache(filepath.Join(t.TempDir(), "sub", "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()

	if _, ok := cache.Get("k", 10, 100); ok {
		t.Fatal("empty cache returned a hit")
	}
	cache.Put("k", 10, 100, "aaa")
	if sum, ok := cache.Get("k", 10, 100); !ok || sum != "aaa" {
		t.Fatalf("Get = %q, %v", sum, ok)
	}
	if _, ok := cache.Get("k", 11, 100); ok {
		t.Error("a different size must miss")
	}
	if _, ok := cache.Get("k", 10, 101); ok {
		t.Error("a different modification time must miss")
	}

	cache.Put("k", 20, 200, "bbb") // the file changed: the old version is dropped
	if _, ok := cache.Get("k", 10, 100); ok {
		t.Error("a stale entry survived")
	}
	if sum, ok := cache.Get("k", 20, 200); !ok || sum != "bbb" {
		t.Errorf("Get = %q, %v", sum, ok)
	}

	var none *HashCache // a missing cache must be harmless
	none.Put("k", 1, 1, "x")
	if _, ok := none.Get("k", 1, 1); ok {
		t.Error("nil cache returned a hit")
	}
	if err := none.Close(); err != nil {
		t.Error(err)
	}
}
