package main

import (
	"context"
	"fmt"

	"photo-bridge/backend"
)

// DiscoverDevices browses the local network for iOS devices for up to
// backend.DiscoveryTimeout and returns what it found. The result is an empty
// slice (never nil) when nothing is found. If a discovery run is already in
// flight it is cancelled and replaced by this one.
func (a *App) DiscoverDevices() ([]backend.Device, error) {
	a.mu.Lock()
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	if a.discoveryCancel != nil {
		a.discoveryCancel()
	}
	ctx, cancel := context.WithTimeout(parent, backend.DiscoveryTimeout)
	a.discoveryToken++
	token := a.discoveryToken
	a.discoveryCancel = cancel
	a.mu.Unlock()

	defer func() {
		cancel()
		a.mu.Lock()
		// Only clear the slot if a newer run has not taken it over.
		if a.discoveryToken == token {
			a.discoveryCancel = nil
		}
		a.mu.Unlock()
	}()

	devices, err := backend.Discover(ctx)
	if err != nil {
		return []backend.Device{}, fmt.Errorf("discover devices: %w", err)
	}
	return devices, nil
}

// StopDiscovery cancels an in-flight discovery run. It is a no-op when no run
// is in progress.
func (a *App) StopDiscovery() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.discoveryCancel != nil {
		a.discoveryCancel()
		a.discoveryCancel = nil
	}
	return nil
}
