import { useState, useRef, useEffect } from "react"
import { Star, Reply, Forward, Trash2, Copy, X } from "lucide-react"
import api from "../services/api"

export function MessageContextMenu({ 
  message, 
  isOpen, 
  onClose, 
  position,
  onStar,
  onReply,
  onDelete 
}) {
  const menuRef = useRef(null)
  const [loading, setLoading] = useState(false)
  const [isStarred, setIsStarred] = useState(message?.isStarred || false)

  // Update starred state when message changes
  useEffect(() => {
    setIsStarred(message?.isStarred || false)
  }, [message])

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (menuRef.current && !menuRef.current.contains(e.target)) {
        onClose()
      }
    }

    const handleEscape = (e) => {
      if (e.key === 'Escape') onClose()
    }

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside)
      document.addEventListener('keydown', handleEscape)
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleEscape)
    }
  }, [isOpen, onClose])

  if (!isOpen || !message) return null

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(message.content || '')
      onClose()
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }

  const handleStar = async () => {
    setLoading(true)
    try {
      if (isStarred) {
        await api.unstarMessage(message.id)
        setIsStarred(false)
      } else {
        await api.starMessage(message.id)
        setIsStarred(true)
      }
      onStar?.(message, !isStarred)
      onClose()
    } catch (err) {
      console.error('Star failed:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async () => {
    if (!confirm('Delete this message?')) return
    
    setLoading(true)
    try {
      await api.deleteMessage(message.id)
      onDelete?.(message)
      onClose()
    } catch (err) {
      console.error('Delete failed:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleReply = () => {
    onReply?.(message)
    onClose()
  }

  // Calculate menu position to stay within viewport
  const menuStyle = {
    position: 'fixed',
    top: Math.min(position.y, window.innerHeight - 250),
    left: Math.min(position.x, window.innerWidth - 180),
    zIndex: 1000
  }

  return (
    <div 
      ref={menuRef}
      style={menuStyle}
      className="bg-popover border border-border rounded-lg shadow-xl py-1.5 min-w-[160px] animate-in fade-in zoom-in-95 duration-100"
    >
      {/* Copy */}
      {message.content && (
        <button
          onClick={handleCopy}
          className="w-full flex items-center gap-3 px-3 py-2 text-sm text-popover-foreground hover:bg-accent transition-colors"
        >
          <Copy className="w-4 h-4" />
          Copy
        </button>
      )}

      {/* Star/Unstar */}
      <button
        onClick={handleStar}
        disabled={loading}
        className="w-full flex items-center gap-3 px-3 py-2 text-sm text-popover-foreground hover:bg-accent transition-colors"
      >
        <Star className={`w-4 h-4 ${isStarred ? 'fill-yellow-500 text-yellow-500' : ''}`} />
        {isStarred ? 'Unstar' : 'Star'}
      </button>

      {/* Reply (placeholder) */}
      <button
        onClick={handleReply}
        className="w-full flex items-center gap-3 px-3 py-2 text-sm text-popover-foreground hover:bg-accent transition-colors"
      >
        <Reply className="w-4 h-4" />
        Reply
      </button>

      {/* Forward (placeholder) */}
      <button
        onClick={() => { /* TODO: Implement forward */ onClose() }}
        className="w-full flex items-center gap-3 px-3 py-2 text-sm text-popover-foreground hover:bg-accent transition-colors"
      >
        <Forward className="w-4 h-4" />
        Forward
      </button>

      {/* Divider */}
      <div className="my-1.5 border-t border-border" />

      {/* Delete (only for own messages) */}
      {message.isOwn && (
        <button
          onClick={handleDelete}
          disabled={loading}
          className="w-full flex items-center gap-3 px-3 py-2 text-sm text-destructive hover:bg-destructive/10 transition-colors"
        >
          <Trash2 className="w-4 h-4" />
          Delete
        </button>
      )}
    </div>
  )
}
