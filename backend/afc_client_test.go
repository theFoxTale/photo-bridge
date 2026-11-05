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
	mov := buildPhoto(cameraRollID, "/DCIM/100APPLE/IMG_0001.MOV", fileStat{size: 12, mtime: taken})
	if !mov.IsVideo || mov.Format != "mov" || mov.Filename != "IMG_0001.MOV" {
		t.Fatalf("mov = %+v", mov)
	}
	mp4 := buildPhoto(cameraRollID, "/DCIM/100APPLE/CLIP.MP4", fileStat{size: 12, mtime: taken})
	if !mp4.IsVideo || mp4.Format != "mp4" {
		t.Fatalf("mp4 = %+v", mp4)
	}
	jpg := buildPhoto(cameraRollID, "/DCIM/100APPLE/IMG_0001.JPG", fileStat{size: 12, mtime: taken})
	if jpg.IsVideo || jpg.Format != "jpeg" {
		t.Fatalf("jpg = %+v", jpg)
	}
	dng := buildPhoto(cameraRollID, "/DCIM/100APPLE/IMG_0001.DNG", fileStat{size: 12, mtime: taken})
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
