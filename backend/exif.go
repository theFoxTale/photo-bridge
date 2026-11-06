package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	exif "github.com/dsoprea/go-exif/v3"
)

const (
	exifTagOrientation       = 0x0112
	exifTagDateTime          = 0x0132
	exifTagExifIFD           = 0x8769
	exifTagDateTimeOriginal  = 0x9003
	exifTagDateTimeDigitized = 0x9004
	exifTypeASCII            = 2
	exifTypeShort            = 3
	exifTypeLong             = 4
	exifIFDEntrySize         = 12
	jpegMaxSegmentBytes      = 65535 // a segment length field counts itself (2 bytes)
)

// exifTimestamp returns the capture time stored in a TIFF EXIF block.
// DateTimeOriginal is preferred, then DateTimeDigitized, then DateTime.
func exifTimestamp(raw []byte) (time.Time, bool) {
	header, err := exif.ParseExifHeader(raw)
	if err != nil {
		return time.Time{}, false
	}
	order := header.ByteOrder
	ifd := int(header.FirstIfdOffset)
	if off, ok := ifdLong(raw, order, ifd, exifTagExifIFD); ok {
		if t, ok := ifdASCIITime(raw, order, off, exifTagDateTimeOriginal); ok {
			return t, true
		}
		if t, ok := ifdASCIITime(raw, order, off, exifTagDateTimeDigitized); ok {
			return t, true
		}
	}
	if t, ok := ifdASCIITime(raw, order, ifd, exifTagDateTime); ok {
		return t, true
	}
	return time.Time{}, false
}

func ifdLong(raw []byte, order binary.ByteOrder, ifd int, tag uint16) (int, bool) {
	var value int
	found := false
	if !eachIFDEntry(raw, order, ifd, func(entry int) bool {
		if order.Uint16(raw[entry:]) != tag {
			return true
		}
		if order.Uint16(raw[entry+2:]) != exifTypeLong || order.Uint32(raw[entry+4:]) != 1 {
			return false
		}
		value = int(order.Uint32(raw[entry+8:]))
		found = true
		return false
	}) {
		return 0, false
	}
	return value, found
}

func ifdASCIITime(raw []byte, order binary.ByteOrder, ifd int, tag uint16) (time.Time, bool) {
	var parsed time.Time
	found := false
	if !eachIFDEntry(raw, order, ifd, func(entry int) bool {
		if order.Uint16(raw[entry:]) != tag {
			return true
		}
		text, ok := entryASCII(raw, order, entry)
		if !ok {
			return false
		}
		t, err := time.Parse("2006:01:02 15:04:05", text)
		if err != nil {
			return false
		}
		parsed = t.UTC()
		found = true
		return false
	}) {
		return time.Time{}, false
	}
	return parsed, found
}

func eachIFDEntry(raw []byte, order binary.ByteOrder, ifd int, visit func(entry int) bool) bool {
	if ifd < 0 || ifd+2 > len(raw) {
		return false
	}
	count := int(order.Uint16(raw[ifd:]))
	base := ifd + 2
	if base > len(raw) || count > (len(raw)-base)/exifIFDEntrySize {
		return false
	}
	for i := 0; i < count; i++ {
		if !visit(base + i*exifIFDEntrySize) {
			return true
		}
	}
	return true
}

func entryASCII(raw []byte, order binary.ByteOrder, entry int) (string, bool) {
	if order.Uint16(raw[entry+2:]) != exifTypeASCII {
		return "", false
	}
	count := int(order.Uint32(raw[entry+4:]))
	if count <= 0 || count > 32 {
		return "", false
	}
	var buf []byte
	if count <= 4 {
		buf = raw[entry+8 : entry+8+count]
	} else {
		off := int(order.Uint32(raw[entry+8:]))
		if off < 0 || off+count > len(raw) {
			return "", false
		}
		buf = raw[off : off+count]
	}
	return string(bytes.TrimRight(buf, "\x00")), true
}

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

const (
	tiffTagImageWidth  = 0x0100
	tiffTagImageLength = 0x0101
	tiffTagSubIFD      = 0x014A
)

// exifOrientation reads the Orientation short from IFD0. The bool is false
// when the block has no usable tag. Callers treat that as orientation 1.
func exifOrientation(raw []byte) (int, bool) {
	raw = bytes.TrimPrefix(raw, exifAPP1Prefix)
	if len(raw) == 0 {
		return 1, false
	}
	header, err := exif.ParseExifHeader(raw)
	if err != nil {
		return 1, false
	}
	value, ok := ifdShort(raw, header.ByteOrder, int(header.FirstIfdOffset), exifTagOrientation)
	if !ok || value < 1 || value > 8 {
		return 1, false
	}
	return int(value), true
}

