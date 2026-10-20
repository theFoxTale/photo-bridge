//go:build !mock

package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
)

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
	data, err := d.src.FetchThumbnail(deviceID, photoID)
	if err != nil {
		return "", err
	}

	mime := http.DetectContentType(data)
	if mime != "image/jpeg" && mime != "image/png" {
		return "", fmt.Errorf("preview not available for content type %s", mime)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
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

// Close closes the cached device connections.
func (d *deviceLibrary) Close() error {
	return d.src.Close()
}
