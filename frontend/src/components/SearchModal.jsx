import { useState, useEffect, useCallback } from 'react';
import { X, Search } from 'lucide-react';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import api from '../services/api';

export function SearchModal({ isOpen, onClose, conversationId, onMessageClick }) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  // Reset state when modal closes
  useEffect(() => {
    if (!isOpen) {
      setQuery('');
      setResults([]);
      setError('');
    }
  }, [isOpen]);

  // Debounced search
  const searchMessages = useCallback(async (searchQuery) => {
    if (searchQuery.length < 2) {
      setResults([]);
      return;
    }

    setLoading(true);
    setError('');

    try {
      let data;
      if (conversationId) {
        // Search within specific conversation
        data = await api.searchMessages(conversationId, searchQuery);
      } else {
        // Global search
        data = await api.searchAllMessages(searchQuery);
      }
      setResults(data.messages || []);
    } catch (err) {
      setError(err.message || 'Search failed');
    } finally {
      setLoading(false);
    }
  }, [conversationId]);

  useEffect(() => {
    const debounce = setTimeout(() => searchMessages(query), 300);
    return () => clearTimeout(debounce);
  }, [query, searchMessages]);

  const formatDate = (dateString) => {
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    });
  };

  const highlightQuery = (text, searchQuery) => {
    if (!searchQuery || searchQuery.length < 2) return text;
    
    const regex = new RegExp(`(${searchQuery.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi');
    const parts = text.split(regex);
    
    return parts.map((part, i) => 
      regex.test(part) ? (
        <mark key={i} className="bg-primary/30 text-foreground rounded px-0.5">
          {part}
        </mark>
      ) : part
    );
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-card border border-border rounded-xl shadow-xl w-full max-w-lg mx-4 max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center gap-3 px-4 py-3 border-b border-border">
          <button
            onClick={onClose}
            className="p-1 rounded-lg hover:bg-secondary transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={conversationId ? "Search in this chat..." : "Search all messages..."}
              className="w-full pl-9 pr-4 py-2 bg-secondary rounded-lg text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary"
              autoFocus
            />
          </div>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto">
          {error && (
            <div className="text-center py-8 text-destructive">{error}</div>
          )}

          {loading && (
            <div className="text-center py-8 text-muted-foreground">
              Searching...
            </div>
          )}

          {!loading && query.length >= 2 && results.length === 0 && (
            <div className="text-center py-8 text-muted-foreground">
              <Search className="w-12 h-12 mx-auto mb-3 opacity-50" />
              <p>No messages found</p>
              <p className="text-sm mt-1">Try a different search term</p>
            </div>
          )}

          {!loading && query.length < 2 && (
            <div className="text-center py-8 text-muted-foreground">
              <Search className="w-12 h-12 mx-auto mb-3 opacity-50" />
              <p>Type at least 2 characters to search</p>
            </div>
          )}

          {!loading && results.length > 0 && (
            <div className="divide-y divide-border">
              {results.map(message => (
                <button
                  key={message.id}
                  onClick={() => onMessageClick?.(message)}
                  className="w-full flex items-start gap-3 p-4 hover:bg-secondary transition-colors text-left"
                >
                  <Avatar className="w-10 h-10 flex-shrink-0">
                    <AvatarImage src={message.sender?.avatar_url} alt={message.sender?.username} />
                    <AvatarFallback className="bg-secondary text-secondary-foreground">
                      {(message.sender?.username || 'U').slice(0, 2).toUpperCase()}
                    </AvatarFallback>
                  </Avatar>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-medium text-sm">
                        {message.sender?.username || 'Unknown'}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {formatDate(message.created_at)}
                      </span>
                    </div>
                    <p className="text-sm text-foreground mt-1 line-clamp-3">
                      {highlightQuery(message.body_text, query)}
                    </p>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Footer with result count */}
        {results.length > 0 && (
          <div className="px-4 py-2 border-t border-border text-xs text-muted-foreground text-center">
            {results.length} result{results.length !== 1 ? 's' : ''} found
          </div>
        )}
      </div>
    </div>
  );
}
