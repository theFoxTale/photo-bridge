package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	exif "github.com/dsoprea/go-exif/v3"
)

const (
	exifTagOrientation  = 0x0112
	exifTypeShort       = 3
	exifIFDEntrySize    = 12
	jpegMaxSegmentBytes = 65535 // a segment length field counts itself (2 bytes)
)

// exifAPP1Prefix introduces EXIF data inside a JPEG APP1 segment.
var exifAPP1Prefix = []byte("Exif\x00\x00")

// normalizeExif returns the raw TIFF block of EXIF data that may or may not
// start with the "Exif\0\0" prefix, after checking that its header parses.
// An empty input yields (nil, nil): the photo simply has no EXIF.
func normalizeExif(raw []byte) ([]byte, error) {
	raw = bytes.TrimPrefix(raw, exifAPP1Prefix)
	if len(raw) == 0 {
		return nil, nil
	}
	if _, err := exif.ParseExifHeader(raw); err != nil {
		return nil, fmt.Errorf("EXIF data is corrupt: %w", err)
	}
	return raw, nil
}

// jpegExif walks the segments of a JPEG and returns the TIFF block of its
// EXIF APP1 segment, or nil when it has none. Only the segments before the
// image data are inspected.
func jpegExif(data []byte) ([]byte, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, errors.New("not a JPEG file")
	}
	pos := 2
	for pos+4 <= len(data) {
		if data[pos] != 0xFF {
			return nil, nil // lost sync; treat as no EXIF
		}
		marker := data[pos+1]
		switch {
		case marker == 0xFF:
			pos++ // fill byte
			continue
		case marker == 0xDA || marker == 0xD9:
			return nil, nil // start of scan or end of image: no more metadata
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD8):
			pos += 2 // markers without a length
			continue
		}

		length := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		if length < 2 || pos+2+length > len(data) {
			return nil, errors.New("truncated JPEG segment")
		}
		payload := data[pos+4 : pos+2+length]
		if marker == 0xE1 && bytes.HasPrefix(payload, exifAPP1Prefix) {
			return normalizeExif(payload)
		}
		pos += 2 + length
	}
	return nil, nil
}

// setExifOrientation overwrites the Orientation tag of IFD0 in place. A block
// without that tag is left untouched.
func setExifOrientation(raw []byte, value uint16) error {
	header, err := exif.ParseExifHeader(raw)
	if err != nil {
		return fmt.Errorf("EXIF data is corrupt: %w", err)
	}
	order := header.ByteOrder

	ifd := int(header.FirstIfdOffset)
	if ifd < 0 || ifd+2 > len(raw) {
		return errors.New("EXIF IFD offset is out of range")
	}
	count := int(order.Uint16(raw[ifd:]))
	for i := 0; i < count; i++ {
		entry := ifd + 2 + i*exifIFDEntrySize
		if entry+exifIFDEntrySize > len(raw) {
			return errors.New("EXIF IFD is truncated")
		}
		if order.Uint16(raw[entry:]) != exifTagOrientation {
			continue
		}
		if order.Uint16(raw[entry+2:]) != exifTypeShort {
			return errors.New("EXIF orientation has an unexpected type")
		}
		order.PutUint16(raw[entry+8:], value)
		return nil
	}
	return nil
}

// insertExif places an EXIF APP1 segment right after the SOI marker of an
// encoded JPEG. Go's encoder writes no APP segments, so that position is valid.
func insertExif(encoded, rawExif []byte) ([]byte, error) {
	if len(encoded) < 2 || encoded[0] != 0xFF || encoded[1] != 0xD8 {
		return nil, errors.New("encoder produced an invalid JPEG")
	}
	segmentLen := 2 + len(exifAPP1Prefix) + len(rawExif)
	if segmentLen > jpegMaxSegmentBytes {
		return nil, fmt.Errorf("EXIF data is too large for a JPEG (%d bytes)", len(rawExif))
	}

	out := make([]byte, 0, len(encoded)+segmentLen+2)
	out = append(out, encoded[:2]...)
	out = append(out, 0xFF, 0xE1, byte(segmentLen>>8), byte(segmentLen))
	out = append(out, exifAPP1Prefix...)
	out = append(out, rawExif...)
	out = append(out, encoded[2:]...)
	return out, nil
}
