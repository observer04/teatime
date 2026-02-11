import { useState, useEffect, useCallback } from "react"
import { X, Search, UserPlus, Crown, Shield, Trash2, Loader2 } from "lucide-react"
import api from "../services/api"

export default function MembersPanel({ 
  isOpen, 
  onClose, 
  conversation, 
  currentUserId,
  onMemberAdded,
  onMemberRemoved 
}) {
  const [members, setMembers] = useState([])
  const [loading, setLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState("")
  const [searchResults, setSearchResults] = useState([])
  const [searching, setSearching] = useState(false)
  const [showAddMember, setShowAddMember] = useState(false)
  const [actionLoading, setActionLoading] = useState(null)
  const [error, setError] = useState("")

  // Get current user's role
  const currentUserMember = members.find(m => m.id === currentUserId)
  const isAdmin = currentUserMember?.role === 'admin'

  const loadMembers = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      // The conversation object should already have members from the API
      if (conversation.members) {
        setMembers(conversation.members)
      } else {
        // Fetch conversation details if members not included
        const data = await api.request(`/conversations/${conversation.id}`)
        setMembers(data.members || [])
      }
    } catch (err) {
      setError("Failed to load members")
      console.error("Load members error:", err)
    } finally {
      setLoading(false)
    }
  }, [conversation.id, conversation.members])

  const searchUsers = useCallback(async () => {
    setSearching(true)
    try {
      const data = await api.searchUsers(searchQuery)
      // Filter out users who are already members
      const memberIds = new Set(members.map(m => m.id))
      setSearchResults((data.users || []).filter(u => !memberIds.has(u.id)))
    } catch (err) {
      console.error("Search error:", err)
    } finally {
      setSearching(false)
    }
  }, [searchQuery, members])

  useEffect(() => {
    if (isOpen && conversation?.id) {
      loadMembers()
    }
  }, [isOpen, conversation?.id, loadMembers])

  useEffect(() => {
    if (searchQuery.length >= 2) {
      const timer = setTimeout(() => searchUsers(), 300)
      return () => clearTimeout(timer)
    } else {
      setSearchResults([])
    }
  }, [searchQuery, searchUsers])

  const handleAddMember = async (user) => {
    setActionLoading(user.id)
    setError("")
    try {
      await api.addMember(conversation.id, user.id)
      setMembers(prev => [...prev, { ...user, role: 'member' }])
      setSearchResults(prev => prev.filter(u => u.id !== user.id))
      setSearchQuery("")
      setShowAddMember(false)
      onMemberAdded?.(user)
    } catch (err) {
      setError(err.message || "Failed to add member")
    } finally {
      setActionLoading(null)
    }
  }

  const handleRemoveMember = async (member) => {
    if (!confirm(`Remove ${member.username} from the group?`)) return
    
    setActionLoading(member.id)
    setError("")
    try {
      await api.removeMember(conversation.id, member.id)
      setMembers(prev => prev.filter(m => m.id !== member.id))
      onMemberRemoved?.(member)
    } catch (err) {
      setError(err.message || "Failed to remove member")
    } finally {
      setActionLoading(null)
    }
  }

  const handleLeaveGroup = async () => {
    if (!confirm("Are you sure you want to leave this group?")) return
    
    setActionLoading(currentUserId)
    setError("")
    try {
      await api.removeMember(conversation.id, currentUserId)
      onMemberRemoved?.({ id: currentUserId, left: true })
      onClose()
    } catch (err) {
      setError(err.message || "Failed to leave group")
    } finally {
      setActionLoading(null)
    }
  }

  const getInitials = (username) => {
    return username?.slice(0, 2).toUpperCase() || '??'
  }

  const getRoleBadge = (role) => {
    if (role === 'admin') {
      return <Crown className="w-3.5 h-3.5 text-yellow-500" title="Admin" />
    }
    return null
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="bg-card border border-border rounded-xl w-full max-w-md mx-4 max-h-[80vh] flex flex-col shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-border">
          <div>
            <h2 className="text-lg font-semibold text-foreground">Group Members</h2>
            <p className="text-sm text-muted-foreground">{members.length} members</p>
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-accent rounded-lg transition-colors"
          >
            <X className="w-5 h-5 text-muted-foreground" />
          </button>
        </div>

        {/* Error */}
        {error && (
          <div className="mx-4 mt-4 p-3 bg-destructive/10 border border-destructive/20 rounded-lg text-destructive text-sm">
            {error}
          </div>
        )}

        {/* Add Member Section */}
        {isAdmin && (
          <div className="p-4 border-b border-border">
            {showAddMember ? (
              <div className="space-y-3">
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                  <input
                    type="text"
                    placeholder="Search users to add..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className="w-full pl-10 pr-4 py-2 bg-background border border-border rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/50"
                    autoFocus
                  />
                </div>

                {/* Search Results */}
                {searching ? (
                  <div className="flex items-center justify-center py-4">
                    <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
                  </div>
                ) : searchResults.length > 0 ? (
                  <div className="space-y-1 max-h-40 overflow-y-auto">
                    {searchResults.map(user => (
                      <button
                        key={user.id}
                        onClick={() => handleAddMember(user)}
                        disabled={actionLoading === user.id}
                        className="w-full flex items-center gap-3 p-2 hover:bg-accent rounded-lg transition-colors text-left"
                      >
                        <div className="w-8 h-8 rounded-full bg-primary/20 flex items-center justify-center text-xs font-medium text-primary">
                          {getInitials(user.username)}
                        </div>
                        <span className="flex-1 text-sm text-foreground">{user.username}</span>
                        {actionLoading === user.id ? (
                          <Loader2 className="w-4 h-4 animate-spin text-muted-foreground" />
                        ) : (
                          <UserPlus className="w-4 h-4 text-primary" />
                        )}
                      </button>
                    ))}
                  </div>
                ) : searchQuery.length >= 2 ? (
                  <p className="text-sm text-muted-foreground text-center py-2">No users found</p>
                ) : null}

                <button
                  onClick={() => {
                    setShowAddMember(false)
                    setSearchQuery("")
                    setSearchResults([])
                  }}
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Cancel
                </button>
              </div>
            ) : (
              <button
                onClick={() => setShowAddMember(true)}
                className="flex items-center gap-2 text-primary hover:text-primary/80 text-sm font-medium"
              >
                <UserPlus className="w-4 h-4" />
                Add Member
              </button>
            )}
          </div>
        )}

        {/* Members List */}
        <div className="flex-1 overflow-y-auto p-4">
          {loading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <div className="space-y-1">
              {members.map(member => (
                <div
                  key={member.id}
                  className="flex items-center gap-3 p-2 rounded-lg hover:bg-accent/50 transition-colors group"
                >
                  {/* Avatar */}
                  <div className="w-10 h-10 rounded-full bg-primary/20 flex items-center justify-center text-sm font-medium text-primary overflow-hidden">
                    {member.avatar_url ? (
                      <img 
                        src={member.avatar_url} 
                        alt={member.username}
                        className="w-full h-full object-cover"
                      />
                    ) : (
                      getInitials(member.username)
                    )}
                  </div>

                  {/* Info */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-1.5">
                      <span className="font-medium text-foreground truncate">
                        {member.username}
                        {member.id === currentUserId && (
                          <span className="text-muted-foreground font-normal"> (You)</span>
                        )}
                      </span>
                      {getRoleBadge(member.role)}
                    </div>
                    <span className="text-xs text-muted-foreground capitalize">
                      {member.role}
                    </span>
                  </div>

                  {/* Actions */}
                  {member.id !== currentUserId && isAdmin && (
                    <button
                      onClick={() => handleRemoveMember(member)}
                      disabled={actionLoading === member.id}
                      className="p-2 text-muted-foreground hover:text-destructive hover:bg-destructive/10 rounded-lg opacity-0 group-hover:opacity-100 transition-all"
                      title="Remove member"
                    >
                      {actionLoading === member.id ? (
                        <Loader2 className="w-4 h-4 animate-spin" />
                      ) : (
                        <Trash2 className="w-4 h-4" />
                      )}
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-border">
          <button
            onClick={handleLeaveGroup}
            disabled={actionLoading === currentUserId}
            className="w-full flex items-center justify-center gap-2 py-2 text-destructive hover:bg-destructive/10 rounded-lg transition-colors text-sm font-medium"
          >
            {actionLoading === currentUserId ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <>Leave Group</>
            )}
          </button>
        </div>
      </div>
    </div>
  )
}
