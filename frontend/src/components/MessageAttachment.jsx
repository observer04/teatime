import { useState, useEffect, useCallback } from 'react'
import { FileIcon, Download, Loader2, Image as ImageIcon, Play, Music, Eye } from 'lucide-react'
import { UploadService } from '../services/upload'

// Cache for attachment URLs (persists during session)
const attachmentCache = new Map();

export function MessageAttachment({ attachmentId, mimeType, filename, sizeBytes }) {
  const [downloadUrl, setDownloadUrl] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [imageLoaded, setImageLoaded] = useState(false)
  const [showPreview, setShowPreview] = useState(false)

  const isImage = mimeType?.startsWith('image/')
  const isVideo = mimeType?.startsWith('video/')
  const isAudio = mimeType?.startsWith('audio/')

  // Check cache on mount
  useEffect(() => {
    const cachedUrl = attachmentCache.get(attachmentId);
    if (cachedUrl) {
      setDownloadUrl(cachedUrl);
      setShowPreview(true);
    }
  }, [attachmentId]);

  const loadUrl = useCallback(async () => {
    if (loading) return

    // Check cache first
    const cachedUrl = attachmentCache.get(attachmentId);
    if (cachedUrl) {
      setDownloadUrl(cachedUrl);
      setShowPreview(true);
      return;
    }

    setLoading(true)
    setError(null)

    try {
      const data = await UploadService.getAttachmentUrl(attachmentId)
      const url = data.download_url;
      // Cache the URL
      attachmentCache.set(attachmentId, url);
      setDownloadUrl(url)
      setShowPreview(true)
    } catch (err) {
      console.error('Failed to load attachment:', err)
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }, [attachmentId, loading]);

  const handleClick = async () => {
    if (showPreview && downloadUrl) {
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

  // Image attachment - show placeholder until clicked
  if (isImage) {
    if (!showPreview) {
      return (
        <button
          onClick={handleClick}
          disabled={loading}
          className="w-48 h-32 rounded-lg bg-secondary hover:bg-secondary/80 transition-colors flex flex-col items-center justify-center gap-2 cursor-pointer"
        >
          {loading ? (
            <Loader2 className="w-8 h-8 animate-spin text-muted-foreground" />
          ) : (
            <>
              <div className="relative">
                <ImageIcon className="w-10 h-10 text-muted-foreground" />
                <Eye className="w-4 h-4 text-primary absolute -bottom-1 -right-1 bg-secondary rounded-full" />
              </div>
              <span className="text-xs text-muted-foreground">Click to load image</span>
              {sizeBytes && (
                <span className="text-xs text-muted-foreground">{formatFileSize(sizeBytes)}</span>
              )}
            </>
          )}
        </button>
      )
    }

    return (
      <div className="max-w-sm rounded-lg overflow-hidden bg-secondary my-1">
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
              <div className="absolute inset-0 flex items-center justify-center bg-secondary">
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

  // Video attachment - show placeholder until clicked
  if (isVideo) {
    if (!showPreview) {
      return (
        <button
          onClick={handleClick}
          disabled={loading}
          className="w-48 h-32 rounded-lg bg-secondary hover:bg-secondary/80 transition-colors flex flex-col items-center justify-center gap-2 cursor-pointer"
        >
          {loading ? (
            <Loader2 className="w-8 h-8 animate-spin text-muted-foreground" />
          ) : (
            <>
              <div className="relative">
                <div className="w-12 h-12 rounded-full bg-primary/20 flex items-center justify-center">
                  <Play className="w-6 h-6 text-primary ml-1" />
                </div>
              </div>
              <span className="text-xs text-muted-foreground">Click to load video</span>
              {sizeBytes && (
                <span className="text-xs text-muted-foreground">{formatFileSize(sizeBytes)}</span>
              )}
            </>
          )}
        </button>
      )
    }

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

  // Audio attachment - show placeholder until clicked
  if (isAudio) {
    if (!showPreview) {
      return (
        <button
          onClick={handleClick}
          disabled={loading}
          className="flex items-center gap-3 px-4 py-3 rounded-lg bg-secondary hover:bg-secondary/80 transition-colors my-1 max-w-sm"
        >
          {loading ? (
            <Loader2 className="w-8 h-8 animate-spin text-muted-foreground" />
          ) : (
            <>
              <Music className="w-8 h-8 text-muted-foreground" />
              <div className="text-left">
                <div className="text-sm truncate">{filename || 'Audio'}</div>
                <div className="text-xs text-muted-foreground">
                  Click to play • {formatFileSize(sizeBytes)}
                </div>
              </div>
            </>
          )}
        </button>
      )
    }

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
      onClick={handleClick}
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
