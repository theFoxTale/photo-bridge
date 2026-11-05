package backend

import "context"

// Photo describes a single media item on a device.
type Photo struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Date     string `json:"date"`   // ISO 8601
	Size     int64  `json:"size"`   // bytes
	Format   string `json:"format"` // "heic" | "jpeg" | "dng" | "png" | "mov" | "mp4"
	IsVideo  bool   `json:"isVideo"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	AlbumID  string `json:"albumId"`
	// ThumbURL is a base64 JPEG data URL of a preview. FetchPhotos leaves it
	// empty on purpose; thumbnails are fetched lazily through FetchThumbnail.
	ThumbURL string `json:"thumbUrl"`
	// IsLivePhoto is true when a .mov file in the same folder shares this
	// still's base name. The .mov is not listed on its own.
	IsLivePhoto bool `json:"isLivePhoto"`
	// LivePhotoVideoID is the device id of that .mov. Empty when IsLivePhoto
	// is false.
	LivePhotoVideoID string `json:"livePhotoVideoId"`
}

// isVideoFormat reports whether format is a video the app can list and copy.
func isVideoFormat(format string) bool {
	return format == "mov" || format == "mp4"
}

// Album groups photos on a device.
type Album struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Library is the source of albums, photos and thumbnails for a device.
// NewLibrary returns the implementation selected by build tags: sample data
// with `-tags mock`, otherwise a library that reports it is unavailable.
type Library interface {
	Albums(ctx context.Context, deviceID string) ([]Album, error)
	Photos(ctx context.Context, deviceID, albumID string) ([]Photo, error)
	Thumbnail(ctx context.Context, deviceID, photoID string) (string, error)

	// Describe returns the metadata of one photo by ID.
	Describe(ctx context.Context, deviceID, photoID string) (Photo, error)

	// Download writes the original file to destPath. onBytes (optional)
	// receives the size of each written chunk; a negative value takes back
	// bytes of an attempt that was restarted.
	Download(ctx context.Context, deviceID, photoID, destPath string, onBytes func(n int64)) error

	// Hash returns the hex SHA-256 of the original file, streamed without
	// storing it.
	Hash(ctx context.Context, deviceID, photoID string, onBytes func(n int64)) (string, error)
}
