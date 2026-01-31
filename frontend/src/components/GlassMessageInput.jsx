import { useState, useRef, useEffect } from "react"
import { Smile, Send, Mic } from "lucide-react"
import { FileUploadButton, ImageUploadButton } from "./FileUploadButton"
import { AttachmentList } from "./AttachmentPreview"
import { UploadService } from "../services/upload"

export function GlassMessageInput({ onSend, conversationId, disabled = false }) {
  const [message, setMessage] = useState("")
  const [isFocused, setIsFocused] = useState(false)
  const [attachments, setAttachments] = useState([])
  const [uploadProgress, setUploadProgress] = useState({})
  const [isUploading, setIsUploading] = useState(false)
  const textareaRef = useRef(null)

  const handleSend = async () => {
    if ((!message.trim() && attachments.length === 0) || disabled || isUploading) {
      return
    }

    try {
      let attachmentIds = []

      // Upload attachments if any
      if (attachments.length > 0) {
        setIsUploading(true)
        attachmentIds = await Promise.all(
          attachments.map(async (file, index) => {
            const result = await UploadService.uploadFile(
              conversationId,
              file,
              (progress) => {
                setUploadProgress((prev) => ({ ...prev, [index]: progress }))
              }
            )
            return result.attachmentId
          })
        )
      }

      // Send message with attachments
      onSend(message.trim() || null, attachmentIds.length > 0 ? attachmentIds[0] : null)

      // Reset state
      setMessage("")
      setAttachments([])
      setUploadProgress({})
      if (textareaRef.current) {
        textareaRef.current.style.height = 'auto'
      }
    } catch (error) {
      console.error('Failed to send message:', error)
      alert(error.message || 'Failed to send message')
    } finally {
      setIsUploading(false)
    }
  }

  const handleKeyDown = (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleFileSelect = (files) => {
    const validFiles = []
    for (const file of files) {
      const validation = UploadService.validateFile(file)
      if (validation.valid) {
        validFiles.push(file)
      } else {
        alert(`${file.name}: ${validation.error}`)
      }
    }
    if (validFiles.length > 0) {
      setAttachments((prev) => [...prev, ...validFiles])
    }
  }

  const handleRemoveAttachment = (index) => {
    setAttachments((prev) => prev.filter((_, i) => i !== index))
    setUploadProgress((prev) => {
      const newProgress = { ...prev }
      delete newProgress[index]
      return newProgress
    })
  }

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      textareaRef.current.style.height = textareaRef.current.scrollHeight + 'px'
    }
  }, [message])

  return (
    <>
      <AttachmentList
        attachments={attachments}
        onRemove={handleRemoveAttachment}
        uploadProgress={uploadProgress}
      />
      <div className="px-4 py-3 border-t border-border bg-card">
        <div
          className={`flex items-end gap-2 p-2 rounded-2xl bg-secondary transition-all ${
            isFocused ? "ring-1 ring-primary" : ""
          }`}
        >
          <div className="flex items-center gap-1 flex-shrink-0">
            <FileUploadButton onFileSelect={handleFileSelect} />
            <ImageUploadButton onFileSelect={handleFileSelect} />
            <button
              onClick={() => console.log("Add emoji")}
              className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-card transition-colors"
              aria-label="Add emoji"
              type="button"
            >
              <Smile className="w-5 h-5" />
            </button>
          </div>

          <textarea
            ref={textareaRef}
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            onKeyDown={handleKeyDown}
            onFocus={() => setIsFocused(true)}
            onBlur={() => setIsFocused(false)}
            placeholder="Type a message"
            disabled={disabled || isUploading}
            rows={1}
            className="flex-1 bg-transparent resize-none outline-none text-sm text-foreground placeholder:text-muted-foreground max-h-32 py-2"
            style={{ minHeight: '24px' }}
          />

          <button
            onClick={handleSend}
            disabled={(!message.trim() && attachments.length === 0) || disabled || isUploading}
            className={`p-2 rounded-lg flex-shrink-0 transition-all ${
              (message.trim() || attachments.length > 0) && !disabled && !isUploading
                ? "bg-primary text-primary-foreground hover:bg-primary/90"
                : "text-muted-foreground"
            }`}
            aria-label={message.trim() || attachments.length > 0 ? "Send message" : "Voice message"}
            type="button"
          >
            {message.trim() || attachments.length > 0 ? <Send className="w-5 h-5" /> : <Mic className="w-5 h-5" />}
          </button>
        </div>
      </div>
    </>
  )
}
