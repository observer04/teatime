import { useState } from "react"
import { MessageSquare, Search, Settings, Plus, Hash, Users, Bell, ChevronDown, LogOut } from "lucide-react"
import { cn } from "@/lib/utils"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"

export function ChatSidebar({ activeChat, onChatSelect, conversations = [], currentUser, onLogout }) {
  const [channelsExpanded, setChannelsExpanded] = useState(true)
  const [dmExpanded, setDmExpanded] = useState(true)
  const [searchQuery, setSearchQuery] = useState("")

  // Split conversations into groups and DMs
  const channels = conversations.filter(c => c.type === 'group')
  const directMessages = conversations.filter(c => c.type === 'direct')

  // Filter based on search
  const filteredChannels = channels.filter(c => 
    c.title?.toLowerCase().includes(searchQuery.toLowerCase())
  )
  const filteredDMs = directMessages.filter(c => 
    c.other_user?.username?.toLowerCase().includes(searchQuery.toLowerCase())
  )

  const getInitials = (name) => {
    if (!name) return '?'
    return name.split(" ").map(n => n[0]).join("").toUpperCase()
  }

  return (
    <aside className="w-72 h-full flex flex-col bg-sidebar/70 backdrop-blur-xl border-r border-sidebar-border">
      {/* Header */}
      <div className="p-4 border-b border-sidebar-border">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-primary flex items-center justify-center">
              <MessageSquare className="w-4 h-4 text-primary-foreground" />
            </div>
            <span className="font-semibold text-sidebar-foreground">TeaTime</span>
          </div>
          <button 
            onClick={onLogout}
            className="p-2 rounded-lg hover:bg-sidebar-accent transition-colors text-muted-foreground hover:text-destructive" 
            aria-label="Logout"
            title="Logout"
          >
            <LogOut className="w-4 h-4" />
          </button>
        </div>
        
        {/* Search */}
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <input
            type="text"
            placeholder="Search conversations..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-input rounded-lg text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring text-foreground"
          />
        </div>
      </div>

      {/* Conversations */}
      <div className="flex-1 overflow-y-auto py-2">
        {/* Channels/Groups */}
        {channels.length > 0 && (
          <div className="px-3">
            <button
              onClick={() => setChannelsExpanded(!channelsExpanded)}
              className="flex items-center justify-between w-full px-2 py-2 text-xs font-medium text-muted-foreground uppercase tracking-wider hover:text-foreground transition-colors"
            >
              <span>Groups</span>
              <ChevronDown className={cn("w-4 h-4 transition-transform", !channelsExpanded && "-rotate-90")} />
            </button>
            
            {channelsExpanded && (
              <div className="space-y-0.5">
                {filteredChannels.map((channel) => (
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
                    <div className="flex items-center gap-2 min-w-0">
                      <Hash className="w-4 h-4 flex-shrink-0" />
                      <span className="truncate">{channel.title || 'Untitled Group'}</span>
                    </div>
                    {channel.unread_count > 0 && (
                      <span className="px-2 py-0.5 text-xs font-medium bg-primary text-primary-foreground rounded-full flex-shrink-0">
                        {channel.unread_count}
                      </span>
                    )}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Direct Messages */}
        {directMessages.length > 0 && (
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
                {filteredDMs.map((dm) => (
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
                    <div className="flex items-center gap-2 min-w-0">
                      <div className="relative flex-shrink-0">
                        <Avatar className="w-6 h-6">
                          <AvatarImage src={dm.other_user?.avatar_url} alt={dm.other_user?.username} />
                          <AvatarFallback className="text-xs bg-secondary text-secondary-foreground">
                            {getInitials(dm.other_user?.username)}
                          </AvatarFallback>
                        </Avatar>
                        <span className="absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full border-2 border-sidebar bg-primary" />
                      </div>
                      <span className="truncate">{dm.other_user?.username || 'Unknown User'}</span>
                    </div>
                    {dm.unread_count > 0 && (
                      <span className="px-2 py-0.5 text-xs font-medium bg-primary text-primary-foreground rounded-full flex-shrink-0">
                        {dm.unread_count}
                      </span>
                    )}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}

        {conversations.length === 0 && (
          <div className="px-4 py-8 text-center">
            <p className="text-muted-foreground text-sm">No conversations yet</p>
            <p className="text-muted-foreground text-xs mt-1">Start chatting to see conversations here</p>
          </div>
        )}
      </div>

      {/* User Profile */}
      <div className="p-3 border-t border-sidebar-border">
        <div className="flex items-center gap-3 p-2 rounded-lg hover:bg-sidebar-accent transition-colors cursor-pointer">
          <div className="relative">
            <Avatar className="w-9 h-9">
              <AvatarImage src={currentUser?.avatar_url} alt={currentUser?.username} />
              <AvatarFallback className="bg-primary text-primary-foreground text-sm">
                {getInitials(currentUser?.username)}
              </AvatarFallback>
            </Avatar>
            <span className="absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full bg-primary border-2 border-sidebar" />
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-sidebar-foreground truncate">{currentUser?.username || 'User'}</p>
            <p className="text-xs text-muted-foreground">Online</p>
          </div>
          <Settings className="w-4 h-4 text-muted-foreground flex-shrink-0" />
        </div>
      </div>
    </aside>
  )
}
