//go:build !mock

package backend

import "context"

// LibraryMode reports where photo data comes from in this build.
func LibraryMode() string {
	return "device"
}

// deviceLibrary adapts a PhotoLibrary (no context, raw thumbnail bytes) to the
// Library interface used by the App.
type deviceLibrary struct {
	src *AFCPhotoLibrary
}

// NewLibrary returns the library used by this build: real devices over AFC.
func NewLibrary() Library {
	return &deviceLibrary{src: NewAFCPhotoLibrary()}
}

func (d *deviceLibrary) Albums(ctx context.Context, deviceID string) ([]Album, error) {
	if err := ctx.Err(); err != nil {
		return []Album{}, err
	}
	return d.src.FetchAlbums(deviceID)
}

func (d *deviceLibrary) Photos(ctx context.Context, deviceID, albumID string) ([]Photo, error) {
	if err := ctx.Err(); err != nil {
		return []Photo{}, err
	}
	return d.src.FetchPhotos(deviceID, albumID)
}

func (d *deviceLibrary) Thumbnail(ctx context.Context, deviceID, photoID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return cachedThumbnail(ctx, deviceID, photoID, func(ctx context.Context, destPath string) error {
		return d.Download(ctx, deviceID, photoID, destPath, nil)
	})
}

func (d *deviceLibrary) Describe(ctx context.Context, deviceID, photoID string) (Photo, error) {
	if err := ctx.Err(); err != nil {
		return Photo{}, err
	}
	return d.src.DescribePhoto(deviceID, photoID)
}

func (d *deviceLibrary) Download(ctx context.Context, deviceID, photoID, destPath string, onBytes func(n int64)) error {
	return d.src.DownloadPhoto(ctx, deviceID, photoID, destPath, onBytes)
}

func (d *deviceLibrary) Hash(ctx context.Context, deviceID, photoID string, onBytes func(n int64)) (string, error) {
	return d.src.HashPhoto(ctx, deviceID, photoID, onBytes)
}

// Close closes the cached device connections.
func (d *deviceLibrary) Close() error {
	return d.src.Close()
}
