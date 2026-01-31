
import { useState, useRef, type KeyboardEvent } from "react"
import { Paperclip, ImageIcon, Smile, Send, Mic, AtSign } from "lucide-react"
import { cn } from "@/lib/utils"

interface MessageInputProps {
  onSend: (message: string) => void
}

export function MessageInput({ onSend }: MessageInputProps) {
  const [message, setMessage] = useState("")
  const [isFocused, setIsFocused] = useState(false)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  const handleSend = () => {
    if (message.trim()) {
      onSend(message.trim())
      setMessage("")
    }
  }

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const attachmentButtons = [
    { icon: Paperclip, label: "Attach file" },
    { icon: ImageIcon, label: "Add image" },
    { icon: AtSign, label: "Mention someone" },
    { icon: Smile, label: "Add emoji" },
  ]

  return (
    <div className="p-4 border-t border-border">
      <div
        className={cn(
          "relative rounded-2xl border bg-card transition-all duration-200",
          isFocused ? "border-primary shadow-lg shadow-primary/10" : "border-border"
        )}
      >
        {/* Attachment buttons */}
        <div className="flex items-center gap-1 px-3 pt-3">
          {attachmentButtons.map(({ icon: Icon, label }) => (
            <button
              key={label}
              className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
              aria-label={label}
            >
              <Icon className="w-5 h-5" />
            </button>
          ))}
        </div>

        {/* Input area */}
        <div className="flex items-end gap-3 p-3 pt-2">
          <textarea
            ref={inputRef}
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            onKeyDown={handleKeyDown}
            onFocus={() => setIsFocused(true)}
            onBlur={() => setIsFocused(false)}
            placeholder="Type your message..."
            rows={1}
            className="flex-1 resize-none bg-transparent text-foreground placeholder:text-muted-foreground focus:outline-none text-sm leading-relaxed max-h-32 overflow-y-auto"
            style={{ minHeight: "24px" }}
          />
          
          <div className="flex items-center gap-2">
            <button
              className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
              aria-label="Voice message"
            >
              <Mic className="w-5 h-5" />
            </button>
            <button
              onClick={handleSend}
              disabled={!message.trim()}
              className={cn(
                "p-2.5 rounded-xl transition-all duration-200",
                message.trim()
                  ? "bg-primary text-primary-foreground hover:bg-primary/90 shadow-md shadow-primary/30"
                  : "bg-secondary text-muted-foreground cursor-not-allowed"
              )}
              aria-label="Send message"
            >
              <Send className="w-5 h-5" />
            </button>
          </div>
        </div>
      </div>
      
      {/* Typing indicator or hint */}
      <p className="text-xs text-muted-foreground mt-2 ml-1">
        Press <kbd className="px-1.5 py-0.5 rounded bg-secondary text-foreground text-[10px] font-mono">Enter</kbd> to send, <kbd className="px-1.5 py-0.5 rounded bg-secondary text-foreground text-[10px] font-mono">Shift + Enter</kbd> for new line
      </p>
    </div>
  )
}
