"use client"

import { useState } from "react"
import { Search, MoreVertical, SquarePlus, Users, Star, CheckSquare, BookOpen, Lock, LogOut, ImageIcon, Check, CheckCheck } from "lucide-react"
import { cn } from "@/lib/utils"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

interface ChatSidebarProps {
  activeChat: string
  onChatSelect: (chatId: string) => void
}

const filterTabs = ["All", "Unread", "Favourites", "Groups"]

const chats = [
  { 
    id: "vipul", 
    name: "Vipul Iiit Cse", 
    avatar: "/avatars/vipul.jpg", 
    lastMessage: "2 min me pauch rhe h", 
    time: "10:13 pm",
    unread: 0,
    isRead: true,
    isPinned: true
  },
  { 
    id: "self", 
    name: "Hello Self (You)", 
    avatar: "/avatars/self.jpg", 
    lastMessage: "Photo", 
    lastMessageType: "image",
    time: "15/01/25",
    unread: 0,
    isRead: true,
  },
  { 
    id: "friend1", 
    name: "F", 
    avatar: "/avatars/f.jpg", 
    lastMessage: "You: https://www.insta...", 
    time: "Yesterday",
    unread: 0,
    isRead: true,
    isPinned: true
  },
  { 
    id: "vipu2", 
    name: "Vipul Iiit Cse", 
    avatar: "/avatars/vipul2.jpg", 
    lastMessage: "Reacted to: \"Ek chili gobi ...\"", 
    time: "Yesterday",
    unread: 0,
    isRead: false,
  },
  { 
    id: "hari", 
    name: "Hari CSE IIIT", 
    avatar: "/avatars/hari.jpg", 
    lastMessage: "You reacted to: \"For Rotiw...\"", 
    time: "Yesterday",
    unread: 0,
    isRead: true,
  },
  { 
    id: "sarah", 
    name: "Sarah Chen", 
    avatar: "/avatars/sarah.jpg", 
    lastMessage: "Something clean and minimal...", 
    time: "10:35 pm",
    unread: 2,
    isRead: false,
  },
]

export function ChatSidebar({ activeChat, onChatSelect }: ChatSidebarProps) {
  const [activeFilter, setActiveFilter] = useState("All")
  const [searchQuery, setSearchQuery] = useState("")

  return (
    <aside className="w-80 h-full flex flex-col bg-card border-r border-border">
      {/* Header */}
      <div className="px-4 py-3 flex items-center justify-between">
        <h1 className="text-xl font-semibold text-foreground">Chats</h1>
        <div className="flex items-center gap-1">
          <button 
            className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
            aria-label="New chat"
          >
            <SquarePlus className="w-5 h-5" />
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
            <DropdownMenuContent align="end" className="w-52 bg-card border-border">
              <DropdownMenuItem className="gap-3 cursor-pointer focus:bg-primary/20 focus:text-primary">
                <Users className="w-4 h-4" />
                New group
              </DropdownMenuItem>
              <DropdownMenuItem className="gap-3 cursor-pointer focus:bg-secondary">
                <Star className="w-4 h-4" />
                Starred messages
              </DropdownMenuItem>
              <DropdownMenuItem className="gap-3 cursor-pointer focus:bg-secondary">
                <CheckSquare className="w-4 h-4" />
                Select chats
              </DropdownMenuItem>
              <DropdownMenuItem className="gap-3 cursor-pointer focus:bg-secondary">
                <BookOpen className="w-4 h-4" />
                Mark all as read
              </DropdownMenuItem>
              <DropdownMenuSeparator className="bg-border" />
              <DropdownMenuItem className="gap-3 cursor-pointer focus:bg-secondary">
                <Lock className="w-4 h-4" />
                App lock
              </DropdownMenuItem>
              <DropdownMenuItem className="gap-3 cursor-pointer focus:bg-secondary">
                <LogOut className="w-4 h-4" />
                Log out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
      
      {/* Search */}
      <div className="px-4 pb-3">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Ask Meta AI or Search"
            className="w-full pl-9 pr-4 py-2 bg-secondary rounded-lg text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary text-foreground"
          />
        </div>
      </div>

      {/* Filter Tabs */}
      <div className="px-4 pb-3 flex flex-wrap gap-2">
        {filterTabs.map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveFilter(tab)}
            className={cn(
              "px-3 py-1.5 rounded-full text-sm font-medium transition-colors",
              activeFilter === tab
                ? "bg-primary/20 text-primary"
                : "bg-secondary text-muted-foreground hover:text-foreground"
            )}
          >
            {tab}
          </button>
        ))}
      </div>

      {/* Chat List */}
      <div className="flex-1 overflow-y-auto">
        {chats.map((chat) => (
          <button
            key={chat.id}
            onClick={() => onChatSelect(chat.id)}
            className={cn(
              "w-full flex items-center gap-3 px-4 py-3 transition-colors text-left",
              activeChat === chat.id
                ? "bg-primary/10"
                : "hover:bg-secondary"
            )}
          >
            <Avatar className="w-12 h-12 flex-shrink-0">
              <AvatarImage src={chat.avatar || "/placeholder.svg"} alt={chat.name} />
              <AvatarFallback className="bg-secondary text-secondary-foreground">
                {chat.name.split(" ").map(n => n[0]).join("").slice(0, 2)}
              </AvatarFallback>
            </Avatar>
            
            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between">
                <span className="font-medium text-foreground truncate">{chat.name}</span>
                <span className={cn(
                  "text-xs flex-shrink-0",
                  chat.unread > 0 ? "text-primary" : "text-muted-foreground"
                )}>
                  {chat.time}
                </span>
              </div>
              <div className="flex items-center justify-between gap-2 mt-0.5">
                <div className="flex items-center gap-1 min-w-0">
                  {chat.isRead && (
                    <CheckCheck className="w-4 h-4 text-primary flex-shrink-0" />
                  )}
                  {chat.lastMessageType === "image" && (
                    <ImageIcon className="w-4 h-4 text-muted-foreground flex-shrink-0" />
                  )}
                  <span className="text-sm text-muted-foreground truncate">
                    {chat.lastMessage}
                  </span>
                </div>
                {chat.unread > 0 && (
                  <span className="min-w-[20px] h-5 flex items-center justify-center px-1.5 text-xs font-medium bg-primary text-primary-foreground rounded-full flex-shrink-0">
                    {chat.unread}
                  </span>
                )}
                {chat.isPinned && chat.unread === 0 && (
                  <div className="w-4 h-4 flex-shrink-0">
                    <svg viewBox="0 0 24 24" fill="currentColor" className="w-4 h-4 text-muted-foreground">
                      <path d="M16.5 3c-1.74 0-3.41.81-4.5 2.09C10.91 3.81 9.24 3 7.5 3 4.42 3 2 5.42 2 8.5c0 3.78 3.4 6.86 8.55 11.54L12 21.35l1.45-1.32C18.6 15.36 22 12.28 22 8.5 22 5.42 19.58 3 16.5 3zm-4.4 15.55l-.1.1-.1-.1C7.14 14.24 4 11.39 4 8.5 4 6.5 5.5 5 7.5 5c1.54 0 3.04.99 3.57 2.36h1.87C13.46 5.99 14.96 5 16.5 5c2 0 3.5 1.5 3.5 3.5 0 2.89-3.14 5.74-7.9 10.05z"/>
                    </svg>
                  </div>
                )}
              </div>
            </div>
          </button>
        ))}
      </div>
    </aside>
  )
}
