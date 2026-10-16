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
func Discover(ctx context.Context) ([]Device, error) {
	if ctx == nil {
		return nil, fmt.Errorf("discovery: nil context")
	}

	timer := time.NewTimer(mockSearchDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return []Device{}, nil
	case <-timer.C:
		return []Device{
			{
				ID:    "mock-iphone-15",
				Name:  "iPhone 15 (mock)",
				IP:    "192.168.0.42",
				Model: "iPhone15,4",
			},
		}, nil
	}
}
