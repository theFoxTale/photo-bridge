export type PhotoFormat = 'heic' | 'jpeg' | 'dng' | 'png'

/** Mirrors the Go `backend.Photo` struct (see backend/photos.go). */
export interface Photo {
  id: string
  filename: string
  /** ISO 8601 */
  date: string
  /** bytes */
  size: number
  format: PhotoFormat
  width: number
  height: number
  albumId: string
  /** base64 data URL; empty until fetched with FetchThumbnail */
  thumbUrl: string
}
