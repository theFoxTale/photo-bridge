package backend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/afc"
)

// Known limitations of this stage (see README):
//   - Only "Camera Roll" (everything under /DCIM) and one album per *APPLE
//     roll folder are listed. Favorites, Screenshots and user albums live in
//     PhotoData/Photos.sqlite, which is not parsed yet.
//   - Live Photos appear as two separate files (image + MOV).
//   - Previews are resized JPEGs in the local thumbnail cache. DNG and MOV
//     have no preview. FetchThumbnail still returns only a short file prefix
//     and is not used for previews.

const (
	afcService = "com.apple.afc"

	// The AFC service is rooted at /var/mobile/Media, so the photo folder is
	// addressed as /DCIM, not /var/mobile/Media/DCIM.
	dcimRoot = "/DCIM"

	cameraRollID = "camera-roll"
	rollIDPrefix = "roll-"

	// thumbnailPrefixBytes is how much of a file FetchThumbnail downloads.
	thumbnailPrefixBytes = 64 * 1024

	// downloadChunkBytes is the size of one AFC read request while downloading.
	downloadChunkBytes = 256 * 1024

	maxWalkDepth = 6
	maxIDLength  = 512
)

// User-facing errors. Raw go-ios errors are logged at debug level only.
var (
	ErrTunnelNotRunning = errors.New("Device tunnel not running. Run `ios tunnel start` in an elevated terminal.")
	ErrAFCUnavailable   = errors.New("Could not open AFC connection. Check that the device is unlocked and Wi-Fi Sync is enabled.")
	ErrDeviceNotFound   = errors.New("Device not found. Check that it is connected, awake and on the same network.")
	ErrNotPaired        = errors.New("Device is not paired with this computer. Connect it via USB once and tap Trust.")
	ErrUsbmuxdMissing   = errors.New("Could not reach usbmuxd. Install Apple Devices (or iTunes) and make sure its device service is running.")
	ErrConnectionLost   = errors.New("Lost the connection to the device. Check that it is unlocked and still reachable.")
)

// PhotoLibrary reads albums, photos and thumbnails from a device. The AFC
// implementation can be swapped for another one (for example a
// Photos.sqlite based one) without touching the callers.
type PhotoLibrary interface {
	FetchAlbums(deviceID string) ([]Album, error)
	FetchPhotos(deviceID, albumID string) ([]Photo, error)
	FetchThumbnail(deviceID, photoID string) ([]byte, error)

	// DescribePhoto returns the metadata (name, size, format, date) of one
	// photo without listing its album.
	DescribePhoto(deviceID, photoID string) (Photo, error)

	// DownloadPhoto streams the photo to destPath without holding it in
	// memory. It uses a connection of its own, so it neither blocks nor is
	// blocked by listing and thumbnail requests. onBytes (optional) receives
	// the size of every chunk written. A cancelled ctx stops the download
	// between chunks and removes the partial file.
	DownloadPhoto(ctx context.Context, deviceID, photoID, destPath string, onBytes func(n int64)) error

	// HashPhoto returns the hex SHA-256 of the photo, read in a stream from
	// the device. Nothing is written to disk.
	HashPhoto(ctx context.Context, deviceID, photoID string, onBytes func(n int64)) (string, error)
}

// mediaFormats maps lower-case file extensions to Photo.Format values.
var mediaFormats = map[string]string{
	"heic": "heic",
	"jpg":  "jpeg",
	"jpeg": "jpeg",
	"png":  "png",
	"dng":  "dng",
	"mov":  "mov",
}

var filenameDate = regexp.MustCompile(`(\d{4})(\d{2})(\d{2})[_-](\d{2})(\d{2})(\d{2})`)

// permanentError marks failures caused by the request itself. They never
// trigger a reconnect.
type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

func permanentf(format string, args ...any) error {
	return permanentError{err: fmt.Errorf(format, args...)}
}

// isAFCStatusError reports whether err is an error status answered by the
// device (missing file, permission denied, ...). The connection is still fine.
func isAFCStatusError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "afc error code")
}

