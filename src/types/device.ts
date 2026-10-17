/** Mirrors the Go `backend.Device` struct (see backend/discovery.go). */
export interface Device {
  id: string
  name: string
  ip: string
  model: string
}

/** Mirrors the Go `backend.DiscoveryResult` struct (see backend/discovery.go). */
export interface DiscoveryResult {
  devices: Device[]
  /** `blocked_by_apple`: a phone is visible via mDNS but usbmuxd lists no device. */
  status: 'idle' | 'connected' | 'blocked_by_apple'
}
