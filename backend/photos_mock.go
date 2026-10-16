//go:build mock

package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"time"
)

const thumbnailLongSide = 200

type mockAlbumSpec struct {
	id    string
	name  string
	count int
}

var mockAlbumSpecs = []mockAlbumSpec{
	{id: "camera-roll", name: "Camera Roll", count: 20},
	{id: "favorites", name: "Favorites", count: 12},
	{id: "screenshots", name: "Screenshots", count: 8},
}

// palette holds start/end colours for generated thumbnails.
var palette = [][2]color.RGBA{
	{{R: 56, G: 189, B: 248, A: 255}, {R: 30, G: 64, B: 175, A: 255}},
	{{R: 251, G: 146, B: 60, A: 255}, {R: 190, G: 18, B: 60, A: 255}},
	{{R: 74, G: 222, B: 128, A: 255}, {R: 6, G: 95, B: 70, A: 255}},
	{{R: 192, G: 132, B: 252, A: 255}, {R: 91, G: 33, B: 182, A: 255}},
	{{R: 250, G: 204, B: 21, A: 255}, {R: 161, G: 98, B: 7, A: 255}},
}

type mockLibrary struct {
	albums []Album
	photos []Photo
}

// NewLibrary returns the sample-data library used by `-tags mock` builds.
func NewLibrary() Library {
	return newMockLibrary()
}

func newMockLibrary() *mockLibrary {
	lib := &mockLibrary{}
	index := 0
	for _, spec := range mockAlbumSpecs {
		lib.albums = append(lib.albums, Album{ID: spec.id, Name: spec.name, Count: spec.count})
		for n := 0; n < spec.count; n++ {
			lib.photos = append(lib.photos, mockPhoto(index, n, spec.id))
			index++
		}
	}
	return lib
}

func mockPhoto(index, indexInAlbum int, albumID string) Photo {
	var (
		format        string
		ext           string
		width, height int
		size          int64
	)

	switch albumID {
	case "screenshots":
		format, ext = "png", "PNG"
		width, height = 1170, 2532
		size = 900_000 + int64(index%7)*140_000
	case "favorites":
		if indexInAlbum%2 == 0 {
			format, ext = "heic", "HEIC"
			size = 1_800_000 + int64(index%9)*130_000
		} else {
			format, ext = "jpeg", "JPG"
			size = 3_100_000 + int64(index%9)*210_000
		}
		width, height = 4032, 3024
	default:
		switch indexInAlbum % 4 {
		case 0, 1:
			format, ext = "heic", "HEIC"
			size = 1_900_000 + int64(index%11)*120_000
		case 2:
			format, ext = "jpeg", "JPG"
			size = 3_300_000 + int64(index%11)*180_000
		default:
			format, ext = "dng", "DNG"
			size = 24_500_000 + int64(index%11)*900_000
		}
		width, height = 4032, 3024
	}

	// Every third non-screenshot photo is a portrait.
	if albumID != "screenshots" && index%3 == 0 {
		width, height = height, width
	}

	filename := fmt.Sprintf("IMG_%04d.%s", 1001+index, ext)
	if albumID == "screenshots" {
		filename = fmt.Sprintf("Screenshot_%04d.%s", 1+indexInAlbum, ext)
	}

	taken := time.Date(2026, time.October, 1, 9, 0, 0, 0, time.UTC).
		Add(-time.Duration(index) * 26 * time.Hour)

	return Photo{
		ID:       fmt.Sprintf("photo-%03d", index+1),
		Filename: filename,
		Date:     taken.Format(time.RFC3339),
		Size:     size,
		Format:   format,
		Width:    width,
		Height:   height,
		AlbumID:  albumID,
		ThumbURL: "",
	}
}

func requireNonEmpty(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	return nil
}

func (l *mockLibrary) Albums(ctx context.Context, deviceID string) ([]Album, error) {
	if err := ctx.Err(); err != nil {
		return []Album{}, err
	}
	if err := requireNonEmpty("deviceID", deviceID); err != nil {
		return []Album{}, err
	}
	albums := make([]Album, len(l.albums))
	copy(albums, l.albums)
	return albums, nil
}

func (l *mockLibrary) Photos(ctx context.Context, deviceID, albumID string) ([]Photo, error) {
	if err := ctx.Err(); err != nil {
		return []Photo{}, err
	}
	if err := requireNonEmpty("deviceID", deviceID); err != nil {
		return []Photo{}, err
	}
	if err := requireNonEmpty("albumID", albumID); err != nil {
		return []Photo{}, err
	}

	known := false
	for _, album := range l.albums {
		if album.ID == albumID {
			known = true
			break
		}
	}
	if !known {
		return []Photo{}, fmt.Errorf("album %q not found", albumID)
	}

	photos := make([]Photo, 0, 20)
	for _, photo := range l.photos {
		if photo.AlbumID == albumID {
			photos = append(photos, photo)
		}
	}
	return photos, nil
}

func (l *mockLibrary) Thumbnail(ctx context.Context, deviceID, photoID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := requireNonEmpty("deviceID", deviceID); err != nil {
		return "", err
	}
	if err := requireNonEmpty("photoID", photoID); err != nil {
		return "", err
	}

	for i, photo := range l.photos {
		if photo.ID != photoID {
			continue
		}
		return renderThumbnail(i, photo.Width, photo.Height)
	}
	return "", fmt.Errorf("photo %q not found", photoID)
}

// renderThumbnail draws a diagonal gradient JPEG whose longer side is
// thumbnailLongSide pixels and returns it as a base64 data URL.
func renderThumbnail(index, srcWidth, srcHeight int) (string, error) {
	if srcWidth <= 0 || srcHeight <= 0 {
		return "", fmt.Errorf("invalid source dimensions %dx%d", srcWidth, srcHeight)
	}

	w, h := thumbnailLongSide, thumbnailLongSide
	if srcWidth >= srcHeight {
		h = thumbnailLongSide * srcHeight / srcWidth
	} else {
		w = thumbnailLongSide * srcWidth / srcHeight
	}

	colors := palette[index%len(palette)]
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	span := w + h - 2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := float64(x+y) / float64(span)
			img.SetRGBA(x, y, color.RGBA{
				R: lerp(colors[0].R, colors[1].R, t),
				G: lerp(colors[0].G, colors[1].G, t),
				B: lerp(colors[0].B, colors[1].B, t),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 70}); err != nil {
		return "", fmt.Errorf("encode thumbnail: %w", err)
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func lerp(from, to uint8, t float64) uint8 {
	return uint8(float64(from) + (float64(to)-float64(from))*t)
}