// classifyConnectError converts a go-ios connection error into a user-facing
// one. The raw error is logged at debug level.
func classifyConnectError(err error) error {
	slog.Debug("go-ios connect error", "error", err)
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "tunnel"):
		return ErrTunnelNotRunning
	case strings.Contains(msg, "usbmuxd"), strings.Contains(msg, "27015"), strings.Contains(msg, "connectex"):
		return ErrUsbmuxdMissing
	case strings.Contains(msg, "pair"):
		return ErrNotPaired
	case strings.Contains(msg, "not found"), strings.Contains(msg, "no ios devices"):
		return ErrDeviceNotFound
	default:
		return ErrAFCUnavailable
	}
}

// ---------------------------------------------------------------------------
// Session: one AFC connection to one device.
// ---------------------------------------------------------------------------

const (
	afcMagic       uint64 = 0x4141504c36414643 // "CFA6LPAA"
	afcHeaderSize  uint64 = 40
	afcOpStatus    uint64 = 0x01
	afcOpData      uint64 = 0x02
	afcOpFileInfo  uint64 = 0x0A
	afcMaxResponse uint64 = 1 << 20
)

type afcHeader struct {
	Magic     uint64
	EntireLen uint64
	ThisLen   uint64
	PacketNum uint64
	Operation uint64
}

type fileStat struct {
	isDir bool
	size  int64
	mtime time.Time
}

type afcSession struct {
	mu        sync.Mutex // serialises requests; AFC is strictly request/response
	conn      goios.DeviceConnectionInterface
	client    *afc.Client
	packetNum atomic.Uint64
}

func (s *afcSession) close() {
	_ = s.client.Close()
}

// stat issues an AFC file-info request. go-ios' own Stat drops st_mtime, which
// the photo date needs, so the (tiny) request is spoken here on the same
// connection. Callers must hold s.mu.
func (s *afcSession) stat(p string) (fileStat, error) {
	headerPayload := []byte(p)
	length := afcHeaderSize + uint64(len(headerPayload))

	var request bytes.Buffer
	if err := binary.Write(&request, binary.LittleEndian, afcHeader{
		Magic:     afcMagic,
		EntireLen: length,
		ThisLen:   length,
		PacketNum: s.packetNum.Add(1),
		Operation: afcOpFileInfo,
	}); err != nil {
		return fileStat{}, err
	}
	request.Write(headerPayload)
	if _, err := s.conn.Write(request.Bytes()); err != nil {
		return fileStat{}, fmt.Errorf("send stat request: %w", err)
	}

	var h afcHeader
	if err := binary.Read(s.conn, binary.LittleEndian, &h); err != nil {
		return fileStat{}, fmt.Errorf("read stat header: %w", err)
	}
	if h.ThisLen < afcHeaderSize || h.EntireLen < h.ThisLen || h.EntireLen > afcMaxResponse {
		return fileStat{}, fmt.Errorf("malformed stat response (this=%d entire=%d)", h.ThisLen, h.EntireLen)
	}
	body := make([]byte, h.EntireLen-afcHeaderSize)
	if _, err := io.ReadFull(s.conn, body); err != nil {
		return fileStat{}, fmt.Errorf("read stat body: %w", err)
	}
	headerPayloadLen := h.ThisLen - afcHeaderSize

	switch h.Operation {
	case afcOpStatus:
		if headerPayloadLen < 8 {
			return fileStat{}, errors.New("malformed stat status response")
		}
		return fileStat{}, fmt.Errorf("afc error code: %d", binary.LittleEndian.Uint64(body))
	case afcOpData:
		return parseStat(body[headerPayloadLen:]), nil
	default:
		return fileStat{}, fmt.Errorf("unexpected stat response operation %d", h.Operation)
	}
}

// parseStat reads the NUL separated key/value pairs of a file-info response.
func parseStat(payload []byte) fileStat {
	var st fileStat
	parts := bytes.Split(payload, []byte{0})
	for i := 0; i+1 < len(parts); i += 2 {
		key, value := string(parts[i]), string(parts[i+1])
		switch key {
		case "st_ifmt":
			st.isDir = value == "S_IFDIR"
		case "st_size":
			if n, err := strconv.ParseInt(value, 10, 64); err == nil {
				st.size = n
			}
		case "st_mtime":
			st.mtime = parseAFCTime(value)
		}
	}
	return st
}

