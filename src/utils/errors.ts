import { i18n } from '../i18n'

/** Exact backend and frontend messages, mapped before they are shown. */
const EXACT: Record<string, string> = {
  'Unknown error': 'errors.unknown',
  'Device disconnected': 'errors.deviceDisconnected',
  'Cannot load albums without a device.': 'errors.albumsNeedDevice',
  'Cannot load photos without a device and an album.': 'errors.photosNeedDevice',
  'Select at least one photo to transfer.': 'errors.selectPhoto',
  'No device is connected.': 'errors.noDevice',
  'Choose a destination folder first.': 'errors.chooseDestinationFirst',
  'Choose a folder to compare with.': 'errors.chooseCompareFolder',
  'choose a folder to compare with': 'errors.chooseCompareFolder',
  'deviceID must not be empty': 'errors.deviceIdEmpty',
  'no photos selected': 'errors.noPhotosSelected',
  'photo IDs must not be empty': 'errors.photoIdsEmpty',
  'choose at least one format to save: original or JPG': 'errors.chooseFormat',
  'choose a destination folder': 'errors.chooseDestination',
  'the photo has no file name': 'errors.noFileName',
  'Cannot convert video to JPG': 'errors.cannotConvertVideo',
  'photo library is closed': 'errors.libraryClosed',
  'albumID must not be empty': 'errors.albumIdEmpty',
  'destPath must not be empty': 'errors.destPathEmpty',
  'comparison cancelled': 'errors.compareCancelled',
  'a transfer is already running': 'errors.transferAlreadyRunning',
  'a comparison is already running': 'errors.compareAlreadyRunning',
  'Device tunnel not running. Run `ios tunnel start` in an elevated terminal.': 'errors.tunnelNotRunning',
  'Could not open AFC connection. Check that the device is unlocked and Wi-Fi Sync is enabled.':
    'errors.afcUnavailable',
  'Device not found. Check that it is connected, awake and on the same network.': 'errors.deviceNotFound',
  'Device is not paired with this computer. Connect it via USB once and tap Trust.': 'errors.notPaired',
  'Could not reach usbmuxd. Install Apple Devices (or iTunes) and make sure its device service is running.':
    'errors.usbmuxdMissing',
  'Lost the connection to the device. Check that it is unlocked and still reachable.': 'errors.connectionLost',
  'file is not a HEIC or JPEG image': 'errors.notImage',
  'convert DNG to JPG: ffmpeg wrote no JPEG': 'errors.ffmpegNoJpeg',
  'context is required': 'errors.contextRequired',
  'choose a cache folder': 'errors.chooseCacheFolder',
  'the application window is not ready yet': 'errors.windowNotReady',
  'choose a folder that is not the application settings folder': 'errors.settingsFolder',
}

/** Longer prefixes first so a specific wrapper wins over a shorter one. */
const PREFIXES: readonly { prefix: string; key: string }[] = [
  { prefix: 'cannot access destination folder: ', key: 'errors.destinationAccess' },
  { prefix: 'destination folder does not exist: ', key: 'errors.destinationMissing' },
  { prefix: 'destination is not a folder: ', key: 'errors.destinationNotFolder' },
  { prefix: 'invalid destination folder: ', key: 'errors.invalidDestination' },
  { prefix: 'cannot write to the destination folder: ', key: 'errors.destinationWrite' },
  { prefix: 'folder structure must be flat, year_month, or year_month_day, got ', key: 'errors.folderStructure' },
  { prefix: 'JPG conversion is not supported for ', key: 'errors.jpgUnsupported' },
  { prefix: 'JPG quality must be between 1 and 100, got ', key: 'errors.jpgQuality' },
  { prefix: 'too many photos selected (', key: 'errors.tooManyPhotos' },
  { prefix: 'list the photos on the device: ', key: 'errors.listPhotos' },
  { prefix: 'cannot access folder: ', key: 'errors.folderAccess' },
  { prefix: 'folder does not exist: ', key: 'errors.folderMissing' },
  { prefix: 'invalid folder: ', key: 'errors.invalidFolder' },
  { prefix: 'not a folder: ', key: 'errors.notAFolder' },
  { prefix: 'save Live Photo video: ', key: 'errors.saveLiveVideo' },
  { prefix: 'ffmpeg not available: ', key: 'errors.ffmpegMissing' },
  { prefix: 'convert DNG to JPG: ', key: 'errors.convertDng' },
  { prefix: 'photo preview: ', key: 'errors.photoPreview' },
  { prefix: 'preview not available for ', key: 'errors.previewUnavailable' },
  { prefix: 'save original: ', key: 'errors.saveOriginal' },
  { prefix: 'convert to JPG: ', key: 'errors.convertJpg' },
  { prefix: 'save JPG: ', key: 'errors.saveJpg' },
  { prefix: 'Unknown device: ', key: 'errors.unknownDevice' },
  { prefix: 'cache folder is not a directory: ', key: 'errors.cacheNotDirectory' },
  { prefix: 'remove previous cache folder: ', key: 'errors.removePreviousCache' },
  { prefix: 'cannot access cache folder: ', key: 'errors.cacheFolderAccess' },
  { prefix: 'select cache folder: ', key: 'errors.selectCacheFolder' },
  { prefix: 'invalid cache folder: ', key: 'errors.invalidCacheFolder' },
  { prefix: 'create cache folder: ', key: 'errors.createCacheFolder' },
  { prefix: 'clear cache: ', key: 'errors.clearCache' },
  { prefix: 'cache stats: ', key: 'errors.cacheStats' },
  { prefix: 'read config: ', key: 'errors.readConfig' },
  { prefix: 'save config: ', key: 'errors.saveConfig' },
]

const FILE_UNAVAILABLE_SUFFIX = ' is not available on the device'

/**
 * Translates a known error string. Anything unrecognized is returned unchanged
 * so a new backend message is still visible.
 */
export function translateError(message: string | null | undefined): string {
  if (message == null || message === '') return ''
  // Reading the locale keeps callers that render this during setup reactive.
  void i18n.global.locale.value

  const exact = EXACT[message]
  if (exact) return i18n.global.t(exact)

  if (message.endsWith(FILE_UNAVAILABLE_SUFFIX)) {
    const name = message.slice(0, -FILE_UNAVAILABLE_SUFFIX.length)
    return i18n.global.t('errors.fileUnavailable', { detail: name })
  }

  for (const entry of PREFIXES) {
    if (message.startsWith(entry.prefix)) {
      const detail = translateError(message.slice(entry.prefix.length))
      return i18n.global.t(entry.key, { detail })
    }
  }

  return message
}
