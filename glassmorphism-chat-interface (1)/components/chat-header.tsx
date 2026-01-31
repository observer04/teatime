"use client"

import { Video, MoreVertical, Search, ChevronDown } from "lucide-react"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

interface ChatHeaderProps {
  name: string
  status?: string
  avatar?: string
  isChannel?: boolean
  memberCount?: number
}

export function ChatHeader({ name, status, avatar, isChannel, memberCount }: ChatHeaderProps) {
  return (
    <header className="flex items-center justify-between px-4 py-2 border-b border-border bg-card">
      <div className="flex items-center gap-3 cursor-pointer hover:bg-secondary/50 rounded-lg p-1.5 -m-1.5 transition-colors">
        <Avatar className="w-10 h-10">
          <AvatarImage src={avatar || "/placeholder.svg"} alt={name} />
          <AvatarFallback className="bg-secondary text-secondary-foreground">
            {name.split(" ").map(n => n[0]).join("").slice(0, 2)}
          </AvatarFallback>
        </Avatar>
        
        <div className="min-w-0">
          <h2 className="font-medium text-foreground truncate">{name}</h2>
          <p className="text-xs text-muted-foreground">
            {isChannel
              ? `${memberCount || 0} members`
              : status === "online"
                ? "online"
                : "last seen recently"}
          </p>
        </div>
      </div>

      <div className="flex items-center gap-1">
        {/* Video Call with dropdown */}
        <div className="flex items-center">
          <button
            className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
            aria-label="Video call"
          >
            <Video className="w-5 h-5" />
          </button>
          <button
            className="p-1 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
            aria-label="Video options"
          >
            <ChevronDown className="w-4 h-4" />
          </button>
        </div>
        
        <button
          className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
          aria-label="Search in conversation"
        >
          <Search className="w-5 h-5" />
        </button>
        
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
              aria-label="More options"
            >
              <MoreVertical className="w-5 h-5" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48 bg-card border-border">
            <DropdownMenuItem className="cursor-pointer focus:bg-secondary">
              Contact info
            </DropdownMenuItem>
            <DropdownMenuItem className="cursor-pointer focus:bg-secondary">
              Select messages
            </DropdownMenuItem>
            <DropdownMenuItem className="cursor-pointer focus:bg-secondary">
              Close chat
            </DropdownMenuItem>
            <DropdownMenuItem className="cursor-pointer focus:bg-secondary">
              Mute notifications
            </DropdownMenuItem>
            <DropdownMenuItem className="cursor-pointer focus:bg-secondary">
              Disappearing messages
            </DropdownMenuItem>
            <DropdownMenuSeparator className="bg-border" />
            <DropdownMenuItem className="cursor-pointer focus:bg-secondary text-destructive">
              Block
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  )
}
