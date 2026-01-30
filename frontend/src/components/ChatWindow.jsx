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
  const messagesEndRef = useRef(null);

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

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

  const getDMName = () => {
    if (!conversation.members) return 'Chat';
    const other = conversation.members.find(m => m.user_id !== user.id);
    if (other?.user) return other.user.username;
    if (other?.username) return other.username;
    return 'Chat';
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

  const chatName = conversation.type === 'dm' ? getDMName() : conversation.name || 'Group Chat';

  return (
    <div className="flex-1 flex flex-col bg-white">
      {/* Chat Header */}
      <div className="px-6 py-4 border-b border-gray-200 bg-white">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-full bg-green-500 flex items-center justify-center text-white font-semibold">
            {chatName.charAt(0).toUpperCase()}
          </div>
          <div>
            <h3 className="font-semibold text-gray-900">{chatName}</h3>
            {/* Typing indicator placeholder */}
            <div className="text-sm text-gray-500 h-5"></div>
          </div>
        </div>
      </div>

      {/* Messages */}
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
  );
}
