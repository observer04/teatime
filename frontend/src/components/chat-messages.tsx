
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { cn } from "@/lib/utils"

export interface Message {
  id: string
  sender: {
    id: string
    name: string
    avatar?: string
  }
  content: string
  timestamp: Date
  isOwn?: boolean
}

interface ChatMessagesProps {
  messages: Message[]
}

function formatTime(date: Date) {
  return date.toLocaleTimeString("en-US", {
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  })
}

function formatDateDivider(date: Date) {
  const today = new Date()
  const yesterday = new Date(today)
  yesterday.setDate(yesterday.getDate() - 1)

  if (date.toDateString() === today.toDateString()) {
    return "Today"
  } else if (date.toDateString() === yesterday.toDateString()) {
    return "Yesterday"
  }
  return date.toLocaleDateString("en-US", {
    weekday: "long",
    month: "long",
    day: "numeric",
  })
}

export function ChatMessages({ messages }: ChatMessagesProps) {
  let lastDate: string | null = null

  return (
    <div className="flex-1 overflow-y-auto p-4 space-y-4">
      {messages.map((message) => {
        const messageDate = message.timestamp.toDateString()
        const showDateDivider = messageDate !== lastDate
        lastDate = messageDate

        return (
          <div key={message.id}>
            {showDateDivider && (
              <div className="flex items-center gap-4 my-6">
                <div className="flex-1 h-px bg-border" />
                <span className="text-xs text-muted-foreground font-medium">
                  {formatDateDivider(message.timestamp)}
                </span>
                <div className="flex-1 h-px bg-border" />
              </div>
            )}
            
            <div
              className={cn(
                "flex gap-3",
                message.isOwn && "flex-row-reverse"
              )}
            >
              {!message.isOwn && (
                <Avatar className="w-8 h-8 mt-1">
                  <AvatarImage src={message.sender.avatar || "/placeholder.svg"} alt={message.sender.name} />
                  <AvatarFallback className="text-xs bg-secondary text-secondary-foreground">
                    {message.sender.name.split(" ").map(n => n[0]).join("")}
                  </AvatarFallback>
                </Avatar>
              )}
              
              <div
                className={cn(
                  "flex flex-col max-w-[70%]",
                  message.isOwn && "items-end"
                )}
              >
                {!message.isOwn && (
                  <span className="text-xs text-muted-foreground mb-1 ml-1">
                    {message.sender.name}
                  </span>
                )}
                <div
                  className={cn(
                    "px-4 py-2.5 rounded-2xl text-sm leading-relaxed",
                    message.isOwn
                      ? "bg-primary text-primary-foreground rounded-br-md"
                      : "bg-card text-card-foreground rounded-bl-md border border-border"
                  )}
                >
                  {message.content}
                </div>
                <span className="text-[10px] text-muted-foreground mt-1 mx-1">
                  {formatTime(message.timestamp)}
                </span>
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