// parseAFCTime converts an epoch value of unknown unit (devices report
// nanoseconds) into a time. It returns the zero time when it cannot parse.
func parseAFCTime(value string) time.Time {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	switch {
	case n > 1e17:
		return time.Unix(0, n).UTC()
	case n > 1e14:
		return time.UnixMicro(n).UTC()
	case n > 1e11:
		return time.UnixMilli(n).UTC()
	default:
		return time.Unix(n, 0).UTC()
	}
}

// collectMedia returns the absolute paths of all media files below dir.
// Callers must hold s.mu.
func (s *afcSession) collectMedia(dir string, depth int) ([]string, error) {
	if depth > maxWalkDepth {
		return nil, nil
	}
	entries, err := s.client.List(dir)
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)

	var found []string
	for _, name := range entries {
		if strings.HasPrefix(name, ".") {
			continue // hidden files
		}
		full := dir + "/" + name
		if _, ok := formatOf(name); ok {
			found = append(found, full)
			continue
		}
		if strings.Contains(name, ".") {
			continue // other files, for example *.AAE sidecars
		}
		// No extension: probably a sub directory, so look inside.
		st, err := s.stat(full)
		if err != nil {
			if isAFCStatusError(err) {
				continue
			}
			return nil, err
		}
		if !st.isDir {
			continue
		}
		sub, err := s.collectMedia(full, depth+1)
		if err != nil {
			if isAFCStatusError(err) {
				continue
			}
			return nil, err
		}
		found = append(found, sub...)
	}
	return found, nil
}

func formatOf(name string) (string, bool) {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	format, ok := mediaFormats[ext]
	return format, ok
}

// ---------------------------------------------------------------------------
// AFCPhotoLibrary
// ---------------------------------------------------------------------------

// AFCPhotoLibrary implements PhotoLibrary with go-ios over the Apple File
// Connection service. It keeps one AFC connection per device ID.
type AFCPhotoLibrary struct {
	mu       sync.Mutex
	sessions map[string]*afcSession
	closed   bool

	// transferIdle holds connections reserved for downloads, per device. They
	// are separate from sessions so that a long download never blocks listing
	// or thumbnails, and so that parallel downloads do not queue on one
	// connection.
	transferIdle map[string][]*afcSession
}

// NewAFCPhotoLibrary creates an empty library; connections open on demand.
func NewAFCPhotoLibrary() *AFCPhotoLibrary {
	return &AFCPhotoLibrary{
		sessions:     make(map[string]*afcSession),
		transferIdle: make(map[string][]*afcSession),
	}
}

// Close closes every cached AFC connection. The library cannot be used after.
func (l *AFCPhotoLibrary) Close() error {
	l.mu.Lock()
	sessions := l.sessions
	idle := l.transferIdle
	l.sessions = make(map[string]*afcSession)
	l.transferIdle = make(map[string][]*afcSession)
	l.closed = true
	l.mu.Unlock()

	for _, s := range sessions {
		s.close()
	}
	for _, list := range idle {
		for _, s := range list {
			s.close()
		}
	}
	return nil
}

// connect opens a new AFC connection to the device.
func connect(deviceID string) (*afcSession, error) {
	device, err := goios.GetDevice(deviceID)
	if err != nil {
		return nil, classifyConnectError(err)
	}
	conn, err := goios.ConnectToService(device, afcService)
	if err != nil {
		return nil, classifyConnectError(err)
	}

	s := &afcSession{conn: conn, client: afc.NewFromConn(conn)}
	s.packetNum.Store(1 << 32)
	return s, nil
}

// session returns the cached connection for the device or opens a new one.
// The lock is held while connecting; with a single phone that is acceptable.
func (l *AFCPhotoLibrary) session(deviceID string) (*afcSession, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil, errors.New("photo library is closed")
	}
	if s, ok := l.sessions[deviceID]; ok {
		return s, nil
	}

	s, err := connect(deviceID)
	if err != nil {
		return nil, err
	}
	l.sessions[deviceID] = s
	return s, nil
}