// exifBlockOrientation is orientation 1 when the block is missing or corrupt.
// A bad tag must not fail a photo that otherwise decodes.
func exifBlockOrientation(raw []byte) int {
	normalized, err := normalizeExif(raw)
	if err != nil || len(normalized) == 0 {
		return 1
	}
	if o, ok := exifOrientation(normalized); ok {
		return o
	}
	return 1
}

// jpegOrientation reads a JPEG's EXIF orientation, or 1 when it has none.
func jpegOrientation(data []byte) int {
	raw, err := jpegExif(data)
	if err != nil || len(raw) == 0 {
		return 1
	}
	if o, ok := exifOrientation(raw); ok {
		return o
	}
	return 1
}

// visualSize is the size a viewer shows. Orientations 5 through 8 turn the
// stored image on its side, so the stored width and height trade places.
func visualSize(width, height, orientation int) (int, int) {
	if width <= 0 || height <= 0 {
		return 0, 0
	}
	switch orientation {
	case 5, 6, 7, 8:
		return height, width
	default:
		return width, height
	}
}

// imageHeaderSize reads the stored pixel size and EXIF orientation from the
// leading bytes of a still. ok is false when those bytes do not contain a size.
func imageHeaderSize(format string, data []byte) (width, height, orientation int, ok bool) {
	switch format {
	case "jpeg":
		return jpegHeaderSize(data)
	case "png":
		return pngHeaderSize(data)
	case "dng":
		return tiffHeaderSize(data)
	case "heic":
		return heicHeaderSize(data)
	default:
		return 0, 0, 1, false
	}
}

func ifdShort(raw []byte, order binary.ByteOrder, ifd int, tag uint16) (uint16, bool) {
	var value uint16
	found := false
	if !eachIFDEntry(raw, order, ifd, func(entry int) bool {
		if order.Uint16(raw[entry:]) != tag {
			return true
		}
		if order.Uint16(raw[entry+2:]) != exifTypeShort || order.Uint32(raw[entry+4:]) != 1 {
			return false
		}
		value = order.Uint16(raw[entry+8:])
		found = true
		return false
	}) {
		return 0, false
	}
	return value, found
}

// ifdInline reads one SHORT or LONG stored in the entry itself.
func ifdInline(raw []byte, order binary.ByteOrder, ifd int, tag uint16) (int, bool) {
	var value int
	found := false
	if !eachIFDEntry(raw, order, ifd, func(entry int) bool {
		if order.Uint16(raw[entry:]) != tag {
			return true
		}
		if order.Uint32(raw[entry+4:]) != 1 {
			return false
		}
		switch order.Uint16(raw[entry+2:]) {
		case exifTypeShort:
			value = int(order.Uint16(raw[entry+8:]))
		case exifTypeLong:
			value = int(order.Uint32(raw[entry+8:]))
		default:
			return false
		}
		found = true
		return false
	}) {
		return 0, false
	}
	return value, found
}

func jpegHeaderSize(data []byte) (width, height, orientation int, ok bool) {
	orientation = 1
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0, 0, 1, false
	}
	pos := 2
	for pos+4 <= len(data) {
		if data[pos] != 0xFF {
			return 0, 0, orientation, false
		}
		marker := data[pos+1]
		switch {
		case marker == 0xFF:
			pos++
			continue
		case marker == 0xDA || marker == 0xD9:
			return 0, 0, orientation, false
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD8):
			pos += 2
			continue
		}

		length := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		if length < 2 || pos+2+length > len(data) {
			return 0, 0, orientation, false
		}
		payload := data[pos+4 : pos+2+length]
		if marker == 0xE1 && bytes.HasPrefix(payload, exifAPP1Prefix) {
			if o, found := exifOrientation(payload); found {
				orientation = o
			}
		}
		if isSOF(marker) && len(payload) >= 5 {
			height = int(binary.BigEndian.Uint16(payload[1:3]))
			width = int(binary.BigEndian.Uint16(payload[3:5]))
			return width, height, orientation, width > 0 && height > 0
		}
		pos += 2 + length
	}
	return 0, 0, orientation, false
}

func isSOF(marker byte) bool {
	switch marker {
	case 0xC0, 0xC1, 0xC2, 0xC3,
		0xC5, 0xC6, 0xC7,
		0xC9, 0xCA, 0xCB,
		0xCD, 0xCE, 0xCF:
		return true
	default:
		return false
	}
}

var pngSignature = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

