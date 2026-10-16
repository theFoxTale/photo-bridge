//go:build !mock

package backend

import (
	"context"
	"errors"
)

// ErrLibraryUnavailable is returned by every library call in builds without
// the `mock` tag, because reading a real device's photo library is not
// implemented yet.
var ErrLibraryUnavailable = errors.New(
	"reading photos from a real device is not implemented yet; build with `-tags mock` to use sample data",
)

type unavailableLibrary struct{}

// NewLibrary returns the library used by this build.
func NewLibrary() Library {
	return unavailableLibrary{}
}

func (unavailableLibrary) Albums(_ context.Context, _ string) ([]Album, error) {
	return []Album{}, ErrLibraryUnavailable
}

func (unavailableLibrary) Photos(_ context.Context, _, _ string) ([]Photo, error) {
	return []Photo{}, ErrLibraryUnavailable
}

func (unavailableLibrary) Thumbnail(_ context.Context, _, _ string) (string, error) {
	return "", ErrLibraryUnavailable
}
