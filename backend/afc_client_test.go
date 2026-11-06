package backend

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestValidPhotoID(t *testing.T) {
	valid := []string{"100APPLE/IMG_0001.HEIC", "IMG_0001.JPG", "100APPLE/sub/IMG_0002.PNG"}
	invalid := []string{"", "/DCIM/IMG.JPG", "../etc/passwd", "100APPLE/../../x.jpg", "a//b.jpg", "a/./b.jpg", `a\b.jpg`, ".hidden/IMG.JPG", "100APPLE/.IMG.JPG", "a\x00b.jpg"}

	for _, id := range valid {
		if !validPhotoID(id) {
			t.Errorf("validPhotoID(%q) = false, want true", id)
		}
	}
	for _, id := range invalid {
		if validPhotoID(id) {
			t.Errorf("validPhotoID(%q) = true, want false", id)
		}
	}
}

func TestAlbumDir(t *testing.T) {
	if dir, err := albumDir(cameraRollID); err != nil || dir != "/DCIM" {
		t.Errorf("albumDir(camera-roll) = %q, %v", dir, err)
	}
	if dir, err := albumDir("roll-100APPLE"); err != nil || dir != "/DCIM/100APPLE" {
		t.Errorf("albumDir(roll-100APPLE) = %q, %v", dir, err)
	}
	for _, id := range []string{"favorites", "roll-", "roll-..", "roll-a/b", `roll-a\b`} {
		if _, err := albumDir(id); err == nil {
			t.Errorf("albumDir(%q) succeeded, want error", id)
		}
	}
}

func TestParseStat(t *testing.T) {
	payload := []byte("st_size\x001234\x00st_blocks\x008\x00st_ifmt\x00S_IFREG\x00st_mtime\x001700000000000000000\x00")
	st := parseStat(payload)
	if st.isDir || st.size != 1234 {
		t.Errorf("parseStat = %+v", st)
	}
	if want := time.Unix(1700000000, 0).UTC(); !st.mtime.Equal(want) {
		t.Errorf("mtime = %v, want %v", st.mtime, want)
	}

	if dir := parseStat([]byte("st_ifmt\x00S_IFDIR\x00")); !dir.isDir {
		t.Error("expected directory")
	}
}

func TestParseAFCTime(t *testing.T) {
	want := time.Unix(1700000000, 0).UTC()
	for _, value := range []string{"1700000000", "1700000000000", "1700000000000000", "1700000000000000000"} {
		if got := parseAFCTime(value); !got.Equal(want) {
			t.Errorf("parseAFCTime(%q) = %v, want %v", value, got, want)
		}
	}
	if !parseAFCTime("nope").IsZero() || !parseAFCTime("0").IsZero() {
		t.Error("expected zero time for invalid input")
	}
}

func TestDateFromFilename(t *testing.T) {
	got := dateFromFilename("20240131_235959_abc.JPG")
	if want := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC); !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if !dateFromFilename("IMG_0001.HEIC").IsZero() {
		t.Error("expected zero time for IMG_ names")
	}
}

func TestRollDisplayName(t *testing.T) {
	if got := rollDisplayName("100APPLE"); got != "Folder 100" {
		t.Errorf("got %q", got)
	}
	if got := rollDisplayName("APPLE"); got != "APPLE" {
		t.Errorf("got %q", got)
	}
}

func TestFormatOf(t *testing.T) {
	cases := map[string]string{
		"a.HEIC": "heic", "a.jpg": "jpeg", "a.JPEG": "jpeg", "a.png": "png",
		"a.DNG": "dng", "a.MOV": "mov", "a.mp4": "mp4", "b.MP4": "mp4",
	}
	for name, want := range cases {
		if got, ok := formatOf(name); !ok || got != want {
			t.Errorf("formatOf(%q) = %q, %v", name, got, ok)
		}
	}
	for _, name := range []string{"a.AAE", "a", "a.txt"} {
		if _, ok := formatOf(name); ok {
			t.Errorf("formatOf(%q) = ok, want not ok", name)
		}
	}
}

