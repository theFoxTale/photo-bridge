//go:build cgo

package backend

import (
	"errors"
	"fmt"
	"os"

	"github.com/adrium/goheif"
	"github.com/adrium/goheif/heif"
)

// decodeHEIC decodes the primary image of a HEIC file together with its EXIF
// block and rotation. goheif needs cgo (libde265), so this file is only part
// of builds made with CGO_ENABLED=1 and a C compiler.
func decodeHEIC(f *os.File) (heicImage, error) {
	var result heicImage

	item, err := heif.Open(f).PrimaryItem()
	if err != nil {
		return heicImage{}, fmt.Errorf("read HEIC structure: %w", err)
	}
	result.rotations = item.Rotations()

	rawExif, err := goheif.ExtractExif(f)
	switch {
	case err == nil:
		result.exif = rawExif
	case errors.Is(err, heif.ErrNoEXIF):
		// A photo without EXIF is converted without it.
	default:
		return heicImage{}, fmt.Errorf("read HEIC EXIF: %w", err)
	}

	img, err := goheif.Decode(f)
	if err != nil {
		return heicImage{}, fmt.Errorf("decode HEIC: %w", err)
	}
	result.img = img
	return result, nil
}
