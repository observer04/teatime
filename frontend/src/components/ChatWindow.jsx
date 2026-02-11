import { useState, useEffect, useRef } from 'react';

export default function ChatWindow({
  conversation,
  messages,
  user,
  onSendMessage,
  onStartTyping,
  onStopTyping
}) {
  const [inputValue, setInputValue] = useState(''); 
  const [typingTimeout, setTypingTimeout] = useState(null);
  const [showMembers, setShowMembers] = useState(false);
  const messagesEndRef = useRef(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!inputValue.trim() || !conversation) return;

    onSendMessage(conversation.id, inputValue.trim());
    setInputValue('');
    onStopTyping(conversation.id);
  };

  const handleInputChange = (e) => {
    setInputValue(e.target.value);

    if (!conversation) return;

    // Clear existing timeout
    if (typingTimeout) {
      clearTimeout(typingTimeout);
    }

    // Start typing indicator
    onStartTyping(conversation.id);

    // Stop typing after 2 seconds of inactivity
    const timeout = setTimeout(() => {
      onStopTyping(conversation.id);
    }, 2000);

    setTypingTimeout(timeout);
  };

  const formatTime = (date) => {
    return new Date(date).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  const getConversationName = () => {
    if (conversation.type === 'group') {
      return conversation.title || 'Group Chat';
    }
    if (!conversation.members) return 'Chat';
    const other = conversation.members.find(m => m.user_id !== user.id);
    if (other?.user) return other.user.username;
    if (other?.username) return other.username;
    return 'Chat';
  };

  const getConversationSubtitle = () => {
    if (conversation.type === 'group') {
      const count = conversation.members?.length || 0;
      return `${count} member${count !== 1 ? 's' : ''}`;
    }
    return 'Online'; // Could show actual online status later
  };

  if (!conversation) {
    return (
      <div className="flex-1 flex items-center justify-center bg-gray-50">
        <div className="text-center">
          <div className="text-6xl mb-4">🍵</div>
          <p className="text-gray-500 text-lg">Select a conversation to start chatting</p>
        </div>
      </div>
    );
  }

  const chatName = getConversationName();
  const isGroup = conversation.type === 'group';

  return (
    <div className="flex-1 flex flex-col bg-white">
      {/* Chat Header */}
      <div className="px-6 py-4 border-b border-gray-200 bg-white">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className={`w-10 h-10 rounded-full flex items-center justify-center text-white font-semibold ${
              isGroup ? 'bg-purple-500 text-lg' : 'bg-green-500'
            }`}>
              {isGroup ? '👥' : chatName.charAt(0).toUpperCase()}
            </div>
            <div>
              <h3 className="font-semibold text-gray-900">{chatName}</h3>
              <div className="text-sm text-gray-500">
                {getConversationSubtitle()}
              </div>
            </div>
          </div>
          
          {/* Group Actions */}
          {isGroup && (
            <button
              onClick={() => setShowMembers(!showMembers)}
              className={`p-2 rounded-lg transition-colors ${
                showMembers ? 'bg-purple-100 text-purple-600' : 'hover:bg-gray-100 text-gray-600'
              }`}
              title="View members"
            >
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197m13.5-9a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0z" />
              </svg>
            </button>
          )}
        </div>
      </div>

      <div className="flex-1 flex overflow-hidden">
        {/* Messages */}
        <div className="flex-1 flex flex-col">
          <div className="flex-1 overflow-y-auto p-6 space-y-4 bg-gray-50">
        {messages.length === 0 ? (
          <div className="text-center text-gray-500 mt-8">
            No messages yet. Start the conversation!
          </div>
        ) : (
          messages.map((msg, idx) => {
            const isSent = msg.sender_id === user.id;
            const showSender = !isSent && (idx === 0 || messages[idx - 1].sender_id !== msg.sender_id);

            return (
              <div
                key={msg.id || idx}
                className={`flex ${isSent ? 'justify-end' : 'justify-start'}`}
              >
                <div className={`${isSent ? 'items-end' : 'items-start'} max-w-[75%]`}>
                  {showSender && (
                    <div className="text-xs font-semibold text-green-600 mb-1 px-1">
                      {msg.sender?.username || msg.sender_username || 'Unknown'}
                    </div>
                  )}
                  <div
                    className={`message-bubble ${
                      isSent ? 'message-sent' : 'message-received'
                    }`}
                  >
                    <div className="whitespace-pre-wrap break-words">
                      {msg.body_text || msg.content}
                    </div>
                    <div className={`text-xs mt-1 ${isSent ? 'text-green-100' : 'text-gray-500'}`}>
                      {formatTime(msg.created_at)}
                    </div>
                  </div>
                </div>
              </div>
            );
          })
        )}
        <div ref={messagesEndRef} />
          </div>

          {/* Input */}
          <form onSubmit={handleSubmit} className="p-4 border-t border-gray-200 bg-white">
            <div className="flex gap-2">
              <input
                type="text"
                value={inputValue}
                onChange={handleInputChange}
                placeholder="Type a message..."
                className="flex-1 px-4 py-3 border border-gray-300 rounded-full focus:outline-none focus:ring-2 focus:ring-green-500 focus:border-transparent"
              />
              <button
                type="submit"
                disabled={!inputValue.trim()}
                className="px-6 py-3 bg-green-500 text-white rounded-full hover:bg-green-600 disabled:opacity-50 disabled:cursor-not-allowed transition-all font-medium"
              >
                Send
              </button>
            </div>
          </form>
        </div>

        {/* Members Panel (for groups) */}
        {isGroup && showMembers && (
          <div className="w-64 border-l border-gray-200 bg-white flex flex-col">
            <div className="p-4 border-b border-gray-200">
              <h4 className="font-semibold text-gray-900">Members</h4>
            </div>
            <div className="flex-1 overflow-y-auto p-2">
              {(conversation.members || []).map(member => {
                const memberUser = member.user || member;
                const isAdmin = member.role === 'admin';
                const isMe = member.user_id === user.id;
                
                return (
                  <div 
                    key={member.user_id}
                    className="flex items-center gap-3 p-2 rounded-lg hover:bg-gray-50"
                  >
                    <div className="w-8 h-8 rounded-full bg-green-500 flex items-center justify-center text-white text-sm font-semibold">
                      {(memberUser.username || 'U').charAt(0).toUpperCase()}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium text-gray-900 truncate">
                        {memberUser.username || 'Unknown'}
                        {isMe && <span className="text-gray-400 ml-1">(you)</span>}
                      </div>
                      {isAdmin && (
                        <div className="text-xs text-purple-600">Admin</div>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
