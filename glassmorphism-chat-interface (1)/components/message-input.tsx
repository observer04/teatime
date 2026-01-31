"use client"

import { useState, useRef, type KeyboardEvent } from "react"
import { Plus, Smile, Mic, Send } from "lucide-react"
import { cn } from "@/lib/utils"

interface MessageInputProps {
  onSend: (message: string) => void
}

export function MessageInput({ onSend }: MessageInputProps) {
  const [message, setMessage] = useState("")
  const inputRef = useRef<HTMLInputElement>(null)

  const handleSend = () => {
    if (message.trim()) {
      onSend(message.trim())
      setMessage("")
    }
  }

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <div className="flex items-center gap-2 px-4 py-3 bg-card border-t border-border">
      {/* Plus button for attachments */}
      <button
        className="p-2.5 rounded-full text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors flex-shrink-0"
        aria-label="Attach file"
      >
        <Plus className="w-6 h-6" />
      </button>

      {/* Emoji button */}
      <button
        className="p-2.5 rounded-full text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors flex-shrink-0"
        aria-label="Emoji"
      >
        <Smile className="w-6 h-6" />
      </button>

      {/* Input field */}
      <div className="flex-1 relative">
        <input
          ref={inputRef}
          type="text"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Type a message"
          className="w-full px-4 py-2.5 bg-secondary rounded-lg text-sm placeholder:text-muted-foreground focus:outline-none text-foreground"
        />
      </div>

      {/* Mic or Send button */}
      {message.trim() ? (
        <button
          onClick={handleSend}
          className="p-2.5 rounded-full bg-primary text-primary-foreground hover:bg-primary/90 transition-colors flex-shrink-0"
          aria-label="Send message"
        >
          <Send className="w-5 h-5" />
        </button>
      ) : (
        <button
          className="p-2.5 rounded-full text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors flex-shrink-0"
          aria-label="Voice message"
        >
          <Mic className="w-6 h-6" />
        </button>
      )}
    </div>
  )
}
