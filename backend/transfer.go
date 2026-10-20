package backend

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// transferWorkers is how many photos are processed at once. Each worker
	// downloads over its own connection, so this is also the number of
	// parallel downloads; more would only hammer a Wi-Fi link.
	transferWorkers = 3

	maxTransferPhotos = 100_000
	progressInterval  = 200 * time.Millisecond
	maxNameSuffix     = 1000
	tempDirPrefix     = ".photo-bridge-"
)

// TransferOptions controls what TransferPhotos writes and where.
type TransferOptions struct {
	SaveOriginal bool   `json:"saveOriginal"`
	SaveJPG      bool   `json:"saveJpg"`
	JPGQuality   int    `json:"jpgQuality"` // 1-100
	DestPath     string `json:"destPath"`

	// OverwritePaths maps a photo ID to an existing file inside DestPath that
	// its original replaces (used to resolve comparison conflicts). Photos not
	// listed keep both files: an existing file with another size is never
	// touched and the new one gets a "_1" style suffix.
	OverwritePaths map[string]string `json:"overwritePaths"`
}

// TransferError describes one photo that could not be transferred.
type TransferError struct {
	PhotoID string `json:"photoId"`
	Reason  string `json:"reason"`
}

// TransferResult summarises a finished (or cancelled) transfer. Photos whose
// files already existed in the destination count as succeeded.
type TransferResult struct {
	Succeeded   []string        `json:"succeeded"`
	Failed      []TransferError `json:"failed"`
	BytesCopied int64           `json:"bytesCopied"`
	DurationMs  int64           `json:"durationMs"`
	// Cancelled is true when the transfer was stopped before every photo was
	// processed. The lists above then cover the photos handled until then.
	Cancelled bool `json:"cancelled"`
}

// TransferProgress is the payload of the "transfer:progress" event.
type TransferProgress struct {
	Completed   int    `json:"completed"`
	Total       int    `json:"total"`
	BytesCopied int64  `json:"bytesCopied"`
	BytesPerSec int64  `json:"bytesPerSec"`
	CurrentFile string `json:"currentFile"`
}

// validateTransfer checks a request and returns the de-duplicated photo IDs
// and the options with an absolute destination path.
func validateTransfer(deviceID string, photoIDs []string, opts TransferOptions) ([]string, TransferOptions, error) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, opts, errors.New("deviceID must not be empty")
	}
	if len(photoIDs) == 0 {
		return nil, opts, errors.New("no photos selected")
	}
	if len(photoIDs) > maxTransferPhotos {
		return nil, opts, fmt.Errorf("too many photos selected (%d, the limit is %d)", len(photoIDs), maxTransferPhotos)
	}

	seen := make(map[string]struct{}, len(photoIDs))
	ids := make([]string, 0, len(photoIDs))
	for _, id := range photoIDs {
		if strings.TrimSpace(id) == "" {
			return nil, opts, errors.New("photo IDs must not be empty")
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	if !opts.SaveOriginal && !opts.SaveJPG {
		return nil, opts, errors.New("choose at least one format to save: original or JPG")
	}
	if opts.JPGQuality < 1 || opts.JPGQuality > 100 {
		return nil, opts, fmt.Errorf("JPG quality must be between 1 and 100, got %d", opts.JPGQuality)
	}

	dest := strings.TrimSpace(opts.DestPath)
	if dest == "" {
		return nil, opts, errors.New("choose a destination folder")
	}
	abs, err := filepath.Abs(dest)
	if err != nil {
		return nil, opts, fmt.Errorf("invalid destination folder: %w", err)
	}
	info, err := os.Stat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, opts, fmt.Errorf("destination folder does not exist: %s", abs)
	case err != nil:
		return nil, opts, fmt.Errorf("cannot access destination folder: %w", err)
	case !info.IsDir():
		return nil, opts, fmt.Errorf("destination is not a folder: %s", abs)
	}
	opts.DestPath = abs

	overwrite, err := validateOverwritePaths(ids, abs, opts.OverwritePaths)
	if err != nil {
		return nil, opts, err
	}
	opts.OverwritePaths = overwrite
	return ids, opts, nil
}

// validateOverwritePaths checks that every overwrite target belongs to a
// transferred photo, is an existing image file, and lies inside dest, so a
// request can never replace files elsewhere on the disk.
func validateOverwritePaths(ids []string, dest string, paths map[string]string) (map[string]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}

	clean := make(map[string]string, len(paths))
	for id, target := range paths {
		if _, ok := selected[id]; !ok {
			return nil, fmt.Errorf("overwrite target given for a photo that is not being transferred: %s", id)
		}
		abs, err := filepath.Abs(strings.TrimSpace(target))
		if err != nil {
			return nil, fmt.Errorf("invalid overwrite path: %w", err)
		}
		rel, err := filepath.Rel(dest, abs)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("overwrite target is outside the destination folder: %s", abs)
		}
		if !localImageExts[strings.ToLower(filepath.Ext(abs))] {
			return nil, fmt.Errorf("overwrite target is not an image file: %s", abs)
		}
		info, err := os.Stat(abs)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("overwrite target does not exist: %s", abs)
		}
		clean[id] = abs
	}
	return clean, nil
}

