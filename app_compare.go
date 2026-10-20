package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"photo-bridge/backend"
)

// compareProgressEvent is the Wails event carrying a backend.CompareProgress.
const compareProgressEvent = "compare:progress"

// hashCache opens the SQLite hash cache on first use. When it cannot be
// opened the comparison still works, it just re-hashes every file.
func (a *App) hashCacheOrNil() *backend.HashCache {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.hashCache != nil || a.hashCacheFailed {
		return a.hashCache
	}
	path, err := backend.DefaultHashCachePath()
	if err == nil {
		a.hashCache, err = backend.OpenHashCache(path)
	}
	if err != nil {
		a.hashCacheFailed = true
		a.hashCache = nil
		slog.Warn("hash cache unavailable, comparing without it", "error", err)
	}
	return a.hashCache
}

// CompareFolder compares the camera roll of the device with the images in
// localPath (searched recursively) and returns what is new on the device,
// already on the PC, in conflict, or only on the PC. It blocks until done or
// cancelled and reports progress through the "compare:progress" event. Only
// one comparison runs at a time. A cancelled comparison returns an error.
func (a *App) CompareFolder(deviceID, localPath string) (backend.CompareResult, error) {
	ctx, cancel := context.WithCancel(a.runtimeContext())
	defer cancel()

	a.mu.Lock()
	if a.compareCancel != nil {
		a.mu.Unlock()
		return emptyCompareResult(), errors.New("a comparison is already running")
	}
	a.compareCancel = cancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.compareCancel = nil
		a.mu.Unlock()
	}()

	emit := func(p backend.CompareProgress) {
		if eventCtx, ok := a.wailsContext(); ok {
			wailsruntime.EventsEmit(eventCtx, compareProgressEvent, p)
		}
	}

	result, err := backend.RunCompare(ctx, a.library, a.hashCacheOrNil(), deviceID, localPath, emit)
	if err != nil {
		if errors.Is(err, backend.ErrCompareCancelled) {
			return result, err
		}
		return result, fmt.Errorf("compare folder: %w", err)
	}
	return result, nil
}

// CancelCompare stops the running comparison. It does nothing when none runs.
func (a *App) CancelCompare() error {
	a.mu.Lock()
	cancel := a.compareCancel
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return nil
}

func emptyCompareResult() backend.CompareResult {
	return backend.CompareResult{
		NewOnDevice: []backend.Photo{},
		AlreadyOnPC: []backend.Photo{},
		Conflicts:   []backend.Conflict{},
		MissingOnPC: []backend.LocalFile{},
	}
}
