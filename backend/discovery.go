// Package backend contains the domain logic of Photo Bridge. The Wails-bound
// App struct lives in package main (so the frontend path is window.go.main.App)
// and delegates to the functions in this package.
package backend

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	// ServiceType is the Bonjour service iOS devices advertise for Wi-Fi sync.
	ServiceType = "_apple-mobdev2._tcp"
	// ServiceDomain is the mDNS domain that is browsed.
	ServiceDomain = "local."
	// DiscoveryTimeout is how long a single discovery run lasts.
	DiscoveryTimeout = 5 * time.Second

	// MdnsDiagnosticsTimeout is how long the mDNS diagnostic browse lasts.
	MdnsDiagnosticsTimeout = 2 * time.Second

	// collectorGrace is how long to wait for the entry collector to finish
	// after the browse context has ended.
	collectorGrace = time.Second
)

// Values of DiscoveryResult.Status.
const (
	// StatusIdle: no device was found anywhere.
	StatusIdle = "idle"
	// StatusConnected: at least one device is visible to usbmuxd.
	StatusConnected = "connected"
	// StatusBlockedByApple: a device is visible on the network via mDNS, but
	// usbmuxd (Apple Devices / iTunes) does not list any device.
	StatusBlockedByApple = "blocked_by_apple"
)

// DiscoveryResult is what a device search reports to the frontend.
type DiscoveryResult struct {
	Devices []Device `json:"devices"`
	Status  string   `json:"status"`
}

// Device describes a device found on the local network.
type Device struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	IP    string `json:"ip"`
	Model string `json:"model"`
}

// BrowseDevices browses the local network for iOS devices until ctx is done.
// It never returns a nil slice. Cancelling ctx (or hitting its deadline) is a
// normal way to end the run and returns the devices found so far.
func BrowseDevices(ctx context.Context) ([]Device, error) {
	if ctx == nil {
		return nil, fmt.Errorf("discovery: nil context")
	}

	resolver, err := zeroconf.NewResolver()
	if err != nil {
		return nil, fmt.Errorf("discovery: create resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	found := make(map[string]Device)
	var mu sync.Mutex
	collectorDone := make(chan struct{})
	abort := make(chan struct{})

	go func() {
		defer close(collectorDone)
		for {
			select {
			case entry, open := <-entries:
				if !open {
					return
				}
				device, ok := deviceFromEntry(entry)
				if !ok {
					continue
				}
				mu.Lock()
				existing, seen := found[device.ID]
				// Entries for the same instance can arrive more than once (IPv4
				// and IPv6 records); keep the one that carries an address.
				if !seen || (existing.IP == "" && device.IP != "") {
					found[device.ID] = device
				}
				mu.Unlock()
			case <-abort:
				return
			}
		}
	}()

	if err := resolver.Browse(ctx, ServiceType, ServiceDomain, entries); err != nil {
		close(abort)
		return nil, fmt.Errorf("discovery: browse %s: %w", ServiceType, err)
	}

	<-ctx.Done()

	select {
	case <-collectorDone:
	case <-time.After(collectorGrace):
	}

	mu.Lock()
	defer mu.Unlock()

	devices := make([]Device, 0, len(found))
	for _, device := range found {
		devices = append(devices, device)
	}
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Name != devices[j].Name {
			return devices[i].Name < devices[j].Name
		}
		return devices[i].ID < devices[j].ID
	})
	return devices, nil
}

// deviceFromEntry converts a zeroconf entry into a Device. It reports false
// for entries that cannot be identified.
func deviceFromEntry(entry *zeroconf.ServiceEntry) (Device, bool) {
	if entry == nil {
		return Device{}, false
	}
	id := strings.TrimSpace(entry.Instance)
	if id == "" {
		return Device{}, false
	}

	name := strings.TrimSuffix(strings.TrimSpace(entry.HostName), ".")
	name = strings.TrimSuffix(name, ".local")
	if name == "" {
		name = id
	}

	ip := ""
	switch {
	case len(entry.AddrIPv4) > 0:
		ip = entry.AddrIPv4[0].String()
	case len(entry.AddrIPv6) > 0:
		ip = entry.AddrIPv6[0].String()
	}

	// The _apple-mobdev2 service does not advertise a device model, so Model
	// stays empty rather than being guessed.
	return Device{ID: id, Name: name, IP: ip, Model: ""}, true
}