func TestBuildPhotoMarksVideo(t *testing.T) {
	taken := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	mov := buildPhoto(cameraRollID, "/DCIM/100APPLE/IMG_0001.MOV", fileStat{size: 12, mtime: taken}, 0, 0, 1)
	if !mov.IsVideo || mov.Format != "mov" || mov.Filename != "IMG_0001.MOV" {
		t.Fatalf("mov = %+v", mov)
	}
	mp4 := buildPhoto(cameraRollID, "/DCIM/100APPLE/CLIP.MP4", fileStat{size: 12, mtime: taken}, 0, 0, 1)
	if !mp4.IsVideo || mp4.Format != "mp4" {
		t.Fatalf("mp4 = %+v", mp4)
	}
	jpg := buildPhoto(cameraRollID, "/DCIM/100APPLE/IMG_0001.JPG", fileStat{size: 12, mtime: taken}, 0, 0, 1)
	if jpg.IsVideo || jpg.Format != "jpeg" {
		t.Fatalf("jpg = %+v", jpg)
	}
	dng := buildPhoto(cameraRollID, "/DCIM/100APPLE/IMG_0001.DNG", fileStat{size: 12, mtime: taken}, 0, 0, 1)
	if dng.IsVideo || dng.Format != "dng" || dng.Filename != "IMG_0001.DNG" {
		t.Fatalf("dng = %+v", dng)
	}
}

