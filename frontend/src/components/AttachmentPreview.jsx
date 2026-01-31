import { X, FileIcon, Loader2 } from 'lucide-react'

export function AttachmentPreview({ file, onRemove, uploadProgress }) {
  const isImage = file.type.startsWith('image/')
  const isUploading = uploadProgress !== null && uploadProgress < 100

  return (
    <div className="relative inline-block mr-2 mb-2">
      <div className="w-20 h-20 rounded-lg border border-border bg-secondary overflow-hidden">
        {isImage ? (
          <img
            src={URL.createObjectURL(file)}
            alt={file.name}
            className="w-full h-full object-cover"
          />
        ) : (
          <div className="w-full h-full flex items-center justify-center">
            <FileIcon className="w-8 h-8 text-muted-foreground" />
          </div>
        )}
        
        {isUploading && (
          <div className="absolute inset-0 bg-background/80 flex items-center justify-center">
            <div className="text-center">
              <Loader2 className="w-6 h-6 animate-spin text-primary mx-auto mb-1" />
              <div className="text-xs text-muted-foreground">{Math.round(uploadProgress)}%</div>
            </div>
          </div>
        )}
      </div>

      {!isUploading && onRemove && (
        <button
          onClick={onRemove}
          className="absolute -top-2 -right-2 w-6 h-6 rounded-full bg-destructive text-destructive-foreground hover:bg-destructive/90 flex items-center justify-center"
          aria-label="Remove attachment"
        >
          <X className="w-4 h-4" />
        </button>
      )}

      <div className="mt-1 text-xs text-muted-foreground truncate max-w-[80px]" title={file.name}>
        {file.name}
      </div>
    </div>
  )
}

export function AttachmentList({ attachments, onRemove, uploadProgress }) {
  if (attachments.length === 0) return null

  return (
    <div className="px-4 py-2 border-t border-border bg-card">
      <div className="flex flex-wrap">
        {attachments.map((file, index) => (
          <AttachmentPreview
            key={`${file.name}-${index}`}
            file={file}
            onRemove={() => onRemove(index)}
            uploadProgress={uploadProgress?.[index] ?? null}
          />
        ))}
      </div>
    </div>
  )
}
