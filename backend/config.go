package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	appDirName   = "PhotoBridge"
	configFile   = "config.json"
	cacheDirName = "thumbnails"
)

// Config is the on-disk application settings file.
type Config struct {
	CacheDir string `json:"cacheDir"`
}

// appRoot is %APPDATA%/PhotoBridge, or the user cache directory when APPDATA
// is unset.
func appRoot() (string, error) {
	root := strings.TrimSpace(os.Getenv("APPDATA"))
	if root == "" {
		var err error
		root, err = os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("thumbnail cache directory: %w", err)
		}
	}
	return filepath.Join(root, appDirName), nil
}

func configFilePath() (string, error) {
	root, err := appRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, configFile), nil
}

// DefaultCacheDir is %APPDATA%/PhotoBridge/thumbnails.
func DefaultCacheDir() (string, error) {
	root, err := appRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, cacheDirName), nil
}

// LoadConfig reads config.json. A missing file is an empty config, so the
// cache directory stays at its default.
func LoadConfig() (Config, error) {
	path, err := configFilePath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	cfg.CacheDir = strings.TrimSpace(cfg.CacheDir)
	return cfg, nil
}

// SaveConfig writes config.json, replacing any previous file.
func SaveConfig(cfg Config) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	cfg.CacheDir = strings.TrimSpace(cfg.CacheDir)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), "config-*.json")
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("save config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("save config: %w", err)
	}
	if err := replaceFile(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

// replaceFile renames tmp onto dst. Windows refuses to rename over a file
// that already exists, so the destination is removed and the rename retried.
func replaceFile(tmp, dst string) error {
	if err := os.Rename(tmp, dst); err == nil {
		return nil
	}
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmp, dst)
}

// ResolveCacheDir returns the configured cache folder, or the default
// thumbnails directory when the config does not set one.
func ResolveCacheDir() (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}
	if cfg.CacheDir != "" {
		return filepath.Clean(cfg.CacheDir), nil
	}
	return DefaultCacheDir()
}

// CacheStats reports the resolved cache folder and the total size of the
// files inside it. A missing folder is an empty cache, not an error.
func CacheStats(ctx context.Context) (string, int64, error) {
	if ctx == nil {
		return "", 0, errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	dir, err := ResolveCacheDir()
	if err != nil {
		return "", 0, err
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return dir, 0, nil
		}
		return "", 0, fmt.Errorf("cache stats: %w", err)
	}
	if !info.IsDir() {
		return "", 0, fmt.Errorf("cache folder is not a directory: %s", dir)
	}

	var size int64
	walkErr := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		size += fileInfo.Size()
		return nil
	})
	if walkErr != nil {
		return "", 0, fmt.Errorf("cache stats: %w", walkErr)
	}
	return dir, size, nil
}

// ClearCache deletes everything inside the cache folder and recreates the
// empty folder.
func ClearCache(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, err := ResolveCacheDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("clear cache: %w", err)
			}
			return nil
		}
		return fmt.Errorf("clear cache: %w", err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return fmt.Errorf("clear cache: %w", err)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("clear cache: %w", err)
	}
	return nil
}

// SetCacheFolder makes path the thumbnail cache. The folder is created when
// it does not exist. The previous cache is removed when it is a different
// directory and neither path contains the other.
func SetCacheFolder(ctx context.Context, path string) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("choose a cache folder")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid cache folder: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot access cache folder: %w", err)
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return fmt.Errorf("create cache folder: %w", err)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("cache folder is not a directory: %s", abs)
	}

	if err := rejectSettingsFolder(abs); err != nil {
		return err
	}

	old, err := ResolveCacheDir()
	if err != nil {
		return err
	}
	oldAbs, err := filepath.Abs(old)
	if err != nil {
		return fmt.Errorf("invalid cache folder: %w", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.CacheDir = abs
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	if samePath(oldAbs, abs) || containsPath(oldAbs, abs) || containsPath(abs, oldAbs) {
		return nil
	}
	if _, statErr := os.Stat(oldAbs); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil
		}
		return fmt.Errorf("remove previous cache folder: %w", statErr)
	}
	if err := os.RemoveAll(oldAbs); err != nil {
		return fmt.Errorf("remove previous cache folder: %w", err)
	}
	return nil
}

// rejectSettingsFolder stops the cache from being the folder that holds
// config.json, or a parent of that folder. Clearing the cache would otherwise
// delete the settings file.
func rejectSettingsFolder(dir string) error {
	root, err := appRoot()
	if err != nil {
		return err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("invalid cache folder: %w", err)
	}
	if samePath(dir, rootAbs) || containsPath(dir, rootAbs) {
		return errors.New("choose a folder that is not the application settings folder")
	}
	return nil
}

func samePath(a, b string) bool {
	a = normalizePath(a)
	b = normalizePath(b)
	return a == b
}

// containsPath reports whether child is strictly inside parent.
func containsPath(parent, child string) bool {
	rel, err := filepath.Rel(normalizePath(parent), normalizePath(child))
	if err != nil || rel == "." {
		return false
	}
	return !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

func normalizePath(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}
