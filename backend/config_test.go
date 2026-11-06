package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCacheDirDefaultsWithoutConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)

	got, err := ResolveCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "PhotoBridge", "thumbnails")
	if got != want {
		t.Fatalf("cache dir = %q, want %q", got, want)
	}
}

func TestConfigRoundTripUsesCustomCacheDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	custom := filepath.Join(t.TempDir(), "thumbs")

	if err := SaveConfig(Config{CacheDir: custom}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CacheDir != custom {
		t.Fatalf("CacheDir = %q, want %q", cfg.CacheDir, custom)
	}
	got, err := ResolveCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != custom {
		t.Fatalf("resolved = %q, want %q", got, custom)
	}
}

func TestLoadConfigRejectsCorruptFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	dir := filepath.Join(root, "PhotoBridge")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected corrupt config to fail")
	}
}

func TestCacheStatsAndClear(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	dir, err := DefaultCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "device-1")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("thumbnail-bytes")
	if err := os.WriteFile(filepath.Join(nested, "a.jpg"), payload, 0o644); err != nil {
		t.Fatal(err)
	}

	path, size, err := CacheStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if path != dir {
		t.Fatalf("path = %q, want %q", path, dir)
	}
	if size != int64(len(payload)) {
		t.Fatalf("size = %d, want %d", size, len(payload))
	}

	missing := filepath.Join(t.TempDir(), "absent")
	if err := SaveConfig(Config{CacheDir: missing}); err != nil {
		t.Fatal(err)
	}
	path, size, err = CacheStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if path != missing || size != 0 {
		t.Fatalf("missing cache = %q size %d, want %q size 0", path, size, missing)
	}

	if err := SaveConfig(Config{CacheDir: dir}); err != nil {
		t.Fatal(err)
	}
	if err := ClearCache(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("cache dir after clear: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("cache dir still has %d entries", len(entries))
	}
	_, size, err = CacheStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if size != 0 {
		t.Fatalf("size after clear = %d, want 0", size)
	}
}

func TestSetCacheFolderMovesCacheAndRemovesOld(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	oldDir, err := DefaultCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(oldDir, "stale.jpg")
	if err := os.WriteFile(oldFile, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	next := filepath.Join(t.TempDir(), "next-cache")
	if err := SetCacheFolder(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(next); err != nil {
		t.Fatalf("new cache dir: %v", err)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("old cache file still present: %v", err)
	}
	got, err := ResolveCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != next {
		t.Fatalf("resolved = %q, want %q", got, next)
	}
}

func TestSetCacheFolderKeepsParentWhenNested(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	oldDir, err := DefaultCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(oldDir, "keep.jpg")
	if err := os.WriteFile(kept, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(oldDir, "nested")
	if err := SetCacheFolder(context.Background(), child); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("parent cache file: %v", err)
	}
	if _, err := os.Stat(child); err != nil {
		t.Fatalf("nested cache dir: %v", err)
	}
}

func TestSetCacheFolderRejectsBadPaths(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)

	if err := SetCacheFolder(context.Background(), "  "); err == nil {
		t.Fatal("expected an empty path to fail")
	}
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetCacheFolder(context.Background(), file); err == nil {
		t.Fatal("expected a file path to fail")
	}
	if err := SetCacheFolder(context.Background(), root); err == nil {
		t.Fatal("expected the settings parent to fail")
	}
	settings := filepath.Join(root, "PhotoBridge")
	if err := os.MkdirAll(settings, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SetCacheFolder(context.Background(), settings); err == nil {
		t.Fatal("expected the settings folder to fail")
	}
}

func TestThumbnailUsesConfiguredCacheDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	custom := filepath.Join(t.TempDir(), "custom-cache")
	if err := SetCacheFolder(context.Background(), custom); err != nil {
		t.Fatal(err)
	}

	payload := pngBytes(t, 20, 20)
	_, err := cachedThumbnail(context.Background(), "device-1", "IMG_0001.PNG", func(_ context.Context, dest string) error {
		return os.WriteFile(dest, payload, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	cacheFile := filepath.Join(custom, "device-1", "IMG_0001.PNG"+thumbnailCacheSuffix)
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("configured cache file: %v", err)
	}
	defaultFile := filepath.Join(root, "PhotoBridge", "thumbnails", "device-1", "IMG_0001.PNG"+thumbnailCacheSuffix)
	if _, err := os.Stat(defaultFile); !os.IsNotExist(err) {
		t.Fatalf("thumbnail written to the default cache: %v", err)
	}
}
