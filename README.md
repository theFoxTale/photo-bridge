# Photo Bridge

Photo Bridge copies a camera roll from an iPhone to a folder on this PC. It is a desktop app: a Vue 3 interface inside a [Wails v2](https://wails.io) window, with Go talking to the phone over Apple File Connection.

The phone must be paired with this computer. After that it can stay on USB, or on the same network with Wi-Fi Sync turned on.

## Features

- Browse Camera Roll, and each camera roll folder, over USB or Wi-Fi Sync. The library refreshes every 15 seconds.
- Preview JPEG, PNG, and HEIC in the app. MOV and MP4 show a frame grabbed by FFmpeg, with a play icon. DNG (Apple ProRAW) shows a preview from the same tool, with a RAW badge.
- Copy the original file, a JPEG, or both. HEIC becomes a JPEG with EXIF kept. DNG becomes a high-quality JPEG through FFmpeg. Video is copied as the original file and is not turned into a JPEG.
- Place files in the destination folder, or in `YYYY/MM` or `YYYY/MM/DD` folders.
- Compare a folder on the PC with the camera roll. Files are matched by date and size, then checked with SHA-256. Unchanged sums are remembered in a local SQLite database, so the next comparison does not hash them again.

## Prerequisites

- [Go](https://go.dev/dl/) 1.26 or newer (`go.mod` requires 1.26).
- [Node.js](https://nodejs.org/) and npm, for the Vue frontend.
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`.
- A C compiler. On Windows, install [MinGW-w64](https://www.mingw-w64.org/) and put `gcc` on `PATH`. HEIC decoding is cgo and will not build without it.
- [FFmpeg](https://ffmpeg.org/) on `PATH`. Video thumbnails and every DNG preview or JPEG conversion call `ffmpeg`.
- [Apple Devices](https://apps.microsoft.com/detail/9np83lwlpz9k) (or iTunes). It provides the usbmuxd service go-ios connects to.
- The [`ios` command from go-ios](https://github.com/danielpaulus/go-ios), for the iOS 17+ tunnel: `npm install -g go-ios`.

On Windows, the tunnel also needs `wintun.dll`. Download it from <https://git.zx2c4.com/wintun> and copy it to `C:\Windows\system32`.

## Connect an iPhone

1. Connect the phone by USB once and tap **Trust**.
2. In Apple Devices, enable **Show this iPhone when on Wi-Fi**. Without that checkbox, usbmuxd will not list the phone when it is off the cable, even if the phone is visible on the network.
3. Leave the phone unlocked the first time AFC opens.
4. On iOS 17 or newer, start the tunnel before the app and leave it running. The app does not start the tunnel itself:

   ```powershell
   ios tunnel start
   ```

   That terminal must be elevated.

## Build and run

HEIC support is compiled only when cgo is on. Set it for every build and for `wails dev`.

PowerShell:

```powershell
$env:CGO_ENABLED = "1"
wails dev
```

A production binary:

```powershell
$env:CGO_ENABLED = "1"
wails build
```

macOS or Linux:

```bash
CGO_ENABLED=1 wails dev
CGO_ENABLED=1 wails build
```

The first run installs frontend packages (`npm install`, from `wails.json`) and Go modules. To refresh modules yourself: `go mod tidy`.

To work on the interface without a phone:

```powershell
$env:CGO_ENABLED = "1"
wails dev -tags mock
```

That build serves sample photos from a fake device and shows a demo banner. A normal build does not contain that data.

Backend tests:

```powershell
go test ./backend/
```

Tests that decode a real video or DNG skip themselves when `ffmpeg` is not on `PATH`.

## How a session works

Discovery asks usbmuxd for devices. The id of a device is its UDID. Photos are read from `/DCIM` (the AFC root is `/var/mobile/Media`). Thumbnails are fetched lazily, resized to a 400-pixel long side, and cached under `%APPDATA%\PhotoBridge\thumbnails`. Comparison state is a SQLite file at `%APPDATA%\PhotoBridge\hash-cache.db`.

A longer description of the processes, with diagrams, is in [ARCHITECTURE.md](ARCHITECTURE.md). Notes for the next person who builds this are in [DEVELOPMENT.md](DEVELOPMENT.md).

## Limitations

- Only Camera Roll and one album per roll folder are listed. Favorites, Screenshots, and user albums need `PhotoData/Photos.sqlite`, which is not parsed.
- Width and height from the phone are 0. AFC does not report them.
- A Live Photo is two files, the still and the MOV. They are not joined back together.
- Listing asks the phone for every file, so a large library is slow over Wi-Fi.
- The iOS 17 tunnel is started by hand, and starting it needs an elevated terminal.
- Without FFmpeg, video and DNG previews fail and the tile shows a placeholder. The rest of the app still runs.
- Without `CGO_ENABLED=1`, HEIC files can still be copied, but they cannot be decoded to JPEG.
