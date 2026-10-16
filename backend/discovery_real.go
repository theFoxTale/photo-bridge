//go:build !mock

package backend

import "context"

// Discover searches the local network for real devices via mDNS.
func Discover(ctx context.Context) ([]Device, error) {
	return BrowseDevices(ctx)
}
