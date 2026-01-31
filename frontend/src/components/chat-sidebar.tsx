"use client"

import { useState } from "react"
import { MessageSquare, Search, Settings, Plus, Hash, Users, Bell, ChevronDown, Star } from "lucide-react"
import { cn } from "@/lib/utils"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"

interface ChatSidebarProps {
  activeChat: string
  onChatSelect: (chatId: string) => void
}

const channels = [
  { id: "general", name: "General", unread: 3 },
  { id: "design", name: "Design Team", unread: 0 },
  { id: "development", name: "Development", unread: 12 },
  { id: "marketing", name: "Marketing", unread: 0 },
]

const directMessages = [
  { id: "sarah", name: "Sarah Chen", avatar: "/avatars/sarah.jpg", status: "online", unread: 2 },
  { id: "alex", name: "Alex Rivera", avatar: "/avatars/alex.jpg", status: "online", unread: 0 },
  { id: "marcus", name: "Marcus Johnson", avatar: "/avatars/marcus.jpg", status: "offline", unread: 0 },
  { id: "emma", name: "Emma Wilson", avatar: "/avatars/emma.jpg", status: "away", unread: 1 },
]

export function ChatSidebar({ activeChat, onChatSelect }: ChatSidebarProps) {
  const [channelsExpanded, setChannelsExpanded] = useState(true)
  const [dmExpanded, setDmExpanded] = useState(true)

  return (
    <aside className="w-72 h-full flex flex-col bg-sidebar/70 backdrop-blur-xl border-r border-sidebar-border">
      {/* Header */}
      <div className="p-4 border-b border-sidebar-border">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-primary flex items-center justify-center">
              <MessageSquare className="w-4 h-4 text-primary-foreground" />
            </div>
            <span className="font-semibold text-sidebar-foreground">Messenger</span>
          </div>
          <button className="p-2 rounded-lg hover:bg-sidebar-accent transition-colors" aria-label="Notifications">
            <Bell className="w-4 h-4 text-muted-foreground" />
          </button>
        </div>
        
        {/* Search */}
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <input
            type="text"
            placeholder="Search conversations..."
            className="w-full pl-9 pr-4 py-2 bg-input rounded-lg text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring text-foreground"
          />
        </div>
      </div>

      {/* Channels */}
      <div className="flex-1 overflow-y-auto py-2">
        <div className="px-3">
          <button
            onClick={() => setChannelsExpanded(!channelsExpanded)}
            className="flex items-center justify-between w-full px-2 py-2 text-xs font-medium text-muted-foreground uppercase tracking-wider hover:text-foreground transition-colors"
          >
            <span>Channels</span>
            <ChevronDown className={cn("w-4 h-4 transition-transform", !channelsExpanded && "-rotate-90")} />
          </button>
          
          {channelsExpanded && (
            <div className="space-y-0.5">
              {channels.map((channel) => (
                <button
                  key={channel.id}
                  onClick={() => onChatSelect(channel.id)}
                  className={cn(
                    "flex items-center justify-between w-full px-2 py-2 rounded-lg text-sm transition-colors",
                    activeChat === channel.id
                      ? "bg-primary/20 text-primary"
                      : "text-sidebar-foreground hover:bg-sidebar-accent"
                  )}
                >
                  <div className="flex items-center gap-2">
                    <Hash className="w-4 h-4" />
                    <span>{channel.name}</span>
                  </div>
                  {channel.unread > 0 && (
                    <span className="px-2 py-0.5 text-xs font-medium bg-primary text-primary-foreground rounded-full">
                      {channel.unread}
                    </span>
                  )}
                </button>
              ))}
              <button className="flex items-center gap-2 w-full px-2 py-2 rounded-lg text-sm text-muted-foreground hover:text-foreground hover:bg-sidebar-accent transition-colors">
                <Plus className="w-4 h-4" />
                <span>Add Channel</span>
              </button>
            </div>
          )}
        </div>

        {/* Direct Messages */}
        <div className="px-3 mt-4">
          <button
            onClick={() => setDmExpanded(!dmExpanded)}
            className="flex items-center justify-between w-full px-2 py-2 text-xs font-medium text-muted-foreground uppercase tracking-wider hover:text-foreground transition-colors"
          >
            <span>Direct Messages</span>
            <ChevronDown className={cn("w-4 h-4 transition-transform", !dmExpanded && "-rotate-90")} />
          </button>
          
          {dmExpanded && (
            <div className="space-y-0.5">
              {directMessages.map((dm) => (
                <button
                  key={dm.id}
                  onClick={() => onChatSelect(dm.id)}
                  className={cn(
                    "flex items-center justify-between w-full px-2 py-2 rounded-lg text-sm transition-colors",
                    activeChat === dm.id
                      ? "bg-primary/20 text-primary"
                      : "text-sidebar-foreground hover:bg-sidebar-accent"
                  )}
                >
                  <div className="flex items-center gap-2">
                    <div className="relative">
                      <Avatar className="w-6 h-6">
                        <AvatarImage src={dm.avatar || "/placeholder.svg"} alt={dm.name} />
                        <AvatarFallback className="text-xs bg-secondary text-secondary-foreground">
                          {dm.name.split(" ").map(n => n[0]).join("")}
                        </AvatarFallback>
                      </Avatar>
                      <span
                        className={cn(
                          "absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full border-2 border-sidebar",
                          dm.status === "online" && "bg-primary",
                          dm.status === "away" && "bg-yellow-500",
                          dm.status === "offline" && "bg-muted-foreground"
                        )}
                      />
                    </div>
                    <span>{dm.name}</span>
                  </div>
                  {dm.unread > 0 && (
                    <span className="px-2 py-0.5 text-xs font-medium bg-primary text-primary-foreground rounded-full">
                      {dm.unread}
                    </span>
                  )}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* User Profile */}
      <div className="p-3 border-t border-sidebar-border">
        <div className="flex items-center gap-3 p-2 rounded-lg hover:bg-sidebar-accent transition-colors cursor-pointer">
          <div className="relative">
            <Avatar className="w-9 h-9">
              <AvatarImage src="/avatars/user.jpg" alt="You" />
              <AvatarFallback className="bg-primary text-primary-foreground text-sm">JD</AvatarFallback>
            </Avatar>
            <span className="absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full bg-primary border-2 border-sidebar" />
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-sidebar-foreground truncate">John Doe</p>
            <p className="text-xs text-muted-foreground">Online</p>
          </div>
          <Settings className="w-4 h-4 text-muted-foreground" />
        </div>
      </div>
    </aside>
  )
}
