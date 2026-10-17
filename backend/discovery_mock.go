//go:build mock

package backend

import (
	"context"
	"fmt"
	"time"
)

// mockSearchDelay simulates the time a network search takes, so the
// "Searching" UI state is visible during development.
const mockSearchDelay = 800 * time.Millisecond

// Discover returns a single fake iPhone after a short delay. If ctx ends first
// (for example StopDiscovery was called) it returns no devices, like a real
// search that found nothing yet.
func Discover(ctx context.Context) (DiscoveryResult, error) {
	if ctx == nil {
		return DiscoveryResult{Devices: []Device{}, Status: StatusIdle}, fmt.Errorf("discovery: nil context")
	}

	timer := time.NewTimer(mockSearchDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return DiscoveryResult{Devices: []Device{}, Status: StatusIdle}, nil
	case <-timer.C:
		return DiscoveryResult{
			Devices: []Device{
				{
					ID:    "mock-iphone-15",
					Name:  "iPhone 15 (mock)",
					IP:    "192.168.0.42",
					Model: "iPhone15,4",
				},
			},
			Status: StatusConnected,
		}, nil
	}
}

// CheckMdns always reports false in mock builds: there is no network to probe.
func CheckMdns(_ context.Context) (bool, error) {
	return false, nil
}