// acquireTransferSession hands out an idle download connection or opens one.
// The caller owns it until releaseTransferSession.
func (l *AFCPhotoLibrary) acquireTransferSession(deviceID string) (*afcSession, error) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, errors.New("photo library is closed")
	}
	if list := l.transferIdle[deviceID]; len(list) > 0 {
		s := list[len(list)-1]
		l.transferIdle[deviceID] = list[:len(list)-1]
		l.mu.Unlock()
		return s, nil
	}
	l.mu.Unlock()

	// Connect outside the lock so parallel workers can connect at once.
	return connect(deviceID)
}

// releaseTransferSession returns a healthy connection to the idle pool, or
// closes it when it failed or the library was closed meanwhile.
func (l *AFCPhotoLibrary) releaseTransferSession(deviceID string, s *afcSession, healthy bool) {
	if healthy {
		l.mu.Lock()
		if !l.closed {
			l.transferIdle[deviceID] = append(l.transferIdle[deviceID], s)
			l.mu.Unlock()
			return
		}
		l.mu.Unlock()
	}
	s.close()
}

// drop forgets and closes a connection that failed.
func (l *AFCPhotoLibrary) drop(deviceID string, s *afcSession) {
	l.mu.Lock()
	if l.sessions[deviceID] == s {
		delete(l.sessions, deviceID)
	}
	l.mu.Unlock()
	s.close()
}

// run executes fn on the device's connection. A connection-level failure
// (for example the phone went away) drops the connection and retries once on
// a fresh one; errors answered by the device or caused by the request do not.
func (l *AFCPhotoLibrary) run(deviceID string, fn func(s *afcSession) error) error {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		s, err := l.session(deviceID)
		if err != nil {
			return err
		}

		s.mu.Lock()
		err = fn(s)
		s.mu.Unlock()

		if err == nil {
			return nil
		}
		var permanent permanentError
		if errors.As(err, &permanent) || isAFCStatusError(err) {
			return err
		}

		slog.Debug("afc operation failed, dropping connection", "device", deviceID, "error", err)
		l.drop(deviceID, s)
		lastErr = err
	}
	slog.Debug("afc operation failed after reconnect", "device", deviceID, "error", lastErr)
	return ErrConnectionLost
}

func validateDeviceID(deviceID string) error {
	if strings.TrimSpace(deviceID) == "" {
		return errors.New("deviceID must not be empty")
	}
	return nil
}

// FetchAlbums lists "Camera Roll" (all of /DCIM) and one album per roll
// folder such as 100APPLE.
func (l *AFCPhotoLibrary) FetchAlbums(deviceID string) ([]Album, error) {
	if err := validateDeviceID(deviceID); err != nil {
		return []Album{}, err
	}

	albums := []Album{}
	err := l.run(deviceID, func(s *afcSession) error {
		paths, err := s.collectMedia(dcimRoot, 0)
		if err != nil {
			return err
		}

		counts := make(map[string]int)
		for _, p := range paths {
			rel := strings.TrimPrefix(p, dcimRoot+"/")
			if roll, _, nested := strings.Cut(rel, "/"); nested {
				counts[roll]++
			}
		}

		rolls := make([]string, 0, len(counts))
		for roll := range counts {
			rolls = append(rolls, roll)
		}
		sort.Strings(rolls)

		built := make([]Album, 0, len(rolls)+1)
		built = append(built, Album{ID: cameraRollID, Name: "Camera Roll", Count: len(paths)})
		for _, roll := range rolls {
			built = append(built, Album{ID: rollIDPrefix + roll, Name: rollDisplayName(roll), Count: counts[roll]})
		}
		albums = built
		return nil
	})
	if err != nil {
		return []Album{}, wrapOperationError("list albums", err)
	}
	return albums, nil
}

