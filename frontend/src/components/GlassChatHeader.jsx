import { useState, useRef, useEffect } from "react"
import { Video, Phone, MoreVertical, Search, ChevronDown, ArrowLeft, Users } from "lucide-react"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"

/**
 * Call type dropdown component
 */
function CallDropdown({ isOpen, onClose, onVideoCall, onAudioCall, buttonRef }) {
  const dropdownRef = useRef(null);

  // Handle click outside to close
  useEffect(() => {
    const handleClickOutside = (e) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target) &&
        buttonRef?.current &&
        !buttonRef.current.contains(e.target)
      ) {
        onClose();
      }
    };

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
    }
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [isOpen, onClose, buttonRef]);

  if (!isOpen) return null;

  return (
    <div
      ref={dropdownRef}
      className="absolute right-0 top-full mt-1 py-1 min-w-[160px] bg-popover border border-border rounded-lg shadow-lg z-50"
    >
      <button
        onClick={() => {
          onVideoCall();
          onClose();
        }}
        className="w-full flex items-center gap-3 px-4 py-2.5 text-sm text-foreground hover:bg-secondary transition-colors"
      >
        <Video className="w-4 h-4 text-muted-foreground" />
        <span>Video call</span>
      </button>
      <button
        onClick={() => {
          onAudioCall();
          onClose();
        }}
        className="w-full flex items-center gap-3 px-4 py-2.5 text-sm text-foreground hover:bg-secondary transition-colors"
      >
        <Phone className="w-4 h-4 text-muted-foreground" />
        <span>Audio call</span>
      </button>
    </div>
  );
}

export function GlassChatHeader({ name, status: _status, avatar, isChannel, memberCount, onSearch, onBack, onMembersClick, onVideoCall, onAudioCall }) {
  const [showCallDropdown, setShowCallDropdown] = useState(false);
  const callDropdownButtonRef = useRef(null);

  return (
    <header className="flex items-center justify-between px-4 py-2 border-b border-border bg-card">
      <div className="flex items-center gap-3">
        {/* Back button for mobile */}
        {onBack && (
          <button
            onClick={onBack}
            className="p-2 -ml-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
            aria-label="Back to chats"
          >
            <ArrowLeft className="w-5 h-5" />
          </button>
        )}
        
        <button 
          onClick={isChannel ? onMembersClick : undefined}
          className="flex items-center gap-3 cursor-pointer hover:bg-secondary/50 rounded-lg p-1.5 -m-1.5 transition-colors text-left"
        >
          <Avatar className="w-10 h-10">
            <AvatarImage src={avatar || "/placeholder.svg"} alt={name} />
            <AvatarFallback className="bg-secondary text-secondary-foreground">
              {name?.split(" ").map(n => n[0]).join("").slice(0, 2).toUpperCase() || 'U'}
            </AvatarFallback>
          </Avatar>
          
          <div className="min-w-0">
            <h2 className="font-medium text-foreground truncate">{name}</h2>
            <p className="text-xs text-muted-foreground">
              {isChannel
                ? `${memberCount || 0} members`
                : "tap for info"}
            </p>
          </div>
        </button>
      </div>

      <div className="flex items-center gap-1">
        {/* Members button for groups */}
        {isChannel && (
          <button
            onClick={onMembersClick}
            className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
            aria-label="View members"
            title="View members"
          >
            <Users className="w-5 h-5" />
          </button>
        )}
        
        {/* Call buttons with dropdown */}
        <div className="relative flex items-center">
          {/* Quick video call button */}
          <button
            onClick={onVideoCall}
            className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
            aria-label="Video call"
            title="Start video call"
          >
            <Video className="w-5 h-5" />
          </button>
          
          {/* Dropdown toggle */}
          <button
            ref={callDropdownButtonRef}
            onClick={() => setShowCallDropdown(!showCallDropdown)}
            className={`p-1 rounded-lg transition-colors ${
              showCallDropdown 
                ? 'bg-secondary text-foreground' 
                : 'text-muted-foreground hover:text-foreground hover:bg-secondary'
            }`}
            aria-label="Call options"
            aria-expanded={showCallDropdown}
          >
            <ChevronDown className={`w-4 h-4 transition-transform ${showCallDropdown ? 'rotate-180' : ''}`} />
          </button>
          
          {/* Call type dropdown */}
          <CallDropdown
            isOpen={showCallDropdown}
            onClose={() => setShowCallDropdown(false)}
            onVideoCall={onVideoCall}
            onAudioCall={onAudioCall || onVideoCall}
            buttonRef={callDropdownButtonRef}
          />
        </div>
        
        <button
          onClick={onSearch}
          className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
          aria-label="Search in conversation"
        >
          <Search className="w-5 h-5" />
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
