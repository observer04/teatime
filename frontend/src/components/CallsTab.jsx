import { useState, useEffect } from 'react';
import { Phone, PhoneIncoming, PhoneOutgoing, PhoneMissed, Video, Clock, User } from 'lucide-react';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import api from '../services/api';

/**
 * Format call duration in human-readable format
 */
function formatDuration(seconds) {
  if (!seconds || seconds === 0) return '0s';
  
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;
  
  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  if (minutes > 0) {
    return `${minutes}m ${secs}s`;
  }
  return `${secs}s`;
}

/**
 * Format date for display
 */
function formatDate(dateStr) {
  const date = new Date(dateStr);
  const now = new Date();
  const diffDays = Math.floor((now - date) / (1000 * 60 * 60 * 24));
  
  if (diffDays === 0) {
    return date.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', hour12: true });
  }
  if (diffDays === 1) {
    return 'Yesterday';
  }
  if (diffDays < 7) {
    return date.toLocaleDateString('en-US', { weekday: 'short' });
  }
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

/**
 * Get call status icon and color
 */
function getCallStatusInfo(call, currentUserId) {
  const isOutgoing = call.initiator_id === currentUserId;
  
  switch (call.status) {
    case 'missed':
      return {
        icon: PhoneMissed,
        color: 'text-red-500',
        label: 'Missed'
      };
    case 'declined':
      return {
        icon: PhoneMissed,
        color: 'text-orange-500',
        label: 'Declined'
      };
    case 'cancelled':
      return {
        icon: isOutgoing ? PhoneOutgoing : PhoneIncoming,
        color: 'text-muted-foreground',
        label: 'Cancelled'
      };
    case 'ended':
      return {
        icon: isOutgoing ? PhoneOutgoing : PhoneIncoming,
        color: isOutgoing ? 'text-green-500' : 'text-blue-500',
        label: formatDuration(call.duration_seconds)
      };
    case 'active':
      return {
        icon: Phone,
        color: 'text-green-500 animate-pulse',
        label: 'Ongoing'
      };
    case 'ringing':
      return {
        icon: Phone,
        color: 'text-yellow-500 animate-pulse',
        label: 'Ringing'
      };
    default:
      return {
        icon: Phone,
        color: 'text-muted-foreground',
        label: call.status
      };
  }
}

/**
 * Single call history item
 */
function CallHistoryItem({ call, currentUserId, onCallBack }) {
  const isOutgoing = call.initiator_id === currentUserId;
  const statusInfo = getCallStatusInfo(call, currentUserId);
  const StatusIcon = statusInfo.icon;
  
  // For DMs, show the other user; for groups, show group info
  const displayName = call.conversation_type === 'dm' 
    ? (call.other_user?.username || call.initiator_username)
    : (call.conversation_title || 'Group Call');
  
  const avatarUrl = call.conversation_type === 'dm'
    ? call.other_user?.avatar_url
    : null;

  return (
    <div className="flex items-center gap-3 px-4 py-3 hover:bg-secondary/50 cursor-pointer transition-colors">
      {/* Avatar */}
      <div className="relative">
        <Avatar className="w-12 h-12">
          <AvatarImage src={avatarUrl} alt={displayName} />
          <AvatarFallback className="bg-primary/10 text-primary">
            {displayName?.slice(0, 2).toUpperCase() || 'U'}
          </AvatarFallback>
        </Avatar>
        {/* Call type indicator */}
        <div className={`absolute -bottom-1 -right-1 w-5 h-5 rounded-full bg-card border-2 border-background flex items-center justify-center ${statusInfo.color}`}>
          {call.call_type === 'video' ? (
            <Video className="w-3 h-3" />
          ) : (
            <Phone className="w-3 h-3" />
          )}
        </div>
      </div>
      
      {/* Call info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-medium text-foreground truncate">{displayName}</span>
          {call.conversation_type === 'group' && (
            <span className="text-xs text-muted-foreground bg-secondary px-1.5 py-0.5 rounded">Group</span>
          )}
        </div>
        <div className="flex items-center gap-1.5 text-sm">
          <StatusIcon className={`w-4 h-4 ${statusInfo.color}`} />
          <span className={call.status === 'missed' ? 'text-red-500' : 'text-muted-foreground'}>
            {isOutgoing ? 'Outgoing' : 'Incoming'}
          </span>
          {call.status === 'ended' && call.duration_seconds > 0 && (
            <>
              <span className="text-muted-foreground">•</span>
              <Clock className="w-3 h-3 text-muted-foreground" />
              <span className="text-muted-foreground">{statusInfo.label}</span>
            </>
          )}
        </div>
      </div>
      
      {/* Time and callback */}
      <div className="flex flex-col items-end gap-1">
        <span className="text-xs text-muted-foreground">{formatDate(call.created_at)}</span>
        <button
          onClick={(e) => {
            e.stopPropagation();
            onCallBack?.(call);
          }}
          className="p-2 rounded-full hover:bg-primary/10 text-primary transition-colors"
          title={call.call_type === 'video' ? 'Video call' : 'Audio call'}
        >
          {call.call_type === 'video' ? (
            <Video className="w-5 h-5" />
          ) : (
            <Phone className="w-5 h-5" />
          )}
        </button>
      </div>
    </div>
  );
}

/**
 * Calls tab content showing call history
 */
export function CallsTab({ currentUserId, onStartCall }) {
  const [calls, setCalls] = useState([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('all'); // 'all', 'missed', 'outgoing', 'incoming'

  useEffect(() => {
    loadCallHistory();
  }, []);

  const loadCallHistory = async () => {
    try {
      setLoading(true);
      const data = await api.getCallHistory(100);
      setCalls(data.calls || []);
    } catch (error) {
      console.error('Failed to load call history:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCallBack = (call) => {
    // Start a new call to the same conversation
    onStartCall?.(call.conversation_id, call.call_type);
  };

  // Filter calls
  const filteredCalls = calls.filter(call => {
    if (filter === 'all') return true;
    if (filter === 'missed') return call.status === 'missed';
    if (filter === 'outgoing') return call.initiator_id === currentUserId;
    if (filter === 'incoming') return call.initiator_id !== currentUserId;
    return true;
  });

  // Group calls by date
  const groupedCalls = filteredCalls.reduce((groups, call) => {
    const date = new Date(call.created_at).toDateString();
    if (!groups[date]) {
      groups[date] = [];
    }
    groups[date].push(call);
    return groups;
  }, {});

  const formatGroupDate = (dateStr) => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffDays = Math.floor((now - date) / (1000 * 60 * 60 * 24));
    
    if (diffDays === 0) return 'Today';
    if (diffDays === 1) return 'Yesterday';
    return date.toLocaleDateString('en-US', { weekday: 'long', month: 'short', day: 'numeric' });
  };

  if (loading) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col bg-card">
      {/* Header */}
      <div className="px-4 py-3 border-b border-border">
        <h2 className="text-lg font-semibold text-foreground mb-3">Calls</h2>
        
        {/* Filter tabs */}
        <div className="flex gap-2">
          {[
            { id: 'all', label: 'All' },
            { id: 'missed', label: 'Missed' },
            { id: 'incoming', label: 'Incoming' },
            { id: 'outgoing', label: 'Outgoing' },
          ].map(tab => (
            <button
              key={tab.id}
              onClick={() => setFilter(tab.id)}
              className={`px-3 py-1.5 text-sm rounded-full transition-colors ${
                filter === tab.id
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-secondary text-muted-foreground hover:text-foreground'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {/* Call list */}
      <div className="flex-1 overflow-y-auto">
        {filteredCalls.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-center p-8">
            <Phone className="w-16 h-16 text-muted-foreground/30 mb-4" />
            <h3 className="text-lg font-medium text-foreground mb-1">No calls yet</h3>
            <p className="text-sm text-muted-foreground">
              {filter === 'all' 
                ? 'Start a call from any chat to see it here'
                : `No ${filter} calls found`}
            </p>
          </div>
        ) : (
          Object.entries(groupedCalls).map(([date, dateCalls]) => (
            <div key={date}>
              <div className="px-4 py-2 bg-secondary/30 sticky top-0">
                <span className="text-xs font-medium text-muted-foreground uppercase">
                  {formatGroupDate(date)}
                </span>
              </div>
              {dateCalls.map(call => (
                <CallHistoryItem
                  key={call.id}
                  call={call}
                  currentUserId={currentUserId}
                  onCallBack={handleCallBack}
                />
              ))}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
