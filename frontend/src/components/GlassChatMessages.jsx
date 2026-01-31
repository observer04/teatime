import { CheckCheck } from "lucide-react"
import { MessageAttachment } from "./MessageAttachment"

function formatTime(date) {
  return new Date(date).toLocaleTimeString("en-US", {
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  }).toLowerCase()
}

export function GlassChatMessages({ messages = [] }) {
  return (
    <div 
      className="flex-1 overflow-y-auto p-4 space-y-1"
      style={{
        backgroundImage: `url("data:image/svg+xml,%3Csvg width='60' height='60' viewBox='0 0 60 60' xmlns='http://www.w3.org/2000/svg'%3E%3Cg fill='none' fillRule='evenodd'%3E%3Cg fill='%23ffffff' fillOpacity='0.03'%3E%3Cpath d='M36 34v-4h-2v4h-4v2h4v4h2v-4h4v-2h-4zm0-30V0h-2v4h-4v2h4v4h2V6h4V4h-4zM6 34v-4H4v4H0v2h4v4h2v-4h4v-2H6zM6 4V0H4v4H0v2h4v4h2V6h4V4H6z'/%3E%3C/g%3E%3C/g%3E%3C/svg%3E")`,
      }}
    >
      {messages.map((message, index) => {
        const prevMessage = messages[index - 1]
        const isSameSender = prevMessage?.sender?.id === message.sender?.id
        const timeDiff = prevMessage ? (new Date(message.timestamp).getTime() - new Date(prevMessage.timestamp).getTime()) / 1000 / 60 : 999
        const isGrouped = isSameSender && timeDiff < 2

        return (
          <div 
            key={message.id}
            className={`flex ${message.isOwn ? "justify-end" : "justify-start"} ${!isGrouped ? "mt-3" : ""}`}
          >
            <div
              className={`relative max-w-[65%] ${
                message.isOwn
                  ? "bg-primary text-primary-foreground rounded-lg rounded-tr-sm"
                  : "bg-card text-card-foreground rounded-lg rounded-tl-sm"
              }`}
            >
              {/* Message tail */}
              {!isGrouped && (
                <div 
                  className={`absolute top-0 w-3 h-3 overflow-hidden ${
                    message.isOwn ? "-right-1.5" : "-left-1.5"
                  }`}
                >
                  <div 
                    className={`w-4 h-4 transform rotate-45 ${
                      message.isOwn ? "bg-primary -translate-x-2" : "bg-card translate-x-2"
                    }`}
                  />
                </div>
              )}
              
              {/* Attachment */}
              {message.attachment && (
                <div className="px-3 pt-3">
                  <MessageAttachment
                    attachmentId={message.attachment.id}
                    mimeType={message.attachment.mime_type}
                    filename={message.attachment.filename}
                    sizeBytes={message.attachment.size_bytes}
                  />
                </div>
              )}
              
              {/* Text content */}
              {message.content && (
                <div className="px-3 py-1.5 text-sm">
                  <div className="flex items-end gap-2">
                    <span className="leading-relaxed">{message.content}</span>
                    <span className="flex items-center gap-0.5 text-[10px] opacity-70 flex-shrink-0 translate-y-0.5">
                      {formatTime(message.timestamp)}
                      {message.isOwn && (
                        <CheckCheck className={`w-4 h-4 ml-0.5 ${message.isRead !== false ? "text-sky-400" : ""}`} />
                      )}
                    </span>
                  </div>
                </div>
              )}
              
              {/* Timestamp for attachment-only messages */}
              {!message.content && message.attachment && (
                <div className="px-3 pb-2 text-[10px] opacity-70 flex items-center gap-0.5">
                  {formatTime(message.timestamp)}
                  {message.isOwn && (
                    <CheckCheck className={`w-4 h-4 ml-0.5 ${message.isRead !== false ? "text-sky-400" : ""}`} />
                  )}
                </div>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