// rollDisplayName turns "100APPLE" into "Roll 100".
func rollDisplayName(roll string) string {
	digits := strings.TrimRight(roll, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_")
	if digits == "" {
		return roll
	}
	return "Roll " + digits
}

// albumDir resolves an album ID to its AFC directory.
func albumDir(albumID string) (string, error) {
	if albumID == cameraRollID {
		return dcimRoot, nil
	}
	roll, ok := strings.CutPrefix(albumID, rollIDPrefix)
	if !ok || roll == "" || roll == "." || roll == ".." ||
		strings.ContainsAny(roll, `/\`+"\x00") || len(roll) > maxIDLength {
		return "", permanentf("unknown album %q", albumID)
	}
	return dcimRoot + "/" + roll, nil
}

// FetchPhotos lists the media files of an album, newest first. Dimensions
// are not available over AFC and are reported as 0 (unknown).
func (l *AFCPhotoLibrary) FetchPhotos(deviceID, albumID string) ([]Photo, error) {
	if err := validateDeviceID(deviceID); err != nil {
		return []Photo{}, err
	}
	if strings.TrimSpace(albumID) == "" {
		return []Photo{}, errors.New("albumID must not be empty")
	}
	dir, err := albumDir(albumID)
	if err != nil {
		return []Photo{}, err
	}

	photos := []Photo{}
	err = l.run(deviceID, func(s *afcSession) error {
		paths, err := s.collectMedia(dir, 0)
		if err != nil {
			return err
		}

		built := make([]Photo, 0, len(paths))
		for _, p := range paths {
			st, err := s.stat(p)
			if err != nil {
				if isAFCStatusError(err) {
					slog.Debug("skipping unreadable file", "path", p, "error", err)
					continue // for example deleted while listing
				}
				return err
			}
			if st.isDir {
				continue
			}
			built = append(built, buildPhoto(albumID, p, st))
		}

		sort.SliceStable(built, func(i, j int) bool {
			if built[i].Date != built[j].Date {
				return built[i].Date > built[j].Date // empty (unknown) dates last
			}
			return built[i].ID > built[j].ID
		})
		photos = built
		return nil
	})
	if err != nil {
		return []Photo{}, wrapOperationError("list photos", err)
	}
	return photos, nil
}

func buildPhoto(albumID, absPath string, st fileStat) Photo {
	name := path.Base(absPath)
	format, _ := formatOf(name)

	taken := st.mtime
	if taken.IsZero() {
		taken = dateFromFilename(name)
	}
	date := ""
	if !taken.IsZero() {
		date = taken.UTC().Format(time.RFC3339)
	}

	return Photo{
		ID:       strings.TrimPrefix(absPath, dcimRoot+"/"),
		Filename: name,
		Date:     date,
		Size:     st.size,
		Format:   format,
		Width:    0,
		Height:   0,
		AlbumID:  albumID,
		ThumbURL: "",
	}
}

// dateFromFilename extracts YYYYMMDD_HHMMSS from a name. iPhone files are
// normally called IMG_1234.HEIC, so this is only a fallback.
func dateFromFilename(name string) time.Time {
	m := filenameDate.FindStringSubmatch(name)
	if m == nil {
		return time.Time{}
	}
	t, err := time.Parse("20060102150405", m[1]+m[2]+m[3]+m[4]+m[5]+m[6])
	if err != nil {
		return time.Time{}
	}
	return t
}

// validPhotoID accepts only clean relative paths below /DCIM, so an ID coming
// from the frontend can never address anything else on the device.
func validPhotoID(id string) bool {
	if id == "" || len(id) > maxIDLength || strings.ContainsAny(id, `\`+"\x00") {
		return false
	}
	if strings.HasPrefix(id, "/") || path.Clean(id) != id {
		return false
	}
	for _, segment := range strings.Split(id, "/") {
		if segment == "" || segment == "." || segment == ".." || strings.HasPrefix(segment, ".") {
			return false
		}
	}
	return true
}

// FetchThumbnail returns the first thumbnailPrefixBytes bytes of a JPEG or PNG.
//
// Previews do not use this. deviceLibrary.Thumbnail downloads the full file
// and stores a resized JPEG through cachedThumbnail. This prefix reader remains
// for a short sample of a JPEG or PNG. HEIC, DNG and MOV are rejected here
// because a file prefix is not a decodable image.
func (l *AFCPhotoLibrary) FetchThumbnail(deviceID, photoID string) ([]byte, error) {
	if err := validateDeviceID(deviceID); err != nil {
		return nil, err
	}
	if !validPhotoID(photoID) {
		return nil, fmt.Errorf("invalid photoID %q", photoID)
	}
	format, ok := formatOf(photoID)
	if !ok {
		return nil, fmt.Errorf("unsupported file type for %q", photoID)
	}
	if format != "jpeg" && format != "png" {
		return nil, fmt.Errorf("preview not available for %s files yet", format)
	}

	var data []byte
	err := l.run(deviceID, func(s *afcSession) error {
		f, err := s.client.Open(dcimRoot+"/"+photoID, afc.READ_ONLY)
		if err != nil {
			return err
		}
		defer f.Close()

		read, err := io.ReadAll(io.LimitReader(f, thumbnailPrefixBytes))
		if err != nil {
			return err
		}
		if len(read) == 0 {
			return permanentf("file %q is empty", photoID)
		}
		data = read
		return nil
	})
	if err != nil {
		return nil, wrapOperationError("read thumbnail", err)
	}
	return data, nil
}

// DescribePhoto returns the metadata of one photo. The album ID is not known
// from a bare photo ID and is left empty.
func (l *AFCPhotoLibrary) DescribePhoto(deviceID, photoID string) (Photo, error) {
	if err := validateDeviceID(deviceID); err != nil {
		return Photo{}, err
	}
	if !validPhotoID(photoID) {
		return Photo{}, fmt.Errorf("invalid photoID %q", photoID)
	}
	if _, ok := formatOf(photoID); !ok {
		return Photo{}, fmt.Errorf("unsupported file type for %q", photoID)
	}

	var photo Photo
	err := l.run(deviceID, func(s *afcSession) error {
		absPath := dcimRoot + "/" + photoID
		st, err := s.stat(absPath)
		if err != nil {
			return err
		}
		if st.isDir {
			return permanentf("%q is not a file", photoID)
		}
		photo = buildPhoto("", absPath, st)
		return nil
	})
	if err != nil {
		if isAFCStatusError(err) {
			return Photo{}, fmt.Errorf("%s is not available on the device", path.Base(photoID))
		}
		return Photo{}, wrapOperationError("read photo details", err)
	}
	return photo, nil
}

// stream copies one remote file to dst in downloadChunkBytes pieces and
// returns the number of bytes written. A cancelled ctx is returned as is. A
// failing dst is reported as a permanent error: the connection is fine.
func (s *afcSession) stream(ctx context.Context, remote string, dst io.Writer, onBytes func(n int64)) (written int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.stat(remote)
	if err != nil {
		return 0, err
	}
	if st.isDir {
		return 0, permanentf("%q is a directory", remote)
	}

	src, err := s.client.Open(remote, afc.READ_ONLY)
	if err != nil {
		return 0, err
	}
	defer src.Close()

	buf := make([]byte, downloadChunkBytes)
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return written, permanentf("write local data: %w", writeErr)
			}
			written += int64(n)
			if onBytes != nil {
				onBytes(int64(n))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return written, readErr
		}
	}

	if written != st.size {
		return written, fmt.Errorf("incomplete download: received %d of %d bytes", written, st.size)
	}
	return written, nil
}

// download copies one remote file to destPath. The partial file is removed on
// error.
func (s *afcSession) download(ctx context.Context, remote, destPath string, onBytes func(n int64)) (int64, error) {
	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, permanentf("create %s: %w", destPath, err)
	}

	written, err := s.stream(ctx, remote, out, onBytes)
	if closeErr := out.Close(); err == nil && closeErr != nil {
		err = permanentf("write %s: %w", destPath, closeErr)
	}
	if err != nil {
		_ = os.Remove(destPath)
		return written, err
	}
	return written, nil
}

// DownloadPhoto streams a photo from the device to destPath. A connection
// failure retries once on a fresh connection; bytes already reported for the
// failed attempt are reported back as a negative onBytes value.
func (l *AFCPhotoLibrary) DownloadPhoto(ctx context.Context, deviceID, photoID, destPath string, onBytes func(n int64)) error {
	if err := validateDeviceID(deviceID); err != nil {
		return err
	}
	if !validPhotoID(photoID) {
		return fmt.Errorf("invalid photoID %q", photoID)
	}
	if strings.TrimSpace(destPath) == "" {
		return errors.New("destPath must not be empty")
	}
	remote := dcimRoot + "/" + photoID

	return l.withTransferSession(ctx, deviceID, photoID, "download photo", onBytes,
		func(s *afcSession) (int64, error) {
			return s.download(ctx, remote, destPath, onBytes)
		})
}

// HashPhoto returns the hex SHA-256 of a photo, streamed from the device
// without storing it. It uses the same dedicated connections as downloads.
func (l *AFCPhotoLibrary) HashPhoto(ctx context.Context, deviceID, photoID string, onBytes func(n int64)) (string, error) {
	if err := validateDeviceID(deviceID); err != nil {
		return "", err
	}
	if !validPhotoID(photoID) {
		return "", fmt.Errorf("invalid photoID %q", photoID)
	}
	remote := dcimRoot + "/" + photoID

	var sum string
	err := l.withTransferSession(ctx, deviceID, photoID, "read photo", onBytes,
		func(s *afcSession) (int64, error) {
			h := sha256.New() // a fresh hash for every attempt
			n, err := s.stream(ctx, remote, h, onBytes)
			if err == nil {
				sum = hex.EncodeToString(h.Sum(nil))
			}
			return n, err
		})
	if err != nil {
		return "", err
	}
	return sum, nil
}

// withTransferSession runs op on a dedicated connection. A connection failure
// retries once on a fresh connection; bytes the failed attempt reported are
// taken back with a negative onBytes value.
func (l *AFCPhotoLibrary) withTransferSession(
	ctx context.Context,
	deviceID, photoID, action string,
	onBytes func(n int64),
	op func(s *afcSession) (written int64, err error),
) error {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var s *afcSession
		var err error
		if attempt == 0 {
			s, err = l.acquireTransferSession(deviceID)
		} else {
			s, err = connect(deviceID) // never reuse a pooled connection after a failure
		}
		if err != nil {
			return wrapOperationError(action, err)
		}

		written, err := op(s)
		if err == nil {
			l.releaseTransferSession(deviceID, s, true)
			return nil
		}
		if written > 0 && onBytes != nil {
			onBytes(-written)
		}

		// An answer from the device (or a local file problem) means the
		// connection itself is fine and a retry would fail the same way.
		var permanent permanentError
		deviceAnswered := errors.As(err, &permanent) || isAFCStatusError(err)
		l.releaseTransferSession(deviceID, s, deviceAnswered)

		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if deviceAnswered {
			if isAFCStatusError(err) {
				return fmt.Errorf("%s is not available on the device", path.Base(photoID))
			}
			return wrapOperationError(action, err)
		}
		slog.Debug("afc transfer failed, retrying on a fresh connection", "action", action, "device", deviceID, "photo", photoID, "error", err)
		lastErr = err
	}
	slog.Debug("afc transfer failed after reconnect", "action", action, "device", deviceID, "photo", photoID, "error", lastErr)
	return ErrConnectionLost
}

// wrapOperationError keeps user-facing sentinel errors readable and adds
// context to everything else.
func wrapOperationError(action string, err error) error {
	switch {
	case errors.Is(err, ErrTunnelNotRunning), errors.Is(err, ErrAFCUnavailable),
		errors.Is(err, ErrDeviceNotFound), errors.Is(err, ErrNotPaired),
		errors.Is(err, ErrUsbmuxdMissing), errors.Is(err, ErrConnectionLost):
		return err
	}
	slog.Debug("afc operation error", "action", action, "error", err)
	return fmt.Errorf("could not %s: %w", action, err)
}