// transferRun is the state shared by the workers of one transfer.
type transferRun struct {
	lib      Library
	deviceID string
	opts     TransferOptions
	tmpDir   string
	total    int

	completed atomic.Int64
	bytes     atomic.Int64

	mu       sync.Mutex // guards the fields below
	current  string
	result   TransferResult
	reserved map[string]struct{} // destination paths claimed during this run
}

// RunTransfer copies the photos to opts.DestPath using transferWorkers
// workers. emit (optional) receives a TransferProgress about every
// progressInterval and once more when the run ends. Cancelling ctx stops the
// run; the result then has Cancelled set and a nil error. Each photo is
// written through a temporary file inside the destination folder, so an
// interrupted transfer never leaves a half-written photo behind.
func RunTransfer(ctx context.Context, lib Library, deviceID string, photoIDs []string, opts TransferOptions, emit func(TransferProgress)) (TransferResult, error) {
	ids, opts, err := validateTransfer(deviceID, photoIDs, opts)
	if err != nil {
		return emptyResult(), err
	}

	tmpDir, err := os.MkdirTemp(opts.DestPath, tempDirPrefix)
	if err != nil {
		return emptyResult(), fmt.Errorf("cannot write to the destination folder: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	started := time.Now()
	run := &transferRun{
		lib:      lib,
		deviceID: deviceID,
		opts:     opts,
		tmpDir:   tmpDir,
		total:    len(ids),
		result:   emptyResult(),
		reserved: make(map[string]struct{}),
	}

	done := make(chan struct{})
	var reporter sync.WaitGroup
	reporter.Add(1)
	go func() {
		defer reporter.Done()
		run.report(emit, started, done)
	}()

	jobs := make(chan string, len(ids))
	for _, id := range ids {
		jobs <- id
	}
	close(jobs)

	var workers sync.WaitGroup
	for slot := 0; slot < min(transferWorkers, len(ids)); slot++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for id := range jobs {
				if ctx.Err() != nil {
					return
				}
				run.handle(ctx, slot, id)
			}
		}()
	}
	workers.Wait()
	close(done)
	reporter.Wait()

	run.mu.Lock()
	result := run.result
	run.mu.Unlock()
	result.BytesCopied = run.bytes.Load()
	result.DurationMs = time.Since(started).Milliseconds()
	result.Cancelled = ctx.Err() != nil

	if emit != nil {
		emit(run.snapshot(avgRate(result.BytesCopied, started)))
	}
	return result, nil
}

func emptyResult() TransferResult {
	return TransferResult{Succeeded: []string{}, Failed: []TransferError{}}
}

func avgRate(bytes int64, since time.Time) int64 {
	elapsed := time.Since(since).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return int64(float64(bytes) / elapsed)
}

func (r *transferRun) snapshot(rate int64) TransferProgress {
	r.mu.Lock()
	current := r.current
	r.mu.Unlock()
	return TransferProgress{
		Completed:   int(r.completed.Load()),
		Total:       r.total,
		BytesCopied: r.bytes.Load(),
		BytesPerSec: rate,
		CurrentFile: current,
	}
}

// report emits progress on a ticker until done is closed. The speed is a
// smoothed average over recent ticks, so it reacts to stalls without jitter.
func (r *transferRun) report(emit func(TransferProgress), started time.Time, done <-chan struct{}) {
	if emit == nil {
		return
	}
	ticker := time.NewTicker(progressInterval)
	defer ticker.Stop()

	lastTime := started
	var lastBytes int64
	var rate float64
	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			bytes := r.bytes.Load()
			if dt := now.Sub(lastTime).Seconds(); dt > 0 {
				instant := float64(bytes-lastBytes) / dt
				if rate == 0 {
					rate = instant
				} else {
					rate = 0.7*rate + 0.3*instant
				}
			}
			lastTime, lastBytes = now, bytes
			emit(r.snapshot(int64(max(rate, 0))))
		}
	}
}

// handle processes one photo and records the outcome.
func (r *transferRun) handle(ctx context.Context, slot int, id string) {
	err := r.process(ctx, slot, id)
	if err != nil && ctx.Err() != nil {
		return // stopped by the user: neither a success nor a failure
	}

	r.mu.Lock()
	if err != nil {
		slog.Debug("transfer failed for photo", "photo", id, "error", err)
		r.result.Failed = append(r.result.Failed, TransferError{PhotoID: id, Reason: err.Error()})
	} else {
		r.result.Succeeded = append(r.result.Succeeded, id)
	}
	r.mu.Unlock()
	r.completed.Add(1)
}

// photoPlan says which files one photo needs.
type photoPlan struct {
	originalPath string
	saveOriginal bool
	jpgPath      string
	saveJPG      bool
}