func pngHeaderSize(data []byte) (width, height, orientation int, ok bool) {
	orientation = 1
	if !bytes.HasPrefix(data, pngSignature) {
		return 0, 0, 1, false
	}
	pos := len(pngSignature)
	for pos+8 <= len(data) {
		length64 := uint64(binary.BigEndian.Uint32(data[pos:]))
		typ := string(data[pos+4 : pos+8])
		pos += 8
		if length64 > uint64(len(data)-pos) || length64+4 > uint64(len(data)-pos) {
			return width, height, orientation, ok
		}
		length := int(length64)
		payload := data[pos : pos+length]
		switch typ {
		case "IHDR":
			if len(payload) < 8 {
				return 0, 0, 1, false
			}
			width = int(binary.BigEndian.Uint32(payload[0:4]))
			height = int(binary.BigEndian.Uint32(payload[4:8]))
			ok = width > 0 && height > 0
		case "eXIf":
			if o, found := exifOrientation(payload); found {
				orientation = o
			}
		case "IEND":
			return width, height, orientation, ok
		}
		pos += length + 4
	}
	return width, height, orientation, ok
}

// pngOrientation reads a PNG eXIf orientation, or 1 when the file has none.
func pngOrientation(data []byte) int {
	_, _, orientation, ok := pngHeaderSize(data)
	if !ok || orientation < 1 || orientation > 8 {
		return 1
	}
	return orientation
}

func tiffHeaderSize(data []byte) (width, height, orientation int, ok bool) {
	header, err := exif.ParseExifHeader(data)
	if err != nil {
		return 0, 0, 1, false
	}
	order := header.ByteOrder
	ifd := int(header.FirstIfdOffset)
	width, height, orientation, ok = tiffIFDSize(data, order, ifd)
	if ok {
		return width, height, orientation, true
	}
	sub, found := ifdInline(data, order, ifd, tiffTagSubIFD)
	if !found {
		return 0, 0, 1, false
	}
	return tiffIFDSize(data, order, sub)
}

func tiffIFDSize(data []byte, order binary.ByteOrder, ifd int) (width, height, orientation int, ok bool) {
	width, _ = ifdInline(data, order, ifd, tiffTagImageWidth)
	height, _ = ifdInline(data, order, ifd, tiffTagImageLength)
	orientation = 1
	if o, found := ifdShort(data, order, ifd, exifTagOrientation); found && o >= 1 && o <= 8 {
		orientation = int(o)
	}
	return width, height, orientation, width > 0 && height > 0
}

// heicHeaderSize reads the primary item's stored size. The container irot
// value wins over EXIF when it is not zero, matching the pixel decoder,
// which otherwise would turn the photo twice.
func heicHeaderSize(data []byte) (width, height, orientation int, ok bool) {
	if len(data) < 12 || string(data[4:8]) != "ftyp" {
		return 0, 0, 1, false
	}
	meta, found := findBox(data, "meta")
	if !found || len(meta) < 4 {
		return 0, 0, 1, false
	}
	var (
		primary     uint32
		havePrimary bool
		props       []heifProp
		ipma        []byte
	)
	walkBoxes(meta[4:], func(typ string, payload []byte) {
		switch typ {
		case "pitm":
			primary, havePrimary = parsePitm(payload)
		case "iprp":
			props, ipma = parseIprp(payload)
		}
	})
	width, height, rot, ok := selectHeifSize(props, ipma, primary, havePrimary)
	if !ok {
		return 0, 0, 1, false
	}
	orientation = 1
	if rot != 0 {
		orientation = orientationFromIrot(rot)
	} else if o, found := heicExifOrientation(data); found {
		orientation = o
	}
	return width, height, orientation, true
}

func orientationFromIrot(rot int) int {
	switch rot & 3 {
	case 1:
		return 8
	case 2:
		return 3
	case 3:
		return 6
	default:
		return 1
	}
}

func heicExifOrientation(data []byte) (int, bool) {
	idx := bytes.Index(data, exifAPP1Prefix)
	if idx < 0 || idx+len(exifAPP1Prefix) >= len(data) {
		return 1, false
	}
	raw := data[idx+len(exifAPP1Prefix):]
	if len(raw) > 64*1024 {
		raw = raw[:64*1024]
	}
	return exifOrientation(raw)
}

type heifProp struct {
	typ     string
	payload []byte
}

func parseIprp(payload []byte) (props []heifProp, ipma []byte) {
	walkBoxes(payload, func(typ string, body []byte) {
		switch typ {
		case "ipco":
			props = parseIpco(body)
		case "ipma":
			ipma = body
		}
	})
	return props, ipma
}

func parseIpco(payload []byte) []heifProp {
	var props []heifProp
	walkBoxes(payload, func(typ string, body []byte) {
		props = append(props, heifProp{typ: typ, payload: body})
	})
	return props
}

func parseIspe(payload []byte) (int, int, bool) {
	if len(payload) < 12 {
		return 0, 0, false
	}
	w := int(binary.BigEndian.Uint32(payload[4:8]))
	h := int(binary.BigEndian.Uint32(payload[8:12]))
	return w, h, w > 0 && h > 0
}

