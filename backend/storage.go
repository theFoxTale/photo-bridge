package backend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/shirou/gopsutil/v3/disk"
)

const (
	systemDrive      = `C:\`
	diskUsageDomain  = "com.apple.disk_usage"
	phoneInfoTimeout = 4 * time.Second
)

// StorageInfo is the free space of the destination volume and the connected
// iPhone. iPhone numbers are zero when no device is connected or lockdown
// does not answer.
type StorageInfo struct {
	PCFree      int64 `json:"pcFree"`
	IPhoneFree  int64 `json:"iPhoneFree"`
	IPhoneTotal int64 `json:"iPhoneTotal"`
}

// GetStorageInfo reads free space for destPath (C:\ when destPath is empty)
// and, when deviceID is set, free space on the iPhone.
func GetStorageInfo(ctx context.Context, deviceID, destPath string) (StorageInfo, error) {
	if ctx == nil {
		return StorageInfo{}, errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return StorageInfo{}, err
	}

	pc, err := pcFreeBytes(destPath)
	if err != nil {
		return StorageInfo{}, err
	}
	free, total := iphoneStorage(ctx, deviceID)
	return StorageInfo{PCFree: pc, IPhoneFree: free, IPhoneTotal: total}, nil
}

func pcFreeBytes(destPath string) (int64, error) {
	path := strings.TrimSpace(destPath)
	if path == "" {
		path = systemDrive
	}
	existing, err := existingDir(path)
	if err != nil {
		return 0, err
	}
	usage, err := disk.Usage(existing)
	if err != nil {
		return 0, fmt.Errorf("read free space: %w", err)
	}
	if usage.Free > math.MaxInt64 {
		return math.MaxInt64, nil
	}
	return int64(usage.Free), nil
}

// existingDir returns path when it is a directory. A missing path walks up to
// the nearest directory that does exist, so a folder chosen but not yet
// created still reports the volume it will live on.
func existingDir(path string) (string, error) {
	current := filepath.Clean(path)
	for {
		info, err := os.Stat(current)
		if err == nil {
			if info.IsDir() {
				return current, nil
			}
			parent := filepath.Dir(current)
			if parent == current {
				return "", fmt.Errorf("not a folder: %s", path)
			}
			current = parent
			continue
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("cannot access folder: %w", err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("folder does not exist: %s", path)
		}
		current = parent
	}
}

// iphoneStorage reads com.apple.disk_usage over lockdown. Any failure,
// including a missing device, is zero free and zero total.
func iphoneStorage(ctx context.Context, deviceID string) (free, total int64) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" || ctx.Err() != nil {
		return 0, 0
	}

	type result struct{ free, total int64 }
	done := make(chan result, 1)
	go func() {
		free, total := readIPhoneDisk(deviceID)
		done <- result{free, total}
	}()

	select {
	case r := <-done:
		return r.free, r.total
	case <-time.After(phoneInfoTimeout):
		slog.Debug("iPhone disk usage timed out", "device", deviceID)
		return 0, 0
	case <-ctx.Done():
		return 0, 0
	}
}

func readIPhoneDisk(deviceID string) (free, total int64) {
	device, err := goios.GetDevice(deviceID)
	if err != nil {
		slog.Debug("iPhone disk usage: device not found", "device", deviceID, "error", err)
		return 0, 0
	}
	conn, err := goios.ConnectLockdownWithSession(device)
	if err != nil {
		slog.Debug("iPhone disk usage: lockdown failed", "device", deviceID, "error", err)
		return 0, 0
	}
	defer conn.Close()

	value, err := conn.GetValueForDomain("", diskUsageDomain)
	if err != nil {
		slog.Debug("iPhone disk usage: read failed", "device", deviceID, "error", err)
		return 0, 0
	}
	usage, ok := value.(map[string]interface{})
	if !ok {
		slog.Debug("iPhone disk usage: unexpected response", "device", deviceID, "type", fmt.Sprintf("%T", value))
		return 0, 0
	}
	return iphoneDiskBytes(usage)
}

// iphoneDiskBytes reads a com.apple.disk_usage dictionary.
// AmountDataAvailable is the free space. TotalDataAvailable is the size of
// the user-data volume, so a 256 GB phone that is almost full still reports
// about 199 GB there. A missing free-space key stays 0 rather than borrowing
// that volume size. TotalDiskCapacity is the whole disk.
func iphoneDiskBytes(usage map[string]interface{}) (free, total int64) {
	if raw, ok := usage["AmountDataAvailable"]; ok {
		free = plistInt(raw)
	}
	total = plistInt(usage["TotalDiskCapacity"])
	return free, total
}

func plistInt(value interface{}) int64 {
	switch n := value.(type) {
	case uint64:
		if n > math.MaxInt64 {
			return math.MaxInt64
		}
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case uint:
		return int64(n)
	case uint32:
		return int64(n)
	case int32:
		return int64(n)
	case float64:
		if n <= 0 {
			return 0
		}
		if n > math.MaxInt64 {
			return math.MaxInt64
		}
		return int64(n)
	case float32:
		if n <= 0 {
			return 0
		}
		return int64(n)
	default:
		return 0
	}
}
