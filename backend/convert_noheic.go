//go:build !cgo

package backend

import (
	"errors"
	"os"
)

// errHEICUnavailable is reported for every HEIC to JPG conversion in a build
// made without cgo.
var errHEICUnavailable = errors.New("HEIC to JPG conversion is not available in this build: rebuild with CGO_ENABLED=1 and a C compiler (MinGW-w64)")

// decodeHEIC cannot decode without the cgo based decoder, see convert_heic.go.
func decodeHEIC(*os.File) (heicImage, error) {
	return heicImage{}, errHEICUnavailable
}
