import { useState, useEffect } from 'react';
import { ChatSidebar } from './ChatSidebarIntegrated';
import { ChatHeader } from './chat-header';
import { ChatMessages } from './chat-messages';
import { MessageInput } from './message-input';
import api from '../services/api';
import { useWebSocket } from '../hooks/useWebSocket';

export default function ModernChatLayout({ user, token, onLogout }) {
  const [conversations, setConversations] = useState([]);
  const [currentConversation, setCurrentConversation] = useState(null);
  const [_loading, setLoading] = useState(true);

  const {
    isConnected,
    messages,
    setMessages,
    sendMessage,
    joinRoom,
    startTyping: _startTyping,
    stopTyping: _stopTyping
  } = useWebSocket(token);

  useEffect(() => {
    loadConversations();
  }, []);

  const loadConversations = async () => {
    try {
      const data = await api.getConversations();
      setConversations(data.conversations || []);
    } catch (error) {
      console.error('Failed to load conversations:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleChatSelect = async (chatId) => {
    const conversation = conversations.find(c => c.id === chatId);
    if (conversation) {
      setCurrentConversation(conversation);
      joinRoom(conversation.id);
      
      try {
        const data = await api.getMessages(conversation.id);
        // Update messages for this specific conversation
        setMessages(prev => ({
          ...prev,
          [conversation.id]: data.messages || []
        }));
      } catch (error) {
        console.error('Failed to load messages:', error);
      }
    }
  };

  const handleSendMessage = async (content) => {
    if (!currentConversation) return;
    
    try {
      await sendMessage(currentConversation.id, content);
    } catch (error) {
      console.error('Failed to send message:', error);
    }
  };

  // Get messages for current conversation
  const currentMessages = currentConversation?.id 
    ? (messages[currentConversation.id] || [])
    : [];

  // Transform messages for ChatMessages component
  const transformedMessages = currentMessages.map(msg => ({
    id: msg.id,
    sender: {
      id: msg.user_id || msg.sender_id,
      name: msg.username || msg.sender?.username || 'Unknown',
      avatar: msg.avatar_url
    },
    content: msg.body_text || msg.content,
    timestamp: new Date(msg.created_at),
    isOwn: (msg.user_id || msg.sender_id) === user.id
  }));

  return (
    <div className="flex h-screen bg-background">
      <ChatSidebar 
        activeChat={currentConversation?.id || ''}
        onChatSelect={handleChatSelect}
        conversations={conversations}
        currentUser={user}
        onLogout={onLogout}
      />
      
      {currentConversation ? (
        <div className="flex-1 flex flex-col">
          <ChatHeader
            name={currentConversation.title || currentConversation.other_user?.username || 'Chat'}
            status={isConnected ? 'online' : 'offline'}
            avatar={currentConversation.other_user?.avatar_url}
            isChannel={currentConversation.type === 'group'}
            memberCount={currentConversation.member_count}
          />
          
          <ChatMessages messages={transformedMessages} />
          
          <MessageInput onSend={handleSendMessage} />
        </div>
      ) : (
        <div className="flex-1 flex items-center justify-center bg-muted/20">
          <div className="text-center">
            <div className="text-6xl mb-4">🍵</div>
            <h2 className="text-2xl font-semibold text-foreground mb-2">Welcome to TeaTime</h2>
            <p className="text-muted-foreground">Select a conversation to start chatting</p>
          </div>
        </div>
      )}
    </div>
  );
}
