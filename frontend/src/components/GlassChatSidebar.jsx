import { useState } from "react"
import { Search, MoreVertical, SquarePlus, Users, Star, CheckSquare, BookOpen, LogOut, CheckCheck, ImageIcon, MessageSquarePlus } from "lucide-react"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"

export function GlassChatSidebar({ activeChat, onChatSelect, conversations = [], onLogout, onNewGroup, onNewChat, onOpenStarred, onMarkAllRead, isMobile = false }) {
  const [activeFilter, setActiveFilter] = useState("All")
  const [searchQuery, setSearchQuery] = useState("")
  const [showMenu, setShowMenu] = useState(false)

  const filterTabs = ["All", "Unread", "Favourites", "Groups"]

  const filteredConversations = conversations.filter(conv => {
    if (searchQuery) {
      const searchLower = searchQuery.toLowerCase()
      return (conv.title?.toLowerCase().includes(searchLower) ||
              conv.other_user?.username?.toLowerCase().includes(searchLower))
    }
    
    if (activeFilter === "Groups") {
      return conv.type === 'group'
    }
    if (activeFilter === "Unread") {
      return conv.unread_count > 0
    }
    if (activeFilter === "Favourites") {
      return conv.is_pinned
    }
    return true
  })

  const formatTime = (dateString) => {
    if (!dateString) return ''
    const date = new Date(dateString)
    const now = new Date()
    const diffMs = now - date
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
    
    if (diffDays === 0) {
      return date.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', hour12: true })
    } else if (diffDays === 1) {
      return 'Yesterday'
    } else if (diffDays < 7) {
      return date.toLocaleDateString('en-US', { weekday: 'short' })
    } else {
      return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
    }
  }

  return (
    <aside className={`${isMobile ? 'w-full' : 'w-80'} h-full flex flex-col bg-card border-r border-border`}>
      <div className="px-4 py-3 flex items-center justify-between">
        <h1 className="text-xl font-semibold text-foreground">Chats</h1>
        <div className="flex items-center gap-1">
          <button 
            onClick={onNewChat}
            className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
            aria-label="New chat"
            title="Start new chat"
          >
            <MessageSquarePlus className="w-5 h-5" />
          </button>
          <button 
            onClick={onNewGroup}
            className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
            aria-label="New group"
            title="Create group"
          >
            <SquarePlus className="w-5 h-5" />
          </button>
          
          <div className="relative">
            <button 
              onClick={() => setShowMenu(!showMenu)}
              className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
              aria-label="More options"
            >
              <MoreVertical className="w-5 h-5" />
            </button>
            
            {showMenu && (
              <div className="absolute right-0 mt-2 w-52 bg-card border border-border rounded-lg shadow-lg z-50">
                <button onClick={() => { onNewGroup(); setShowMenu(false); }} className="w-full flex items-center gap-3 px-4 py-2 hover:bg-secondary text-left">
                  <Users className="w-4 h-4" />
                  <span>New group</span>
                </button>
                <button onClick={() => { onOpenStarred?.(); setShowMenu(false); }} className="w-full flex items-center gap-3 px-4 py-2 hover:bg-secondary text-left">
                  <Star className="w-4 h-4" />
                  <span>Starred messages</span>
                </button>
                <button className="w-full flex items-center gap-3 px-4 py-2 hover:bg-secondary text-left">
                  <CheckSquare className="w-4 h-4" />
                  <span>Select chats</span>
                </button>
                <button onClick={() => { onMarkAllRead?.(); setShowMenu(false); }} className="w-full flex items-center gap-3 px-4 py-2 hover:bg-secondary text-left">
                  <BookOpen className="w-4 h-4" />
                  <span>Mark all as read</span>
                </button>
                <div className="border-t border-border my-1"></div>
                <button onClick={() => { onLogout(); setShowMenu(false); }} className="w-full flex items-center gap-3 px-4 py-2 hover:bg-secondary text-left text-destructive">
                  <LogOut className="w-4 h-4" />
                  <span>Log out</span>
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
      
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

      <div className="px-4 pb-3 flex flex-wrap gap-2">
        {filterTabs.map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveFilter(tab)}
            className={`px-3 py-1.5 rounded-full text-sm font-medium transition-colors ${
              activeFilter === tab
                ? "bg-primary/20 text-primary"
                : "bg-secondary text-muted-foreground hover:text-foreground"
            }`}
          >
            {tab}
          </button>
        ))}
      </div>

      <div className="flex-1 overflow-y-auto">
        {filteredConversations.map((conv) => {
          const displayName = conv.type === 'group' 
            ? conv.title 
            : (conv.other_user?.username || 'Unknown')
          const avatar = conv.type === 'group' 
            ? conv.avatar_url 
            : conv.other_user?.avatar_url
          
          return (
            <button
              key={conv.id}
              onClick={() => onChatSelect(conv.id)}
              className={`w-full flex items-center gap-3 px-4 py-3 transition-colors text-left ${
                activeChat === conv.id ? "bg-primary/10" : "hover:bg-secondary"
              }`}
            >
              <Avatar className="w-12 h-12 flex-shrink-0">
                <AvatarImage src={avatar || "/placeholder.svg"} alt={displayName} />
                <AvatarFallback className="bg-secondary text-secondary-foreground">
                  {displayName.split(" ").map(n => n[0]).join("").slice(0, 2).toUpperCase()}
                </AvatarFallback>
              </Avatar>
              
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between">
                  <span className="font-medium text-foreground truncate">{displayName}</span>
                  <span className={`text-xs flex-shrink-0 ${
                    conv.unread_count > 0 ? "text-primary" : "text-muted-foreground"
                  }`}>
                    {formatTime(conv.last_message?.created_at || conv.updated_at)}
                  </span>
                </div>
                <div className="flex items-center justify-between gap-2 mt-0.5">
                  <div className="flex items-center gap-1 min-w-0">
                    {conv.last_message && (
                      <span className="text-sm text-muted-foreground truncate flex items-center gap-1">
                        {conv.last_message.attachment_id && !conv.last_message.body_text && (
                          <>
                            <ImageIcon className="w-3 h-3" />
                            <span>Photo</span>
                          </>
                        )}
                        {conv.last_message.body_text && conv.last_message.body_text}
                        {!conv.last_message.body_text && !conv.last_message.attachment_id && "Message"}
                      </span>
                    )}
                  </div>
                  {conv.unread_count > 0 && (
                    <span className="min-w-[20px] h-5 flex items-center justify-center px-1.5 text-xs font-medium bg-primary text-primary-foreground rounded-full flex-shrink-0">
                      {conv.unread_count}
                    </span>
                  )}
                  {conv.is_pinned && conv.unread_count === 0 && (
                    <div className="w-4 h-4 flex-shrink-0">
                      <svg viewBox="0 0 24 24" fill="currentColor" className="w-4 h-4 text-muted-foreground">
                        <path d="M16 9V4h1c.55 0 1-.45 1-1s-.45-1-1-1H7c-.55 0-1 .45-1 1s.45 1 1 1h1v5c0 1.66-1.34 3-3 3v2h5.97v7l1 1 1-1v-7H19v-2c-1.66 0-3-1.34-3-3z"/>
                      </svg>
                    </div>
                  )}
                </div>
              </div>
            </button>
          )
        })}
      </div>
    </aside>
  )
}
