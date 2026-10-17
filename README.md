# Photo Bridge

Desktop app (Wails v2, Go, Vue 3, TypeScript, Pinia, Tailwind CSS) that browses the photos of a
connected iPhone over Apple File Connection (AFC) using
[go-ios](https://github.com/danielpaulus/go-ios).

## Setup

1. Install go-ios (provides the `ios` command): `npm install -g go-ios`
2. **Windows only:** download `wintun.dll` from <https://git.zx2c4.com/wintun> and copy it to
   `C:/Windows/system32`.
3. **iOS 17+:** run `ios tunnel start` in a separate **elevated** terminal and keep it running
   before you launch the app. The app does not start the tunnel itself.
4. Pair the device with this computer via USB at least once (tap **Trust** on the phone) and enable
   **Show this iPhone when on Wi-Fi** in Apple Devices (iTunes on older setups). Apple Devices must
   be installed, because it provides the `usbmuxd` service that go-ios talks to.
5. Install the Go dependencies once: `go mod tidy`
6. Launch with `wails dev`.

To work on the UI without a phone, run `wails dev -tags mock`. This serves 40 sample photos in 3
albums from a fake "iPhone 15 (mock)" device, and the photo grid shows a "Demo mode" banner. A normal
build never contains or returns mock data.

## How it works

- Devices are discovered through usbmuxd (USB and Wi-Fi Sync), so a device ID is its UDID.
- Photos are read from `/DCIM` on the device (the AFC root is `/var/mobile/Media`).
- Background polling every 15 seconds notices a phone being plugged in or removed. When the active
  device disappears the grid is cleared and shows "Device disconnected".

## Known limitations

- Only **Camera Roll** (everything under `/DCIM`) and one album per roll folder (for example
  "Roll 100") are listed. Favorites, Screenshots and user albums require parsing
  `PhotoData/Photos.sqlite` and will be added later.
- **Thumbnails are a temporary stand-in:** the first 64 KB of a JPEG or PNG is sent as a data URL, so
  large JPEGs render only partly. HEIC, DNG and MOV files cannot be previewed this way and show
  "Preview unavailable". Real thumbnails come in a later stage.
- Image width and height are not available over AFC and are reported as `0`.
- Live Photos appear as two separate files (an image and a MOV). Re-assembly is out of scope.
- The tunnel daemon must be started manually; starting it needs elevated privileges.
- Listing a large library issues one request per file, so it can take a while over Wi-Fi.
- Full-resolution transfer, HEIC to JPG conversion and folder comparison are not implemented yet.
