import { useEffect, useRef, useState } from "react"
import { Check, CheckCheck } from "lucide-react"
import { MessageAttachment } from "./MessageAttachment"
import { MessageContextMenu } from "./MessageContextMenu"

function formatTime(date) {
  return new Date(date).toLocaleTimeString("en-US", {
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  }).toLowerCase()
}

/**
 * Message receipt status indicator component
 * Shows: single tick (sent), double tick (delivered), blue double tick (read)
 */
function ReceiptStatus({ status, isOwn }) {
  if (!isOwn) return null;
  
  // Determine which icon and color to use
  switch (status) {
    case 'read':
      // Double blue tick - message has been read
      return <CheckCheck className="w-4 h-4 ml-0.5 text-sky-400" />;
    case 'delivered':
      // Double gray tick - message has been delivered
      return <CheckCheck className="w-4 h-4 ml-0.5 opacity-70" />;
    case 'sent':
    default:
      // Single gray tick - message has been sent to server
      return <Check className="w-4 h-4 ml-0.5 opacity-70" />;
  }
}

export function GlassChatMessages({ messages = [], onMessageDeleted, onReply }) {
  const messagesEndRef = useRef(null);
  const containerRef = useRef(null);
  const [contextMenu, setContextMenu] = useState({ isOpen: false, message: null, position: { x: 0, y: 0 } });

  // Auto-scroll to bottom when messages change
  useEffect(() => {
    if (messagesEndRef.current) {
      messagesEndRef.current.scrollIntoView({ behavior: "smooth" });
    }
  }, [messages]);

  const handleContextMenu = (e, message) => {
    e.preventDefault();
    setContextMenu({
      isOpen: true,
      message,
      position: { x: e.clientX, y: e.clientY }
    });
  };

  const closeContextMenu = () => {
    setContextMenu({ isOpen: false, message: null, position: { x: 0, y: 0 } });
  };

  return (
    <div 
      ref={containerRef}
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
              onContextMenu={(e) => handleContextMenu(e, message)}
              className={`relative max-w-[65%] cursor-pointer select-none ${
                message.isOwn
                  ? "bg-primary text-primary-foreground rounded-lg rounded-tr-sm"
                  : "bg-card text-card-foreground rounded-lg rounded-tl-sm"
              } hover:brightness-95 transition-all`}
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
                      <ReceiptStatus status={message.receiptStatus} isOwn={message.isOwn} />
                    </span>
                  </div>
                </div>
              )}
              
              {/* Timestamp for attachment-only messages */}
              {!message.content && message.attachment && (
                <div className="px-3 pb-2 text-[10px] opacity-70 flex items-center gap-0.5">
                  {formatTime(message.timestamp)}
                  <ReceiptStatus status={message.receiptStatus} isOwn={message.isOwn} />
                </div>
              )}
            </div>
          </div>
        )
      })}
      {/* Scroll anchor */}
      <div ref={messagesEndRef} />

      {/* Context Menu */}
      <MessageContextMenu
        isOpen={contextMenu.isOpen}
        message={contextMenu.message}
        position={contextMenu.position}
        onClose={closeContextMenu}
        onDelete={(msg) => onMessageDeleted?.(msg)}
        onReply={(msg) => onReply?.(msg)}
      />
    </div>
  )
}
