import { useRef } from 'react'
import { Paperclip, ImageIcon } from 'lucide-react'

export function FileUploadButton({ onFileSelect, accept, icon: Icon = Paperclip, label = 'Attach file' }) {
  const fileInputRef = useRef(null)

  const handleClick = () => {
    fileInputRef.current?.click()
  }

  const handleFileChange = (e) => {
    const files = Array.from(e.target.files || [])
    if (files.length > 0) {
      onFileSelect(files)
    }
    // Reset input so same file can be selected again
    e.target.value = ''
  }

  return (
    <>
      <button
        onClick={handleClick}
        className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-card transition-colors"
        aria-label={label}
        type="button"
      >
        <Icon className="w-5 h-5" />
      </button>
      <input
        ref={fileInputRef}
        type="file"
        accept={accept}
        onChange={handleFileChange}
        className="hidden"
        multiple
      />
    </>
  )
}

export function ImageUploadButton({ onFileSelect }) {
  return (
    <FileUploadButton
      onFileSelect={onFileSelect}
      accept="image/*"
      icon={ImageIcon}
      label="Attach image"
    />
  )
}
