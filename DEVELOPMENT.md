# Development notes

These are the decisions that look arbitrary until they break a build. The layout of the code is in [ARCHITECTURE.md](ARCHITECTURE.md).

## HEIC uses `adrium/goheif`, not `jdeng/goheif`

`go.mod` contains:

```text
replace github.com/jdeng/goheif => github.com/adrium/goheif v0.0.0-20230113233934-ca402e77a786
```

`jdeng/goheif` does not compile with MinGW on Windows. Its libde265 cgo build (headers, C++ flags, and the way the package expects a Unix toolchain) fails under the compiler Wails projects actually use here. `adrium/goheif` is the fork that builds in that environment, and it is what `backend/convert_heic.go` imports.

The decoder is behind `//go:build cgo`. `backend/convert_noheic.go` is the other half: the same function, returning a clear error, so a build with cgo off still compiles and can still copy the `.HEIC` bytes. It cannot turn them into a JPEG. Build with `CGO_ENABLED=1` and `gcc` from MinGW-w64 on `PATH`. A missing compiler shows up as a cgo or libde265 error, not as a Vue error.

Do not delete the `replace` line to "use the upstream module". That puts the Windows build back on the package that does not compile.

## FFmpeg has to be installed on the machine that runs the app

Go's image packages decode JPEG, PNG, and (through goheif) HEIC. They do not decode a MOV, an MP4, or a DNG. Those three go through the `ffmpeg` binary:

- A video thumbnail is one frame, at 1 second, or at 0 seconds when the clip is shorter.
- A DNG thumbnail is one frame from the raw file.
- A DNG transferred as a JPEG is `ffmpeg -i <file> -map_metadata 0 -q:v <n>`. Quality 100 on the slider is `-q:v 2`. Quality 1 is `-q:v 31`.

The process is started with `exec.LookPath("ffmpeg")`. A missing binary is `exec.ErrNotFound`, not an `ExitError`. `ExitError` means FFmpeg started and then failed, which is what a corrupt file does. The missing-binary warning is logged once: "FFmpeg not found. Video thumbnails will be unavailable." The same log covers DNG, because both paths share that check. The transfer of an original file does not need FFmpeg. JPG conversion of a DNG does.

On Windows the process is created with `CREATE_NO_WINDOW` (`backend/exec_windows.go`). Without that, every preview opens a console over the Wails window.

Installing FFmpeg is not enough if the app's process cannot see it. WinGet often puts the binary under `%LOCALAPPDATA%\Microsoft\WinGet\Packages\...\bin` and does not add that directory to the PATH of an already-open terminal. A new terminal, or an explicit PATH entry, is what `LookPath` uses.

## SQLite is `modernc.org/sqlite` on purpose

The hash cache is opened with `database/sql` and the driver name `"sqlite"`. The import is `modernc.org/sqlite`, which is a pure Go SQLite. It does not need cgo, and it does not need the SQLite C library.

That split is easy to mix up with HEIC. The database works in a `CGO_ENABLED=0` build. HEIC decoding does not. Turning cgo off to "simplify the build" keeps comparison and original-file copy, and breaks HEIC to JPEG.

The cache file is `%APPDATA%\PhotoBridge\hash-cache.db` (`os.UserConfigDir`). An entry is valid only for the same key, size, and modification time. A failed open is logged, remembered for the session, and ignored: comparison then hashes every candidate again. A nil `*HashCache` is a supported state, including in tests.

## Wi-Fi Sync is a checkbox inside Apple Devices

usbmuxd on Windows is the service installed by Apple Devices (or by iTunes on older machines). go-ios can only see a phone that this service lists. Browsing `_apple-mobdev2._tcp` is a diagnostic, not a connection. mDNS answering while usbmuxd is empty is reported as `blocked_by_apple`.

For a phone that is not plugged in, Apple Devices lists it only when **Show this iPhone when on Wi-Fi** is enabled for that device. The phone also has to have been paired once over USB, with Trust accepted. If the checkbox is off, the phone can be on the same network and the app will still show no device, or the blocked status. Enabling the checkbox, then waiting for the 15 second poll (or starting discovery again), is the fix. Unlocking the phone matters the first time AFC is opened; a locked phone fails that handshake even when usbmuxd already lists it.

On iOS 17 and newer there is a second, separate requirement. Remote access goes through a tunnel that this app does not start. Run `ios tunnel start` in an elevated terminal and leave it running. On Windows that tunnel needs `wintun.dll` in `C:\Windows\system32`. A tunnel error is surfaced as "Device tunnel not running", which is not the same failure as the Wi-Fi checkbox.
