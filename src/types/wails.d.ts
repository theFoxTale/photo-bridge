import type { Album } from './album'
import type { DiscoveryResult } from './device'
import type { Photo } from './photo'

/**
 * Methods the Go `App` struct exposes to the frontend. Only methods that
 * actually exist in app_discovery.go and app_photos.go are declared here.
 * `go` is optional because it is undefined when the page runs outside the
 * Wails shell (for example plain `vite` in a browser).
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
        }
      }
    }
  }
}

export {}
