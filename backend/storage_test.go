package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetStorageInfoForTempDir(t *testing.T) {
	info, err := GetStorageInfo(context.Background(), "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if info.PCFree <= 0 {
		t.Fatalf("pcFree = %d, want a positive byte count", info.PCFree)
	}
	if info.IPhoneFree != 0 || info.IPhoneTotal != 0 {
		t.Fatalf("iPhone = %d/%d, want 0/0 without a device", info.IPhoneFree, info.IPhoneTotal)
	}
}

func TestGetStorageInfoDefaultsToSystemDrive(t *testing.T) {
	info, err := GetStorageInfo(context.Background(), "  ", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if info.PCFree <= 0 {
		t.Fatalf("pcFree = %d, want free space on %s", info.PCFree, systemDrive)
	}
}

func TestGetStorageInfoWalksUpToExistingDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-created", "deeper")
	info, err := GetStorageInfo(context.Background(), "", missing)
	if err != nil {
		t.Fatal(err)
	}
	if info.PCFree <= 0 {
		t.Fatalf("pcFree = %d, want the parent volume", info.PCFree)
	}
}

func TestGetStorageInfoUsesParentOfAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := GetStorageInfo(context.Background(), "", file)
	if err != nil {
		t.Fatal(err)
	}
	if info.PCFree <= 0 {
		t.Fatalf("pcFree = %d, want the folder that contains the file", info.PCFree)
	}
}

func TestGetStorageInfoCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := GetStorageInfo(ctx, "", t.TempDir()); err == nil {
		t.Fatal("expected a canceled context to fail")
	}
}

func TestPlistInt(t *testing.T) {
	if got := plistInt(uint64(42)); got != 42 {
		t.Fatalf("uint64 = %d", got)
	}
	if got := plistInt(float64(99)); got != 99 {
		t.Fatalf("float64 = %d", got)
	}
	if got := plistInt("nope"); got != 0 {
		t.Fatalf("string = %d, want 0", got)
	}
}

func TestIPhoneDiskUsesFreeSpaceNotDataVolume(t *testing.T) {
	// TotalDataAvailable formats as 199.3 GB. That is the data volume, not
	// the space left on a phone Settings shows as 252 GB of 256 GB used.
	usage := map[string]interface{}{
		"TotalDiskCapacity":   uint64(256_000_000_000),
		"TotalDataCapacity":   uint64(214_000_000_000),
		"TotalDataAvailable":  uint64(213_943_058_432),
		"AmountDataAvailable": uint64(4_000_000_000),
	}
	free, total := iphoneDiskBytes(usage)
	if free != 4_000_000_000 {
		t.Fatalf("free = %d, want AmountDataAvailable", free)
	}
	if total != 256_000_000_000 {
		t.Fatalf("total = %d, want TotalDiskCapacity", total)
	}
}

func TestIPhoneDiskFullPhoneStaysZero(t *testing.T) {
	usage := map[string]interface{}{
		"TotalDiskCapacity":   uint64(256_000_000_000),
		"TotalDataAvailable":  uint64(213_943_058_432),
		"AmountDataAvailable": uint64(0),
	}
	free, total := iphoneDiskBytes(usage)
	if free != 0 {
		t.Fatalf("free = %d, want 0 when the data volume has no free space", free)
	}
	if total != 256_000_000_000 {
		t.Fatalf("total = %d, want TotalDiskCapacity", total)
	}
}

func TestIPhoneDiskIgnoresVolumeSizeWithoutFreeKey(t *testing.T) {
	usage := map[string]interface{}{
		"TotalDiskCapacity":  uint64(256_000_000_000),
		"TotalDataAvailable": uint64(213_943_058_432),
	}
	free, total := iphoneDiskBytes(usage)
	if free != 0 || total != 256_000_000_000 {
		t.Fatalf("got %d/%d, want 0/%d", free, total, uint64(256_000_000_000))
	}
}

func TestIPhoneStorageEmptyDevice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	free, total := iphoneStorage(ctx, "  ")
	if free != 0 || total != 0 {
		t.Fatalf("empty device = %d/%d, want 0/0", free, total)
	}
}
