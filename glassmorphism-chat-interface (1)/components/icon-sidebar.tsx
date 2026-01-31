"use client"

import { MessageSquare, Phone, CircleDot, Users, Archive, Settings, User } from "lucide-react"
import { cn } from "@/lib/utils"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"

interface IconSidebarProps {
  activeTab: string
  onTabChange: (tab: string) => void
}

const navItems = [
  { id: "chats", icon: MessageSquare, label: "Chats", badge: 3 },
  { id: "calls", icon: Phone, label: "Calls", badge: 1 },
  { id: "status", icon: CircleDot, label: "Status" },
  { id: "channels", icon: Users, label: "Channels" },
  { id: "communities", icon: Users, label: "Communities" },
]

const bottomItems = [
  { id: "archived", icon: Archive, label: "Archived" },
  { id: "settings", icon: Settings, label: "Settings" },
]

export function IconSidebar({ activeTab, onTabChange }: IconSidebarProps) {
  return (
    <aside className="w-16 h-full flex flex-col items-center py-4 bg-card border-r border-border">
      {/* Navigation Items */}
      <nav className="flex flex-col items-center gap-2 flex-1">
        {navItems.map(({ id, icon: Icon, label, badge }) => (
          <button
            key={id}
            onClick={() => onTabChange(id)}
            className={cn(
              "relative w-11 h-11 flex items-center justify-center rounded-xl transition-colors",
              activeTab === id
                ? "bg-primary/20 text-primary"
                : "text-muted-foreground hover:bg-secondary hover:text-foreground"
            )}
            aria-label={label}
          >
            <Icon className="w-5 h-5" />
            {badge && badge > 0 && (
              <span className="absolute -top-0.5 -right-0.5 min-w-[18px] h-[18px] flex items-center justify-center px-1 text-[10px] font-medium bg-primary text-primary-foreground rounded-full">
                {badge}
              </span>
            )}
          </button>
        ))}
      </nav>

      {/* Bottom Items */}
      <div className="flex flex-col items-center gap-2 mt-auto">
        {bottomItems.map(({ id, icon: Icon, label }) => (
          <button
            key={id}
            onClick={() => onTabChange(id)}
            className={cn(
              "w-11 h-11 flex items-center justify-center rounded-xl transition-colors",
              activeTab === id
                ? "bg-primary/20 text-primary"
                : "text-muted-foreground hover:bg-secondary hover:text-foreground"
            )}
            aria-label={label}
          >
            <Icon className="w-5 h-5" />
          </button>
        ))}
        
        {/* User Avatar */}
        <div className="mt-2 pt-2 border-t border-border">
          <Avatar className="w-9 h-9 cursor-pointer hover:ring-2 hover:ring-primary transition-all">
            <AvatarImage src="/avatars/user.jpg" alt="You" />
            <AvatarFallback className="bg-primary text-primary-foreground text-xs">JD</AvatarFallback>
          </Avatar>
        </div>
      </div>
    </aside>
  )
}
