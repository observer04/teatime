import { useState, useEffect } from 'react';
import { X, Search, MessageCircle } from 'lucide-react';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import api from '../services/api';

export function NewChatModal({ isOpen, onClose, onChatStarted, currentUserId }) {
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState([]);
  const [loading, setLoading] = useState(false);
  const [startingChat, setStartingChat] = useState(null);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!isOpen) {
      setSearchQuery('');
      setSearchResults([]);
      setError('');
      setStartingChat(null);
    }
  }, [isOpen]);

  useEffect(() => {
    const searchUsers = async () => {
      if (searchQuery.length < 2) {
        setSearchResults([]);
        return;
      }

      setLoading(true);
      try {
        const data = await api.searchUsers(searchQuery);
        // Filter out current user
        setSearchResults((data.users || []).filter(u => u.id !== currentUserId));
      } catch (err) {
        console.error('Search failed:', err);
        setError('Failed to search users');
      } finally {
        setLoading(false);
      }
    };

    const debounce = setTimeout(searchUsers, 300);
    return () => clearTimeout(debounce);
  }, [searchQuery, currentUserId]);

  const startChat = async (user) => {
    setStartingChat(user.id);
    setError('');

    try {
      // Create a DM conversation with this user
      const conversation = await api.createConversation('dm', [user.id]);
      onChatStarted(conversation);
      onClose();
    } catch (err) {
      console.error('Failed to start chat:', err);
      setError(err.message || 'Failed to start chat');
    } finally {
      setStartingChat(null);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-card border border-border rounded-xl shadow-xl w-full max-w-md mx-4 max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-border">
          <div className="flex items-center gap-3">
            <button
              onClick={onClose}
              className="p-1 rounded-lg hover:bg-secondary transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
            <h2 className="text-lg font-semibold">New Chat</h2>
          </div>
        </div>

        {/* Search */}
        <div className="p-4 border-b border-border">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search by username"
              className="w-full pl-10 pr-4 py-2 bg-secondary rounded-lg text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary"
              autoFocus
            />
          </div>
        </div>

        {/* Error */}
        {error && (
          <div className="mx-4 mt-4 p-3 bg-destructive/10 text-destructive rounded-lg text-sm">
            {error}
          </div>
        )}

        {/* Results */}
        <div className="flex-1 overflow-y-auto">
          {loading && (
            <div className="flex items-center justify-center py-8">
              <div className="animate-spin w-6 h-6 border-2 border-primary border-t-transparent rounded-full" />
            </div>
          )}

          {!loading && searchQuery.length >= 2 && searchResults.length === 0 && (
            <div className="text-center py-8 text-muted-foreground">
              No users found
            </div>
          )}

          {!loading && searchQuery.length < 2 && (
            <div className="text-center py-8 text-muted-foreground">
              Type at least 2 characters to search
            </div>
          )}

          {searchResults.map(user => (
            <button
              key={user.id}
              onClick={() => startChat(user)}
              disabled={startingChat === user.id}
              className="w-full flex items-center gap-3 px-4 py-3 hover:bg-secondary transition-colors disabled:opacity-50"
            >
              <Avatar className="w-10 h-10">
                <AvatarImage src={user.avatar_url} alt={user.username} />
                <AvatarFallback className="bg-primary/10 text-primary">
                  {user.username.slice(0, 2).toUpperCase()}
                </AvatarFallback>
              </Avatar>
              <div className="flex-1 text-left">
                <div className="font-medium">{user.display_name || user.username}</div>
                <div className="text-sm text-muted-foreground">@{user.username}</div>
              </div>
              {startingChat === user.id ? (
                <div className="animate-spin w-5 h-5 border-2 border-primary border-t-transparent rounded-full" />
              ) : (
                <MessageCircle className="w-5 h-5 text-muted-foreground" />
              )}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
