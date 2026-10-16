import type { Device } from './device'

/**
 * Methods the Go `App` struct exposes to the frontend. Only methods that
 * actually exist in app_discovery.go are declared here. `go` is optional
 * because it is undefined when the page runs outside the Wails shell
 * (for example plain `vite` in a browser).
 */
declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          DiscoverDevices(): Promise<Device[]>
          StopDiscovery(): Promise<void>
        }
      }
    }
  }
}

export {}
