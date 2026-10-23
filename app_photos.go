package main

import (
	"fmt"
	"strings"

	"photo-bridge/backend"
)

func requireID(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	return nil
}

// LibraryMode reports where photo data comes from: "device" for a real
// iPhone, "mock" in builds made with `-tags mock`.
func (a *App) LibraryMode() string {
	return backend.LibraryMode()
}

// FetchAlbums returns the albums of the given device. The result is an empty
// slice (never nil) when the device has none.
func (a *App) FetchAlbums(deviceID string) ([]backend.Album, error) {
	if err := requireID("deviceID", deviceID); err != nil {
		return []backend.Album{}, err
	}
	albums, err := a.library.Albums(a.runtimeContext(), deviceID)
	if err != nil {
		return []backend.Album{}, fmt.Errorf("fetch albums: %w", err)
	}
	if albums == nil {
		albums = []backend.Album{}
	}
	return albums, nil
}

// FetchPhotos returns the photos of one album. Thumbnails are not included;
// request them with FetchThumbnail.
func (a *App) FetchPhotos(deviceID, albumID string) ([]backend.Photo, error) {
	if err := requireID("deviceID", deviceID); err != nil {
		return []backend.Photo{}, err
	}
	if err := requireID("albumID", albumID); err != nil {
		return []backend.Photo{}, err
	}
	photos, err := a.library.Photos(a.runtimeContext(), deviceID, albumID)
	if err != nil {
		return []backend.Photo{}, fmt.Errorf("fetch photos: %w", err)
	}
	if photos == nil {
		photos = []backend.Photo{}
	}
	return photos, nil
}

// FetchThumbnail returns a base64 JPEG data URL for a photo preview.
// Device builds resize the original to at most 400 pixels on the long side
// and cache the result on disk.
func (a *App) FetchThumbnail(deviceID, photoID string) (string, error) {
	if err := requireID("deviceID", deviceID); err != nil {
		return "", err
	}
	if err := requireID("photoID", photoID); err != nil {
		return "", err
	}
	thumb, err := a.library.Thumbnail(a.runtimeContext(), deviceID, photoID)
	if err != nil {
		return "", fmt.Errorf("fetch thumbnail: %w", err)
	}
	return thumb, nil
}
