package backend

import (
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
	if got := rollDisplayName("100APPLE"); got != "Roll 100" {
		t.Errorf("got %q", got)
	}
	if got := rollDisplayName("APPLE"); got != "APPLE" {
		t.Errorf("got %q", got)
	}
}

func TestFormatOf(t *testing.T) {
	cases := map[string]string{"a.HEIC": "heic", "a.jpg": "jpeg", "a.JPEG": "jpeg", "a.png": "png", "a.DNG": "dng", "a.MOV": "mov"}
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
