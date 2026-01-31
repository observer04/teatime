import { useState, useEffect } from 'react'
import { FileIcon, Download, Loader2, Image as ImageIcon } from 'lucide-react'
import { UploadService } from '../services/upload'

export function MessageAttachment({ attachmentId, mimeType, filename, sizeBytes }) {
  const [downloadUrl, setDownloadUrl] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [imageLoaded, setImageLoaded] = useState(false)

  const isImage = mimeType?.startsWith('image/')
  const isVideo = mimeType?.startsWith('video/')
  const isAudio = mimeType?.startsWith('audio/')

  useEffect(() => {
    // Auto-load images and videos
    if (isImage || isVideo) {
      loadUrl()
    }
  }, [attachmentId])

  const loadUrl = async () => {
    if (loading || downloadUrl) return

    setLoading(true)
    setError(null)

    try {
      const data = await UploadService.getAttachmentUrl(attachmentId)
      setDownloadUrl(data.download_url)
    } catch (err) {
      console.error('Failed to load attachment:', err)
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleDownload = async () => {
    if (downloadUrl) {
      window.open(downloadUrl, '_blank')
    } else {
      await loadUrl()
    }
  }

  const formatFileSize = (bytes) => {
    if (!bytes) return ''
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }

  // Image attachment
  if (isImage) {
    return (
      <div className="max-w-sm rounded-lg overflow-hidden bg-secondary my-1">
        {loading && !downloadUrl && (
          <div className="w-full h-48 flex items-center justify-center">
            <Loader2 className="w-8 h-8 animate-spin text-muted-foreground" />
          </div>
        )}
        {error && (
          <div className="w-full h-48 flex flex-col items-center justify-center text-destructive p-4">
            <ImageIcon className="w-8 h-8 mb-2" />
            <p className="text-sm">{error}</p>
            <button
              onClick={loadUrl}
              className="mt-2 text-xs underline hover:no-underline"
            >
              Retry
            </button>
          </div>
        )}
        {downloadUrl && (
          <div className="relative group cursor-pointer" onClick={() => window.open(downloadUrl, '_blank')}>
            <img
              src={downloadUrl}
              alt={filename || 'Image attachment'}
              className={`w-full h-auto transition-opacity ${imageLoaded ? 'opacity-100' : 'opacity-0'}`}
              onLoad={() => setImageLoaded(true)}
              loading="lazy"
            />
            {!imageLoaded && (
              <div className="absolute inset-0 flex items-center justify-center">
                <Loader2 className="w-8 h-8 animate-spin text-muted-foreground" />
              </div>
            )}
            <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
              <Download className="w-6 h-6 text-white" />
            </div>
          </div>
        )}
      </div>
    )
  }

  // Video attachment
  if (isVideo && downloadUrl) {
    return (
      <div className="max-w-sm rounded-lg overflow-hidden bg-secondary my-1">
        <video
          src={downloadUrl}
          controls
          className="w-full h-auto"
          preload="metadata"
        >
          Your browser does not support the video tag.
        </video>
      </div>
    )
  }

  // Audio attachment
  if (isAudio && downloadUrl) {
    return (
      <div className="max-w-sm rounded-lg bg-secondary p-3 my-1">
        <audio src={downloadUrl} controls className="w-full">
          Your browser does not support the audio tag.
        </audio>
        {filename && (
          <div className="mt-2 text-xs text-muted-foreground truncate">{filename}</div>
        )}
      </div>
    )
  }

  // Generic file attachment
  return (
    <button
      onClick={handleDownload}
      disabled={loading}
      className="flex items-center gap-3 px-4 py-3 rounded-lg bg-secondary hover:bg-secondary/80 transition-colors my-1 max-w-sm"
    >
      <div className="flex-shrink-0">
        {loading ? (
          <Loader2 className="w-10 h-10 animate-spin text-muted-foreground" />
        ) : (
          <FileIcon className="w-10 h-10 text-muted-foreground" />
        )}
      </div>
      <div className="flex-1 min-w-0 text-left">
        <div className="text-sm font-medium truncate">
          {filename || 'Attachment'}
        </div>
        {sizeBytes && (
          <div className="text-xs text-muted-foreground">
            {formatFileSize(sizeBytes)}
          </div>
        )}
        {error && (
          <div className="text-xs text-destructive mt-1">{error}</div>
        )}
      </div>
      {!loading && (
        <Download className="w-5 h-5 text-muted-foreground flex-shrink-0" />
      )}
    </button>
  )
}
