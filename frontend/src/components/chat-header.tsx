
import { Phone, Video, MoreVertical, Star, Search, Pin } from "lucide-react"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"

interface ChatHeaderProps {
  name: string
  status?: string
  avatar?: string
  isChannel?: boolean
  memberCount?: number
}

export function ChatHeader({ name, status, avatar, isChannel, memberCount }: ChatHeaderProps) {
  return (
    <header className="flex items-center justify-between px-6 py-4 border-b border-border bg-card/50 backdrop-blur-sm">
      <div className="flex items-center gap-3">
        {!isChannel ? (
          <div className="relative">
            <Avatar className="w-10 h-10">
              <AvatarImage src={avatar || "/placeholder.svg"} alt={name} />
              <AvatarFallback className="bg-secondary text-secondary-foreground">
                {name.split(" ").map(n => n[0]).join("")}
              </AvatarFallback>
            </Avatar>
            {status === "online" && (
              <span className="absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full bg-primary border-2 border-card" />
            )}
          </div>
        ) : (
          <div className="w-10 h-10 rounded-lg bg-primary/20 flex items-center justify-center">
            <span className="text-primary font-semibold">#</span>
          </div>
        )}
        
        <div>
          <h2 className="font-semibold text-foreground">{isChannel ? `#${name}` : name}</h2>
          <p className="text-xs text-muted-foreground">
            {isChannel
              ? `${memberCount || 0} members`
              : status === "online"
                ? "Active now"
                : "Offline"}
          </p>
        </div>
      </div>

      <div className="flex items-center gap-1">
        <button
          className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
          aria-label="Search in conversation"
        >
          <Search className="w-5 h-5" />
        </button>
        <button
          className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
          aria-label="Pin conversation"
        >
          <Pin className="w-5 h-5" />
        </button>
        <button
          className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
          aria-label="Voice call"
        >
          <Phone className="w-5 h-5" />
        </button>
        <button
          className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
          aria-label="Video call"
        >
          <Video className="w-5 h-5" />
        </button>
        <button
          className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
          aria-label="Star conversation"
        >
          <Star className="w-5 h-5" />
        </button>
        <button
          className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
          aria-label="More options"
        >
          <MoreVertical className="w-5 h-5" />
        </button>
      </div>
    </header>
  )
}
