package main

import "photo-bridge/backend"

// GetStorageInfo returns free space on the destination volume and, when a
// device is connected, on that iPhone. An empty destPath reads C:\. iPhone
// numbers are zero when the device is missing or does not answer.
func (a *App) GetStorageInfo(deviceID, destPath string) (backend.StorageInfo, error) {
	return backend.GetStorageInfo(a.runtimeContext(), deviceID, destPath)
}
