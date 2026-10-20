import type { Album } from './album'
import type { DiscoveryResult } from './device'
import type { Photo } from './photo'
import type { TransferOptions, TransferResult } from './transfer'

/**
 * Methods the Go `App` struct exposes to the frontend. Only methods that
 * actually exist in app_discovery.go, app_photos.go and app_transfer.go are
 * declared here. `go` is optional because it is undefined when the page runs
 * outside the Wails shell (for example plain `vite` in a browser).
 *
 * `runtime` is the Wails runtime, of which only event subscription is used.
 */
declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          DiscoverDevices(): Promise<DiscoveryResult>
          CheckMdnsDiagnostics(): Promise<boolean>
          StopDiscovery(): Promise<void>
          LibraryMode(): Promise<'device' | 'mock'>
          FetchAlbums(deviceId: string): Promise<Album[]>
          FetchPhotos(deviceId: string, albumId: string): Promise<Photo[]>
          FetchThumbnail(deviceId: string, photoId: string): Promise<string>
          /** Resolves to an empty string when the user cancels the dialog. */
          SelectFolder(): Promise<string>
          TransferPhotos(
            deviceId: string,
            photoIds: string[],
            options: TransferOptions,
          ): Promise<TransferResult>
          CancelTransfer(): Promise<void>
        }
      }
    }
    runtime?: {
      /** Returns a function that removes the listener. */
      EventsOn(eventName: string, callback: (...data: unknown[]) => void): () => void
    }
  }
}

export {}
