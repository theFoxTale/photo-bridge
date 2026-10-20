package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	compareWorkers          = 3
	compareProgressInterval = 500 * time.Millisecond
	compareHashBuffer       = 1 << 20

	// Phases reported in CompareProgress.
	PhaseScanning = "scanning" // walking the local folder
	PhaseListing  = "listing"  // listing the photos on the device
	PhaseHashing  = "hashing"  // verifying matches by SHA-256
	PhaseDone     = "done"

	reasonContentDiffers = "content differs"
)

// ErrCompareCancelled is returned when a comparison is stopped by the user.
var ErrCompareCancelled = errors.New("comparison cancelled")

// localImageExts are the file types looked at in the local folder.
var localImageExts = map[string]bool{".jpg": true, ".jpeg": true, ".heic": true, ".png": true, ".dng": true}

// deviceImageFormats are the Photo.Format values that are compared. Videos
// are left out on both sides.
var deviceImageFormats = map[string]bool{"heic": true, "jpeg": true, "png": true, "dng": true}

// LocalFile is an image file found in the local folder.
type LocalFile struct {
	Path     string `json:"path"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	ModTime  string `json:"modTime"` // RFC 3339, UTC
}

// Conflict is a photo that has a local file with the same date and size but
// different content.
type Conflict struct {
	Photo     Photo  `json:"photo"`
	LocalPath string `json:"localPath"`
	Reason    string `json:"reason"`
}

// CompareResult is the outcome of comparing a device with a local folder.
type CompareResult struct {
	NewOnDevice []Photo     `json:"newOnDevice"`
	AlreadyOnPC []Photo     `json:"alreadyOnPc"`
	Conflicts   []Conflict  `json:"conflicts"`
	MissingOnPC []LocalFile `json:"missingOnPc"` // on the PC, not on the device
	ScannedAt   string      `json:"scannedAt"`   // RFC 3339, UTC
}

// CompareProgress is the payload of the "compare:progress" event. Total is 0
// while it is still unknown (scanning and listing).
type CompareProgress struct {
	Scanned int    `json:"scanned"`
	Total   int    `json:"total"`
	Phase   string `json:"phase"`
}

type localEntry struct {
	LocalFile
	modTime time.Time
}

// hashResult is the outcome of hashing one file.
type hashResult struct {
	sum string
	err error
}

// compareTracker holds the numbers behind CompareProgress.
type compareTracker struct {
	mu      sync.Mutex
	phase   string
	scanned int
	total   int
}

func (t *compareTracker) setPhase(phase string, scanned, total int) {
	t.mu.Lock()
	t.phase, t.scanned, t.total = phase, scanned, total
	t.mu.Unlock()
}

func (t *compareTracker) advance() {
	t.mu.Lock()
	t.scanned++
	t.mu.Unlock()
}

func (t *compareTracker) snapshot() CompareProgress {
	t.mu.Lock()
	defer t.mu.Unlock()
	return CompareProgress{Scanned: t.scanned, Total: t.total, Phase: t.phase}
}

// signature is the cheap first-pass key: the modification time to the second
// plus the size.
func signature(t time.Time, size int64) string {
	return fmt.Sprintf("%d_%d", t.Unix(), size)
}

// photoSignature returns the signature of a device photo. Photos without a
// usable date cannot be matched.
func photoSignature(p Photo) (string, time.Time, bool) {
	t, err := time.Parse(time.RFC3339, p.Date)
	if err != nil {
		return "", time.Time{}, false
	}
	return signature(t, p.Size), t, true
}

// RunCompare compares the camera roll of a device with the images below
// localPath.
//
// Files are first matched by signature (date to the second plus size). Only
// the matches are verified by SHA-256: the local file is read from disk and
// the device file is streamed from the phone. Sums are cached in cache (which
// may be nil), so repeated runs only hash what changed. emit (optional)
// receives progress about every compareProgressInterval. Cancelling ctx
// returns ErrCompareCancelled.
func RunCompare(ctx context.Context, lib Library, cache *HashCache, deviceID, localPath string, emit func(CompareProgress)) (CompareResult, error) {
	root, err := validateCompare(deviceID, localPath)
	if err != nil {
		return emptyCompareResult(), err
	}

	tracker := &compareTracker{phase: PhaseScanning}
	done := make(chan struct{})
	var reporter sync.WaitGroup
	if emit != nil {
		reporter.Add(1)
		go func() {
			defer reporter.Done()
			ticker := time.NewTicker(compareProgressInterval)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					emit(tracker.snapshot())
				}
			}
		}()
	}
	finish := func() {
		close(done)
		reporter.Wait()
		if emit != nil {
			emit(tracker.snapshot())
		}
	}

	result, err := runCompare(ctx, lib, cache, deviceID, root, tracker)
	if err == nil {
		tracker.setPhase(PhaseDone, tracker.snapshot().Scanned, tracker.snapshot().Total)
	}
	finish()

	if err != nil {
		if ctx.Err() != nil {
			return emptyCompareResult(), ErrCompareCancelled
		}
		return emptyCompareResult(), err
	}
	return result, nil
}

func validateCompare(deviceID, localPath string) (string, error) {
	if strings.TrimSpace(deviceID) == "" {
		return "", errors.New("deviceID must not be empty")
	}
	if strings.TrimSpace(localPath) == "" {
		return "", errors.New("choose a folder to compare with")
	}
	abs, err := filepath.Abs(strings.TrimSpace(localPath))
	if err != nil {
		return "", fmt.Errorf("invalid folder: %w", err)
	}
	info, err := os.Stat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("folder does not exist: %s", abs)
	case err != nil:
		return "", fmt.Errorf("cannot access folder: %w", err)
	case !info.IsDir():
		return "", fmt.Errorf("not a folder: %s", abs)
	}
	return abs, nil
}

func emptyCompareResult() CompareResult {
	return CompareResult{
		NewOnDevice: []Photo{},
		AlreadyOnPC: []Photo{},
		Conflicts:   []Conflict{},
		MissingOnPC: []LocalFile{},
	}
}

func runCompare(ctx context.Context, lib Library, cache *HashCache, deviceID, root string, tracker *compareTracker) (CompareResult, error) {
	// 1. The local folder.
	locals, err := scanLocalImages(ctx, root, tracker)
	if err != nil {
		return CompareResult{}, err
	}

	// 2. The device.
	tracker.setPhase(PhaseListing, 0, 0)
	listed, err := lib.Photos(ctx, deviceID, cameraRollID)
	if err != nil {
		return CompareResult{}, fmt.Errorf("list the photos on the device: %w", err)
	}
	photos := make([]Photo, 0, len(listed))
	for _, p := range listed {
		if deviceImageFormats[p.Format] {
			photos = append(photos, p)
		}
	}

	// 3. First pass: candidates by signature.
	bySignature := make(map[string][]*localEntry, len(locals))
	for i := range locals {
		key := signature(locals[i].modTime, locals[i].Size)
		bySignature[key] = append(bySignature[key], &locals[i])
	}
	candidates := make(map[string][]*localEntry) // photo ID -> local candidates
	for _, p := range photos {
		if key, _, ok := photoSignature(p); ok {
			if found := bySignature[key]; len(found) > 0 {
				candidates[p.ID] = found
			}
		}
	}

	// 4. Second pass: hash only the candidates.
	deviceSums, localSums, err := hashCandidates(ctx, lib, cache, deviceID, photos, candidates, tracker)
	if err != nil {
		return CompareResult{}, err
	}

	// 5. Classify.
	result := emptyCompareResult()
	claimed := make(map[string]bool) // local paths that belong to a device photo
	heicStems := make(map[string]bool)
	for _, p := range photos {
		if p.Format == "heic" {
			heicStems[strings.ToLower(strings.TrimSuffix(p.Filename, filepath.Ext(p.Filename)))] = true
		}
	}

	for _, p := range photos {
		var free []*localEntry
		for _, c := range candidates[p.ID] {
			if !claimed[c.Path] {
				free = append(free, c)
			}
		}
		if len(free) == 0 {
			result.NewOnDevice = append(result.NewOnDevice, p)
			continue
		}

		dev := deviceSums[p.ID]
		var matched *localEntry
		if dev.err == nil {
			for _, c := range free {
				if l := localSums[c.Path]; l.err == nil && l.sum == dev.sum {
					matched = c
					break
				}
			}
		}
		if matched != nil {
			claimed[matched.Path] = true
			result.AlreadyOnPC = append(result.AlreadyOnPC, p)
			continue
		}

		reason := reasonContentDiffers
		if dev.err != nil {
			reason = "could not read the file on the iPhone: " + dev.err.Error()
		} else {
			for _, c := range free {
				if l := localSums[c.Path]; l.err != nil {
					reason = "could not read the file on the PC: " + l.err.Error()
					break
				}
			}
		}
		for _, c := range free {
			claimed[c.Path] = true
		}
		result.Conflicts = append(result.Conflicts, Conflict{Photo: p, LocalPath: free[0].Path, Reason: reason})
	}

	for _, l := range locals {
		if claimed[l.Path] {
			continue
		}
		// A JPG next to a HEIC of the same name is most likely the copy this
		// app converted; it cannot match anything on the phone by design.
		ext := strings.ToLower(filepath.Ext(l.Filename))
		stem := strings.ToLower(strings.TrimSuffix(l.Filename, filepath.Ext(l.Filename)))
		if (ext == ".jpg" || ext == ".jpeg") && heicStems[stem] {
			continue
		}
		result.MissingOnPC = append(result.MissingOnPC, l.LocalFile)
	}
	sort.Slice(result.MissingOnPC, func(i, j int) bool {
		return strings.ToLower(result.MissingOnPC[i].Path) < strings.ToLower(result.MissingOnPC[j].Path)
	})

	result.ScannedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

// scanLocalImages walks root and returns every image file below it. Hidden
// folders and files (including this app's temporary transfer folders) and
// anything that cannot be read are skipped.
func scanLocalImages(ctx context.Context, root string, tracker *compareTracker) ([]localEntry, error) {
	tracker.setPhase(PhaseScanning, 0, 0)
	var entries []localEntry

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			if path == root {
				return walkErr
			}
			slog.Debug("compare: skipping unreadable path", "path", path, "error", walkErr)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if !localImageExts[strings.ToLower(filepath.Ext(d.Name()))] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			slog.Debug("compare: skipping unreadable file", "path", path, "error", err)
			return nil
		}
		entries = append(entries, localEntry{
			LocalFile: LocalFile{
				Path:     path,
				Filename: d.Name(),
				Size:     info.Size(),
				ModTime:  info.ModTime().UTC().Format(time.RFC3339),
			},
			modTime: info.ModTime(),
		})
		tracker.advance()
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", root, err)
	}
	return entries, nil
}

// hashJob is one file to hash: a device photo or a local file.
type hashJob struct {
	photo *Photo      // set for device files
	local *localEntry // set for local files
}

// hashCandidates computes the SHA-256 of every device photo that has local
// candidates, and of those candidates. Results are keyed by photo ID and by
// local path. Individual failures are kept in the results; only cancellation
// aborts.
func hashCandidates(
	ctx context.Context,
	lib Library,
	cache *HashCache,
	deviceID string,
	photos []Photo,
	candidates map[string][]*localEntry,
	tracker *compareTracker,
) (deviceSums, localSums map[string]hashResult, err error) {
	var jobs []hashJob
	seenLocal := make(map[string]bool)
	for i := range photos {
		list := candidates[photos[i].ID]
		if len(list) == 0 {
			continue
		}
		jobs = append(jobs, hashJob{photo: &photos[i]})
		for _, c := range list {
			if !seenLocal[c.Path] {
				seenLocal[c.Path] = true
				jobs = append(jobs, hashJob{local: c})
			}
		}
	}

	deviceSums = make(map[string]hashResult)
	localSums = make(map[string]hashResult)
	tracker.setPhase(PhaseHashing, 0, len(jobs))
	if len(jobs) == 0 {
		return deviceSums, localSums, nil
	}

	queue := make(chan hashJob, len(jobs))
	for _, j := range jobs {
		queue <- j
	}
	close(queue)

	var mu sync.Mutex
	var workers sync.WaitGroup
	for w := 0; w < min(compareWorkers, len(jobs)); w++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range queue {
				if ctx.Err() != nil {
					return
				}
				var res hashResult
				if job.photo != nil {
					res.sum, res.err = hashDevicePhoto(ctx, lib, cache, deviceID, *job.photo)
				} else {
					res.sum, res.err = hashLocalFile(ctx, cache, *job.local)
				}
				if ctx.Err() != nil {
					return
				}
				mu.Lock()
				if job.photo != nil {
					deviceSums[job.photo.ID] = res
				} else {
					localSums[job.local.Path] = res
				}
				mu.Unlock()
				tracker.advance()
			}
		}()
	}
	workers.Wait()

	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return deviceSums, localSums, nil
}

func hashLocalFile(ctx context.Context, cache *HashCache, e localEntry) (string, error) {
	key := "local:" + strings.ToLower(e.Path)
	mod := e.modTime.UnixNano()
	if sum, ok := cache.Get(key, e.Size, mod); ok {
		return sum, nil
	}

	f, err := os.Open(e.Path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", e.Filename, err)
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, compareHashBuffer)
	if _, err := io.CopyBuffer(h, &contextReader{ctx: ctx, r: f}, buf); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("read %s: %w", e.Filename, err)
	}
	sum := hex.EncodeToString(h.Sum(nil))
	cache.Put(key, e.Size, mod, sum)
	return sum, nil
}

func hashDevicePhoto(ctx context.Context, lib Library, cache *HashCache, deviceID string, p Photo) (string, error) {
	key := "device:" + deviceID + "/" + p.ID
	var mod int64
	if t, err := time.Parse(time.RFC3339, p.Date); err == nil {
		mod = t.UnixNano()
	}
	if sum, ok := cache.Get(key, p.Size, mod); ok {
		return sum, nil
	}

	sum, err := lib.Hash(ctx, deviceID, p.ID, nil)
	if err != nil {
		return "", err
	}
	cache.Put(key, p.Size, mod, sum)
	return sum, nil
}

// contextReader stops reading once its context is cancelled.
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *contextReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
