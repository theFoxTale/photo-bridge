package main

import (
	"errors"
	"fmt"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"photo-bridge/backend"
)

// GetCacheStats returns the current thumbnail cache folder and the total size
// of the files in it. A missing folder is reported as size 0.
func (a *App) GetCacheStats() (map[string]interface{}, error) {
	path, size, err := backend.CacheStats(a.runtimeContext())
	if err != nil {
		return map[string]interface{}{}, err
	}
	return map[string]interface{}{
		"path":      path,
		"sizeBytes": size,
	}, nil
}

// ClearCache deletes every file in the thumbnail cache and recreates the
// empty folder.
func (a *App) ClearCache() error {
	return backend.ClearCache(a.runtimeContext())
}

// SelectCacheFolder opens the native folder picker for a new thumbnail cache.
// It returns an empty string, and no error, when the user cancels the dialog.
func (a *App) SelectCacheFolder() (string, error) {
	ctx, ok := a.wailsContext()
	if !ok {
		return "", errors.New("the application window is not ready yet")
	}
	path, err := wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
		Title:                "Choose cache folder",
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", fmt.Errorf("select cache folder: %w", err)
	}
	return path, nil
}

// SetCacheFolder stores path as the thumbnail cache, creating it when needed,
// and removes the previous cache when the two folders are separate.
func (a *App) SetCacheFolder(path string) error {
	return backend.SetCacheFolder(a.runtimeContext(), path)
}
