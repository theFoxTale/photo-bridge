//go:build !mock

package backend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
)

// deviceInfoTimeout bounds the lockdown query that reads a device's name.
const deviceInfoTimeout = 4 * time.Second

// Discover lists the devices usbmuxd knows about. On Windows usbmuxd is
// provided by Apple Devices / iTunes and includes devices reachable over
// Wi-Fi Sync. The ID of each Device is the UDID, which is what go-ios (and so
// the photo library) needs.
func Discover(ctx context.Context) ([]Device, error) {
	if ctx == nil {
		return nil, errors.New("discovery: nil context")
	}
	if err := ctx.Err(); err != nil {
		return []Device{}, nil
	}

	list, err := goios.ListDevices()
	if err != nil {
		slog.Debug("go-ios list devices failed", "error", err)
		return []Device{}, classifyDiscoveryError(err)
	}

	// A device can be listed twice (USB and Network); keep one entry per UDID
	// and prefer the USB one.
	entries := make(map[string]goios.DeviceEntry)
	for _, entry := range list.DeviceList {
		udid := entry.Properties.SerialNumber
		if udid == "" {
			continue
		}
		existing, seen := entries[udid]
		if !seen || (existing.Properties.ConnectionType != "USB" && entry.Properties.ConnectionType == "USB") {
			entries[udid] = entry
		}
	}

	devices := make([]Device, 0, len(entries))
	for udid, entry := range entries {
		if err := ctx.Err(); err != nil {
			return []Device{}, nil
		}
		name, model := lookupDeviceInfo(ctx, entry)
		if name == "" {
			name = "iOS device " + shortID(udid)
		}
		devices = append(devices, Device{ID: udid, Name: name, IP: "", Model: model})
	}

	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Name != devices[j].Name {
			return devices[i].Name < devices[j].Name
		}
		return devices[i].ID < devices[j].ID
	})
	return devices, nil
}

func shortID(udid string) string {
	if len(udid) > 8 {
		return udid[:8]
	}
	return udid
}

// lookupDeviceInfo reads the device name and product type from lockdown. It
// returns empty strings when the device does not answer in time (for example
// because it is locked); the device is still listed.
func lookupDeviceInfo(ctx context.Context, entry goios.DeviceEntry) (name, model string) {
	type result struct{ name, model string }
	done := make(chan result, 1)

	go func() {
		values, err := goios.GetValues(entry)
		if err != nil {
			slog.Debug("go-ios get values failed", "udid", entry.Properties.SerialNumber, "error", err)
			done <- result{}
			return
		}
		done <- result{name: strings.TrimSpace(values.Value.DeviceName), model: strings.TrimSpace(values.Value.ProductType)}
	}()

	select {
	case r := <-done:
		return r.name, r.model
	case <-time.After(deviceInfoTimeout):
		slog.Debug("go-ios get values timed out", "udid", entry.Properties.SerialNumber)
		return "", ""
	case <-ctx.Done():
		return "", ""
	}
}

func classifyDiscoveryError(err error) error {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "usbmuxd") || strings.Contains(msg, "27015") ||
		strings.Contains(msg, "connectex") || strings.Contains(msg, "refused") {
		return ErrUsbmuxdMissing
	}
	return fmt.Errorf("could not list devices: %w", err)
}