func parseIrot(payload []byte) (int, bool) {
	if len(payload) < 5 {
		return 0, false
	}
	return int(payload[4] & 3), true
}

func parsePitm(payload []byte) (uint32, bool) {
	if len(payload) < 6 {
		return 0, false
	}
	if payload[0] == 0 {
		return uint32(binary.BigEndian.Uint16(payload[4:6])), true
	}
	if len(payload) < 8 {
		return 0, false
	}
	return binary.BigEndian.Uint32(payload[4:8]), true
}

func selectHeifSize(props []heifProp, ipma []byte, primary uint32, havePrimary bool) (w, h, rot int, ok bool) {
	if havePrimary && len(ipma) > 0 {
		for _, idx := range ipmaIndices(ipma, primary) {
			if idx <= 0 || idx > len(props) {
				continue
			}
			prop := props[idx-1]
			switch prop.typ {
			case "ispe":
				if pw, ph, pok := parseIspe(prop.payload); pok {
					w, h, ok = pw, ph, true
				}
			case "irot":
				if r, rok := parseIrot(prop.payload); rok {
					rot = r
				}
			}
		}
		if ok {
			return w, h, rot, true
		}
	}

	best := 0
	haveRot := false
	for _, prop := range props {
		switch prop.typ {
		case "ispe":
			pw, ph, pok := parseIspe(prop.payload)
			if pok && pw*ph > best {
				best = pw * ph
				w, h, ok = pw, ph, true
			}
		case "irot":
			if !haveRot {
				if r, rok := parseIrot(prop.payload); rok {
					rot = r
					haveRot = true
				}
			}
		}
	}
	return w, h, rot, ok
}

// ipmaIndices returns the 1-based property indexes associated with itemID.
func ipmaIndices(payload []byte, itemID uint32) []int {
	if len(payload) < 8 {
		return nil
	}
	version := payload[0]
	flags := binary.BigEndian.Uint32(payload[0:4]) & 0xFFFFFF
	largeIndex := flags&1 != 0
	count := int(binary.BigEndian.Uint32(payload[4:8]))
	if count < 0 {
		return nil
	}
	pos := 8
	for i := 0; i < count; i++ {
		var id uint32
		if version < 1 {
			if pos+2 > len(payload) {
				return nil
			}
			id = uint32(binary.BigEndian.Uint16(payload[pos:]))
			pos += 2
		} else {
			if pos+4 > len(payload) {
				return nil
			}
			id = binary.BigEndian.Uint32(payload[pos:])
			pos += 4
		}
		if pos >= len(payload) {
			return nil
		}
		n := int(payload[pos])
		pos++
		indices := make([]int, 0, n)
		for j := 0; j < n; j++ {
			var index int
			if largeIndex {
				if pos+2 > len(payload) {
					return nil
				}
				index = int(binary.BigEndian.Uint16(payload[pos:]) & 0x7FFF)
				pos += 2
			} else {
				if pos >= len(payload) {
					return nil
				}
				index = int(payload[pos] & 0x7F)
				pos++
			}
			indices = append(indices, index)
		}
		if id == itemID {
			return indices
		}
	}
	return nil
}

// findBox returns the payload of the first top-level box of typ.
// A box that runs past data is returned truncated so the caller can still
// read the children that did fit in the prefix.
func findBox(data []byte, typ string) ([]byte, bool) {
	var found []byte
	ok := false
	walkBoxes(data, func(boxType string, payload []byte) {
		if !ok && boxType == typ {
			found = payload
			ok = true
		}
	})
	return found, ok
}

// walkBoxes visits each box in data. A truncated final box is still visited
// with the bytes that are present, and the walk then stops.
func walkBoxes(data []byte, visit func(typ string, payload []byte)) {
	for len(data) >= 8 {
		typ, payload, rest, ok := boxPayload(data)
		if !ok {
			return
		}
		visit(typ, payload)
		if rest == nil {
			return
		}
		data = rest
	}
}

func boxPayload(data []byte) (typ string, payload, rest []byte, ok bool) {
	if len(data) < 8 {
		return "", nil, nil, false
	}
	size := uint64(binary.BigEndian.Uint32(data[0:4]))
	typ = string(data[4:8])
	header := uint64(8)
	switch size {
	case 1:
		if len(data) < 16 {
			return "", nil, nil, false
		}
		size = binary.BigEndian.Uint64(data[8:16])
		header = 16
	case 0:
		size = uint64(len(data))
	}
	if size < header {
		return "", nil, nil, false
	}
	if size > uint64(len(data)) {
		return typ, data[header:], nil, true
	}
	end := int(size)
	return typ, data[int(header):end], data[end:], true
}
