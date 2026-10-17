package main

import (
	"context"
	"fmt"

	"photo-bridge/backend"
)

// DiscoverDevices searches for iOS devices for up to backend.DiscoveryTimeout.
// The result holds the devices (an empty slice, never nil, when none) and a
// status: "connected", "idle", or "blocked_by_apple" when usbmuxd sees nothing
// but a phone is visible on the network. If a discovery run is already in
// flight it is cancelled and replaced by this one.
func (a *App) DiscoverDevices() (backend.DiscoveryResult, error) {
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

	result, err := backend.Discover(ctx)
	if err != nil {
		return backend.DiscoveryResult{Devices: []backend.Device{}, Status: backend.StatusIdle},
			fmt.Errorf("discover devices: %w", err)
	}
	if result.Devices == nil {
		result.Devices = []backend.Device{}
	}
	return result, nil
}

// CheckMdnsDiagnostics runs a quick (2 second) mDNS browse for
// _apple-mobdev2._tcp and reports whether any device answered. It is a
// diagnostic only; it never produces devices that can be used.
func (a *App) CheckMdnsDiagnostics() (bool, error) {
	visible, err := backend.CheckMdns(a.runtimeContext())
	if err != nil {
		return false, fmt.Errorf("check mdns: %w", err)
	}
	return visible, nil
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
