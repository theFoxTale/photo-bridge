/** Mirrors the Go `backend.Device` struct (see backend/discovery.go). */
export interface Device {
  id: string
  name: string
  ip: string
  model: string
}
