package main

import (
	"context"
	"errors"
	"fmt"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"photo-bridge/backend"
)

// transferProgressEvent is the Wails event carrying a backend.TransferProgress.
const transferProgressEvent = "transfer:progress"

// SelectFolder opens the native folder picker and returns the chosen path. It
// returns an empty string, and no error, when the user cancels the dialog.
func (a *App) SelectFolder() (string, error) {
	ctx, ok := a.wailsContext()
	if !ok {
		return "", errors.New("the application window is not ready yet")
	}
	path, err := wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
		Title:                "Choose destination folder",
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", fmt.Errorf("select folder: %w", err)
	}
	return path, nil
}

// TransferPhotos copies the given photos from the device to opts.DestPath and
// blocks until it is done or cancelled. Progress is reported through the
// "transfer:progress" event. Only one transfer can run at a time. When the
// transfer is cancelled the partial result is returned with Cancelled set.
func (a *App) TransferPhotos(deviceID string, photoIDs []string, opts backend.TransferOptions) (backend.TransferResult, error) {
	ctx, cancel := context.WithCancel(a.runtimeContext())
	defer cancel()

	a.mu.Lock()
	if a.transferCancel != nil {
		a.mu.Unlock()
		return backend.TransferResult{Succeeded: []string{}, Failed: []backend.TransferError{}},
			errors.New("a transfer is already running")
	}
	a.transferCancel = cancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.transferCancel = nil
		a.mu.Unlock()
	}()

	emit := func(p backend.TransferProgress) {
		if eventCtx, ok := a.wailsContext(); ok {
			wailsruntime.EventsEmit(eventCtx, transferProgressEvent, p)
		}
	}

	result, err := backend.RunTransfer(ctx, a.library, deviceID, photoIDs, opts, emit)
	if err != nil {
		return result, fmt.Errorf("transfer photos: %w", err)
	}
	return result, nil
}

// CancelTransfer stops the running transfer after the files in flight are
// abandoned. It does nothing when no transfer is running.
func (a *App) CancelTransfer() error {
	a.mu.Lock()
	cancel := a.transferCancel
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return nil
}
