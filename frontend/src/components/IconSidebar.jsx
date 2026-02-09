import { MessageSquare, Phone, Archive, Star, Search } from "lucide-react"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"

export function IconSidebar({ activeTab, onTabChange, currentUser, onOpenStarred, onOpenSearch, onOpenProfile }) {
  const navItems = [
    { id: "chats", icon: MessageSquare, label: "Chats", badge: 0 },
    { id: "calls", icon: Phone, label: "Calls", badge: 0 },
  ]

  const actionItems = [
    { id: "search", icon: Search, label: "Search", action: onOpenSearch },
    { id: "starred", icon: Star, label: "Starred", action: onOpenStarred },
  ]

  const bottomItems = [
    { id: "archived", icon: Archive, label: "Archived" },
  ]

  return (
    <aside className="w-14 md:w-16 h-full flex flex-col items-center py-2 md:py-4 bg-card border-r border-border">
      <nav className="flex flex-col items-center gap-1 md:gap-2 flex-1">
        {navItems.map(({ id, icon: Icon, label, badge }) => (
          <button
            key={id}
            onClick={() => onTabChange(id)}
            className={`relative w-11 h-11 flex items-center justify-center rounded-xl transition-colors ${
              activeTab === id
                ? "bg-primary/20 text-primary"
                : "text-muted-foreground hover:bg-secondary hover:text-foreground"
            }`}
            aria-label={label}
          >
            <Icon className="w-5 h-5" />
            {badge > 0 && (
              <span className="absolute -top-0.5 -right-0.5 min-w-[18px] h-[18px] flex items-center justify-center px-1 text-[10px] font-medium bg-primary text-primary-foreground rounded-full">
                {badge}
              </span>
            )}
          </button>
        ))}

        {/* Separator */}
        <div className="w-8 h-px bg-border my-2" />

        {/* Action items (Search & Starred) */}
        {actionItems.map(({ id, icon: Icon, label, action }) => (
          <button
            key={id}
            onClick={action}
            className="w-11 h-11 flex items-center justify-center rounded-xl transition-colors text-muted-foreground hover:bg-secondary hover:text-foreground"
            aria-label={label}
          >
            <Icon className="w-5 h-5" />
          </button>
        ))}
      </nav>

      <div className="flex flex-col items-center gap-2 mt-auto">
        {bottomItems.map(({ id, icon: Icon, label }) => (
          <button
            key={id}
            onClick={() => onTabChange(id)}
            className={`w-11 h-11 flex items-center justify-center rounded-xl transition-colors ${
              activeTab === id
                ? "bg-primary/20 text-primary"
                : "text-muted-foreground hover:bg-secondary hover:text-foreground"
            }`}
            aria-label={label}
          >
            <Icon className="w-5 h-5" />
          </button>
        ))}
        
        <div className="mt-2 pt-2 border-t border-border">
          <button onClick={onOpenProfile} className="rounded-full" aria-label="Open Profile Settings">
            <Avatar className="w-9 h-9 cursor-pointer hover:ring-2 hover:ring-primary transition-all">
              <AvatarImage src={currentUser?.avatar_url} alt={currentUser?.username} />
              <AvatarFallback className="bg-primary text-primary-foreground text-xs">
                {currentUser?.username?.slice(0, 2).toUpperCase() || 'U'}
              </AvatarFallback>
            </Avatar>
          </button>
        </div>
      </div>
    </aside>
  )
}