func (r *transferRun) process(ctx context.Context, slot int, id string) error {
	info, err := r.lib.Describe(ctx, r.deviceID, id)
	if err != nil {
		return err
	}
	name := sanitizeFilename(info.Filename)
	if name == "" {
		return errors.New("the photo has no file name")
	}
	r.mu.Lock()
	r.current = name
	r.mu.Unlock()

	plan, err := r.plan(info, name)
	if err != nil {
		return err
	}
	if !plan.saveOriginal && !plan.saveJPG {
		return nil // everything is already in the destination
	}

	part := filepath.Join(r.tmpDir, fmt.Sprintf("worker%d.part", slot))
	defer os.Remove(part)
	if err := r.lib.Download(ctx, r.deviceID, id, part, func(n int64) { r.bytes.Add(n) }); err != nil {
		return err
	}

	// The original goes first: if the conversion fails, the original is
	// still saved, and a re-run skips it and retries only the JPG.
	source := part
	if plan.saveOriginal {
		if err := os.Rename(part, plan.originalPath); err != nil {
			return fmt.Errorf("save original: %w", err)
		}
		source = plan.originalPath
		touch(plan.originalPath, info.Date)
	}

	if plan.saveJPG {
		tmpJPG := filepath.Join(r.tmpDir, fmt.Sprintf("worker%d.jpg", slot))
		defer os.Remove(tmpJPG)
		if err := convertToJPEG(source, tmpJPG, r.opts.JPGQuality); err != nil {
			return fmt.Errorf("convert to JPG: %w", err)
		}
		if err := os.Rename(tmpJPG, plan.jpgPath); err != nil {
			return fmt.Errorf("save JPG: %w", err)
		}
		touch(plan.jpgPath, info.Date)
	}
	return nil
}

// plan decides which outputs a photo needs and where they go, skipping files
// that are already present.
func (r *transferRun) plan(info Photo, name string) (photoPlan, error) {
	var p photoPlan

	jpgCapable := info.Format == "heic" || info.Format == "jpeg"
	wantOriginal := r.opts.SaveOriginal
	wantJPG := r.opts.SaveJPG && jpgCapable
	if r.opts.SaveJPG && !jpgCapable && !wantOriginal {
		return p, fmt.Errorf("JPG conversion is not supported for %s files", strings.ToUpper(info.Format))
	}
	// A JPEG original already is a JPG; a second, re-encoded copy would only
	// be a worse duplicate with a clashing name.
	if info.Format == "jpeg" && wantOriginal {
		wantJPG = false
	}

	if wantOriginal {
		if target, ok := r.opts.OverwritePaths[info.ID]; ok {
			// A conflict the user chose to resolve by replacing the local file.
			p.originalPath, p.saveOriginal = target, true
		} else {
			path, skip, err := r.claimPath(name, func(fi fs.FileInfo) bool { return fi.Size() == info.Size })
			if err != nil {
				return p, err
			}
			p.originalPath, p.saveOriginal = path, !skip
		}
	}
	if wantJPG {
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		path, skip, err := r.claimPath(stem+".jpg", func(fs.FileInfo) bool { return true })
		if err != nil {
			return p, err
		}
		p.jpgPath, p.saveJPG = path, !skip
	}
	return p, nil
}

// claimPath picks the destination path for name. A free path is reserved and
// returned. An existing file for which same returns true means the work is
// already done (skip). Any other existing file, or a path claimed earlier in
// this run, makes the search continue with "name_1.ext", "name_2.ext", ...
func (r *transferRun) claimPath(name string, same func(fs.FileInfo) bool) (string, bool, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)

	r.mu.Lock()
	defer r.mu.Unlock()

	for n := 0; n <= maxNameSuffix; n++ {
		candidate := name
		if n > 0 {
			candidate = fmt.Sprintf("%s_%d%s", stem, n, ext)
		}
		path := filepath.Join(r.opts.DestPath, candidate)
		key := strings.ToLower(path) // Windows file names are case-insensitive
		if _, taken := r.reserved[key]; taken {
			continue
		}

		fi, err := os.Stat(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			r.reserved[key] = struct{}{}
			return path, false, nil
		case err != nil:
			return "", false, fmt.Errorf("cannot check %s: %w", candidate, err)
		case !fi.IsDir() && same(fi):
			r.reserved[key] = struct{}{}
			return path, true, nil
		}
	}
	return "", false, fmt.Errorf("too many files named %s in the destination folder", name)
}

// sanitizeFilename makes a device file name safe to create on Windows.
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"<", "_", ">", "_", ":", "_", `"`, "_", "/", "_", `\`, "_", "|", "_", "?", "_", "*", "_",
	)
	clean := strings.TrimRight(strings.TrimSpace(replacer.Replace(name)), ". ")
	if clean == "" || clean == "." || clean == ".." {
		return ""
	}
	return clean
}

// touch gives a written file the photo's date. Failure is not worth reporting.
func touch(path, date string) {
	t, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return
	}
	if err := os.Chtimes(path, t, t); err != nil {
		slog.Debug("could not set file time", "path", path, "error", err)
	}
}