func TestMergeLivePhotos(t *testing.T) {
	heic := Photo{ID: "100APPLE/IMG_1234.HEIC", Filename: "IMG_1234.HEIC", Format: "heic", Size: 10, Date: "2026-05-01T10:00:00Z"}
	mov := Photo{ID: "100APPLE/IMG_1234.MOV", Filename: "IMG_1234.MOV", Format: "mov", Size: 20, IsVideo: true}
	jpg := Photo{ID: "100APPLE/IMG_1234.JPG", Filename: "IMG_1234.JPG", Format: "jpeg", Size: 30}
	otherMov := Photo{ID: "100APPLE/CLIP.MOV", Filename: "CLIP.MOV", Format: "mov", Size: 4, IsVideo: true}
	mp4 := Photo{ID: "100APPLE/IMG_1234.MP4", Filename: "IMG_1234.MP4", Format: "mp4", Size: 5, IsVideo: true}
	dng := Photo{ID: "100APPLE/IMG_9.DNG", Filename: "IMG_9.DNG", Format: "dng", Size: 6}
	dngMov := Photo{ID: "100APPLE/IMG_9.MOV", Filename: "IMG_9.MOV", Format: "mov", Size: 7, IsVideo: true}

	t.Run("heic and mov become one", func(t *testing.T) {
		got := mergeLivePhotos([]Photo{mov, heic})
		if len(got) != 1 {
			t.Fatalf("got %d photos, want 1: %+v", len(got), got)
		}
		if got[0].ID != heic.ID || got[0].Size != heic.Size || got[0].Date != heic.Date || got[0].Format != "heic" {
			t.Fatalf("still = %+v, want the HEIC unchanged", got[0])
		}
		if !got[0].IsLivePhoto || got[0].LivePhotoVideoID != mov.ID {
			t.Fatalf("live fields = %+v", got[0])
		}
	})

	t.Run("jpg and mov become one", func(t *testing.T) {
		got := mergeLivePhotos([]Photo{jpg, mov})
		if len(got) != 1 || !got[0].IsLivePhoto || got[0].Format != "jpeg" || got[0].LivePhotoVideoID != mov.ID {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("heic wins over jpg", func(t *testing.T) {
		got := mergeLivePhotos([]Photo{jpg, mov, heic})
		if len(got) != 2 {
			t.Fatalf("got %d photos, want the HEIC and the JPG: %+v", len(got), got)
		}
		if got[0].ID != jpg.ID || got[0].IsLivePhoto || got[0].LivePhotoVideoID != "" {
			t.Fatalf("jpg = %+v, want a normal still", got[0])
		}
		if got[1].ID != heic.ID || !got[1].IsLivePhoto || got[1].LivePhotoVideoID != mov.ID {
			t.Fatalf("heic = %+v", got[1])
		}
	})

	t.Run("case and folder", func(t *testing.T) {
		lower := Photo{ID: "100APPLE/img_1234.heic", Filename: "img_1234.heic", Format: "heic", Size: 1}
		upperMov := Photo{ID: "100APPLE/IMG_1234.MOV", Filename: "IMG_1234.MOV", Format: "mov", IsVideo: true}
		otherDir := Photo{ID: "101APPLE/IMG_1234.MOV", Filename: "IMG_1234.MOV", Format: "mov", IsVideo: true}
		got := mergeLivePhotos([]Photo{lower, upperMov, otherDir})
		if len(got) != 2 {
			t.Fatalf("got %+v, want the still and the other folder's mov", got)
		}
		if !got[0].IsLivePhoto || got[0].LivePhotoVideoID != upperMov.ID {
			t.Fatalf("still = %+v", got[0])
		}
		if got[1].ID != otherDir.ID || got[1].IsLivePhoto {
			t.Fatalf("other folder = %+v", got[1])
		}
	})

	t.Run("unpaired videos stay", func(t *testing.T) {
		got := mergeLivePhotos([]Photo{heic, mp4, otherMov, dng, dngMov})
		if len(got) != 5 {
			t.Fatalf("got %d, want all five (mp4 and dng mov are not live companions): %+v", len(got), got)
		}
		for _, p := range got {
			if p.IsLivePhoto {
				t.Fatalf("unexpected live photo %+v", p)
			}
		}
	})

	t.Run("every mov in the group is dropped", func(t *testing.T) {
		second := Photo{ID: "100APPLE/IMG_1234.mov", Filename: "IMG_1234.mov", Format: "mov", IsVideo: true}
		got := mergeLivePhotos([]Photo{mov, second, heic})
		if len(got) != 1 || got[0].LivePhotoVideoID != mov.ID {
			t.Fatalf("got %+v, want one still pointing at the first mov", got)
		}
	})
}

func TestAlbumsFromMediaPathsOmitsPairedLiveVideo(t *testing.T) {
	albums := albumsFromMediaPaths([]string{
		"/DCIM/100APPLE/IMG_1234.HEIC",
		"/DCIM/100APPLE/IMG_1234.MOV",
		"/DCIM/100APPLE/IMG_1235.JPG",
		"/DCIM/100APPLE/CLIP.MP4",
		"/DCIM/101APPLE/IMG_1234.MOV",
	})
	if len(albums) != 3 {
		t.Fatalf("albums = %+v", albums)
	}
	if albums[0].ID != cameraRollID || albums[0].Count != 4 {
		t.Errorf("camera roll = %+v, want count 4", albums[0])
	}
	if albums[1].ID != "roll-100APPLE" || albums[1].Name != "Folder 100" || albums[1].Count != 3 {
		t.Errorf("folder 100 = %+v, want count 3", albums[1])
	}
	if albums[2].ID != "roll-101APPLE" || albums[2].Count != 1 {
		t.Errorf("folder 101 = %+v, want the unpaired mov", albums[2])
	}
}

func TestVisualSize(t *testing.T) {
	cases := []struct {
		name         string
		w, h, orient int
		dw, dh       int
	}{
		{name: "normal", w: 4032, h: 3024, orient: 1, dw: 4032, dh: 3024},
		{name: "mirror", w: 4032, h: 3024, orient: 2, dw: 4032, dh: 3024},
		{name: "portrait clockwise", w: 4032, h: 3024, orient: 6, dw: 3024, dh: 4032},
		{name: "portrait counter clockwise", w: 4032, h: 3024, orient: 8, dw: 3024, dh: 4032},
		{name: "transpose", w: 4032, h: 3024, orient: 5, dw: 3024, dh: 4032},
		{name: "unknown", w: 4032, h: 3024, orient: 0, dw: 4032, dh: 3024},
		{name: "missing", w: 0, h: 3024, orient: 6, dw: 0, dh: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dw, dh := visualSize(tc.w, tc.h, tc.orient)
			if dw != tc.dw || dh != tc.dh {
				t.Fatalf("visualSize(%d, %d, %d) = %d x %d, want %d x %d", tc.w, tc.h, tc.orient, dw, dh, tc.dw, tc.dh)
			}
		})
	}
}

func TestBuildPhotoUsesVisualDimensions(t *testing.T) {
	portrait := buildPhoto(cameraRollID, "/DCIM/100APPLE/IMG_0001.JPG", fileStat{size: 10}, 4032, 3024, 6)
	if portrait.Width != 3024 || portrait.Height != 4032 {
		t.Fatalf("portrait = %d x %d, want 3024 x 4032", portrait.Width, portrait.Height)
	}
	landscape := buildPhoto(cameraRollID, "/DCIM/100APPLE/IMG_0002.JPG", fileStat{size: 10}, 4032, 3024, 1)
	if landscape.Width != 4032 || landscape.Height != 3024 {
		t.Fatalf("landscape = %d x %d, want 4032 x 3024", landscape.Width, landscape.Height)
	}
	unknown := buildPhoto(cameraRollID, "/DCIM/100APPLE/IMG_0003.JPG", fileStat{size: 10}, 0, 0, 6)
	if unknown.Width != 0 || unknown.Height != 0 {
		t.Fatalf("unknown size = %d x %d, want 0 x 0", unknown.Width, unknown.Height)
	}
}

func TestImageHeaderSize(t *testing.T) {
	t.Run("jpeg", func(t *testing.T) {
		raw := orientedJPEG(t, 64, 32, 6, nil)
		w, h, o, ok := imageHeaderSize("jpeg", raw)
		if !ok || w != 64 || h != 32 || o != 6 {
			t.Fatalf("jpeg header = %d x %d orient %d ok %v", w, h, o, ok)
		}
	})

	t.Run("png", func(t *testing.T) {
		raw := pngWithOrientation(t, 40, 10, 8)
		w, h, o, ok := imageHeaderSize("png", raw)
		if !ok || w != 40 || h != 10 || o != 8 {
			t.Fatalf("png header = %d x %d orient %d ok %v", w, h, o, ok)
		}
	})

	t.Run("dng", func(t *testing.T) {
		raw := tiffHeader(binary.LittleEndian, 4032, 3024, 6)
		w, h, o, ok := imageHeaderSize("dng", raw)
		if !ok || w != 4032 || h != 3024 || o != 6 {
			t.Fatalf("dng header = %d x %d orient %d ok %v", w, h, o, ok)
		}
	})

	t.Run("dng big endian", func(t *testing.T) {
		raw := tiffHeader(binary.BigEndian, 4032, 3024, 8)
		w, h, o, ok := imageHeaderSize("dng", raw)
		if !ok || w != 4032 || h != 3024 || o != 8 {
			t.Fatalf("dng header = %d x %d orient %d ok %v", w, h, o, ok)
		}
	})

	t.Run("dng subifd", func(t *testing.T) {
		raw := tiffSubIFD(100, 80, 6)
		w, h, o, ok := imageHeaderSize("dng", raw)
		if !ok || w != 100 || h != 80 || o != 6 {
			t.Fatalf("subifd = %d x %d orient %d ok %v", w, h, o, ok)
		}
	})

	t.Run("heic primary association", func(t *testing.T) {
		raw := testHEIC(heicProps{
			primaryW: 320, primaryH: 240, primaryRot: 1,
			otherW: 4032, otherH: 3024,
		})
		w, h, o, ok := imageHeaderSize("heic", raw)
		if !ok || w != 320 || h != 240 || o != 8 {
			t.Fatalf("heic = %d x %d orient %d ok %v, want the primary item turned by irot", w, h, o, ok)
		}
	})

	t.Run("heic exif when irot is zero", func(t *testing.T) {
		raw := testHEIC(heicProps{primaryW: 4032, primaryH: 3024, exifOrient: 6})
		w, h, o, ok := imageHeaderSize("heic", raw)
		if !ok || w != 4032 || h != 3024 || o != 6 {
			t.Fatalf("heic = %d x %d orient %d ok %v", w, h, o, ok)
		}
	})

	t.Run("heic irot wins over exif", func(t *testing.T) {
		raw := testHEIC(heicProps{primaryW: 4032, primaryH: 3024, primaryRot: 1, exifOrient: 6})
		_, _, o, ok := imageHeaderSize("heic", raw)
		if !ok || o != 8 {
			t.Fatalf("orientation = %d ok %v, want irot 8 rather than EXIF 6", o, ok)
		}
	})

	t.Run("truncated", func(t *testing.T) {
		if _, _, _, ok := imageHeaderSize("jpeg", []byte{0xFF, 0xD8, 0xFF}); ok {
			t.Fatal("a truncated JPEG reported a size")
		}
		if _, _, _, ok := imageHeaderSize("png", []byte("not a png")); ok {
			t.Fatal("a non-PNG reported a size")
		}
	})
}

type heicProps struct {
	primaryW, primaryH uint32
	primaryRot         byte
	otherW, otherH     uint32
	exifOrient         uint16
}

func testHEIC(p heicProps) []byte {
	props := [][]byte{ispeBox(p.primaryW, p.primaryH), irotBox(p.primaryRot)}
	assocCount := byte(2)
	indexes := []byte{1, 2}
	if p.otherW > 0 {
		props = append(props, ispeBox(p.otherW, p.otherH))
	}
	ipco := bmffBox("ipco", bytes.Join(props, nil))
	ipmaBody := []byte{
		0, 0, 0, 0,
		0, 0, 0, 1,
		0, 1,
		assocCount,
	}
	ipmaBody = append(ipmaBody, indexes...)
	iprp := bmffBox("iprp", append(ipco, bmffBox("ipma", ipmaBody)...))
	meta := bmffFullBox("meta", append(bmffFullBox("pitm", []byte{0, 1}), iprp...))
	out := append(bmffBox("ftyp", []byte("heic")), meta...)
	if p.exifOrient != 0 {
		raw := testExif()
		binary.LittleEndian.PutUint16(raw[18:], p.exifOrient)
		out = append(out, exifAPP1Prefix...)
		out = append(out, raw...)
	}
	return out
}

func bmffBox(typ string, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(out[:4], uint32(len(out)))
	copy(out[4:8], typ)
	copy(out[8:], payload)
	return out
}

func bmffFullBox(typ string, body []byte) []byte {
	payload := make([]byte, 4+len(body))
	copy(payload[4:], body)
	return bmffBox(typ, payload)
}

func ispeBox(w, h uint32) []byte {
	body := make([]byte, 8)
	binary.BigEndian.PutUint32(body[0:4], w)
	binary.BigEndian.PutUint32(body[4:8], h)
	return bmffFullBox("ispe", body)
}

func irotBox(rot byte) []byte {
	return bmffFullBox("irot", []byte{rot})
}

func tiffHeader(order binary.ByteOrder, w, h int, orientation uint16) []byte {
	raw := make([]byte, 8+2+3*12+4)
	if order == binary.BigEndian {
		raw[0], raw[1] = 'M', 'M'
	} else {
		raw[0], raw[1] = 'I', 'I'
	}
	order.PutUint16(raw[2:4], 42)
	order.PutUint32(raw[4:8], 8)
	order.PutUint16(raw[8:10], 3)
	putTIFFEntry(raw, order, 10, 0x0100, 4, uint32(w))
	putTIFFEntry(raw, order, 22, 0x0101, 4, uint32(h))
	putTIFFEntry(raw, order, 34, 0x0112, 3, uint32(orientation))
	return raw
}

func tiffSubIFD(w, h int, orientation uint16) []byte {
	const sub = 26
	raw := make([]byte, sub+2+3*12+4)
	raw[0], raw[1] = 'I', 'I'
	binary.LittleEndian.PutUint16(raw[2:4], 42)
	binary.LittleEndian.PutUint32(raw[4:8], 8)
	binary.LittleEndian.PutUint16(raw[8:10], 1)
	putTIFFEntry(raw, binary.LittleEndian, 10, 0x014A, 4, sub)
	binary.LittleEndian.PutUint16(raw[sub:], 3)
	putTIFFEntry(raw, binary.LittleEndian, sub+2, 0x0100, 4, uint32(w))
	putTIFFEntry(raw, binary.LittleEndian, sub+14, 0x0101, 4, uint32(h))
	putTIFFEntry(raw, binary.LittleEndian, sub+26, 0x0112, 3, uint32(orientation))
	return raw
}

func putTIFFEntry(raw []byte, order binary.ByteOrder, off int, tag, typ uint16, value uint32) {
	order.PutUint16(raw[off:], tag)
	order.PutUint16(raw[off+2:], typ)
	order.PutUint32(raw[off+4:], 1)
	if typ == 3 {
		order.PutUint16(raw[off+8:], uint16(value))
		return
	}
	order.PutUint32(raw[off+8:], value)
}

func pngWithOrientation(t *testing.T, w, h int, orientation uint16) []byte {
	t.Helper()
	raw := pngBytes(t, w, h)
	if len(raw) < 12 {
		t.Fatal("png is shorter than an IEND chunk")
	}
	exif := testExif()
	binary.LittleEndian.PutUint16(exif[18:], orientation)
	chunk := make([]byte, 12+len(exif))
	binary.BigEndian.PutUint32(chunk[0:4], uint32(len(exif)))
	copy(chunk[4:8], "eXIf")
	copy(chunk[8:], exif)
	iendAt := len(raw) - 12
	out := make([]byte, 0, len(raw)+len(chunk))
	out = append(out, raw[:iendAt]...)
	out = append(out, chunk...)
	out = append(out, raw[iendAt:]...)
	return out
}
