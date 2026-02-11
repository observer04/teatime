import { useState, useEffect } from 'react';
import { X, Star } from 'lucide-react';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import api from '../services/api';

export default function StarredMessagesModal({ isOpen, onClose, onMessageClick }) {
  const [messages, setMessages] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (isOpen) {
      loadStarredMessages();
    }
  }, [isOpen]);

  const loadStarredMessages = async () => {
    setLoading(true);
    setError('');
    try {
      const data = await api.getStarredMessages();
      setMessages(data.messages || []);
    } catch (err) {
      setError(err.message || 'Failed to load starred messages');
    } finally {
      setLoading(false);
    }
  };

  const handleUnstar = async (messageId, e) => {
    e.stopPropagation();
    try {
      await api.unstarMessage(messageId);
      setMessages(messages.filter(m => m.id !== messageId));
    } catch (err) {
      console.error('Failed to unstar message:', err);
    }
  };

  const formatDate = (dateString) => {
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    });
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-card border border-border rounded-xl shadow-xl w-full max-w-lg mx-4 max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-border">
          <div className="flex items-center gap-3">
            <button
              onClick={onClose}
              className="p-1 rounded-lg hover:bg-secondary transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
            <h2 className="text-lg font-semibold flex items-center gap-2">
              <Star className="w-5 h-5 text-yellow-500" />
              Starred Messages
            </h2>
          </div>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="text-muted-foreground">Loading...</div>
            </div>
          ) : error ? (
            <div className="text-center py-12 text-destructive">{error}</div>
          ) : messages.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              <Star className="w-12 h-12 mx-auto mb-3 opacity-50" />
              <p>No starred messages</p>
              <p className="text-sm mt-1">Long press on a message to star it</p>
            </div>
          ) : (
            <div className="divide-y divide-border">
              {messages.map(message => (
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
                    <p className="text-sm text-foreground mt-1 line-clamp-2">
                      {message.body_text}
                    </p>
                  </div>
                  <button
                    onClick={(e) => handleUnstar(message.id, e)}
                    className="p-1.5 text-yellow-500 hover:bg-yellow-500/10 rounded-lg transition-colors"
                    aria-label="Unstar message"
                  >
                    <Star className="w-4 h-4 fill-current" />
                  </button>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
