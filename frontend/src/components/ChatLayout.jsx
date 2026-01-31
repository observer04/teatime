import { useState, useEffect } from 'react';
import Sidebar from './Sidebar';
import ChatWindow from './ChatWindow';
import api from '../services/api';
import { useWebSocket } from '../hooks/useWebSocket';

export default function ChatLayout({ user, token, onLogout }) {
  const [conversations, setConversations] = useState([]);
  const [currentConversation, setCurrentConversation] = useState(null);
  const [_loading, setLoading] = useState(true);

  const {
    isConnected,
    messages,
    setMessages,
    sendMessage,
    joinRoom,
    startTyping,
    stopTyping
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

  const selectConversation = async (conv) => {
    setCurrentConversation(conv);
    joinRoom(conv.id);

    // Load messages if not already loaded
    if (!messages[conv.id]) {
      try {
        const data = await api.getMessages(conv.id);
        setMessages(prev => ({
          ...prev,
          [conv.id]: data.messages || []
        }));
      } catch (error) {
        console.error('Failed to load messages:', error);
      }
    }
  };

  const handleCreateDM = async (userId) => {
    try {
      const data = await api.createConversation('dm', [userId]);
      await loadConversations();
      
      // Find and select the new conversation
      const newConv = conversations.find(c => c.id === data.id);
      if (newConv) {
        selectConversation(newConv);
      }
    } catch (error) {
      console.error('Failed to create DM:', error);
      throw error;
    }
  };

  const handleCreateGroup = async (title, memberIds) => {
    try {
      const data = await api.createConversation('group', memberIds, title);
      await loadConversations();
      
      // Select the new group
      if (data) {
        selectConversation(data);
      }
    } catch (error) {
      console.error('Failed to create group:', error);
      throw error;
    }
  };

  return (
    <div className="h-screen flex bg-gray-100">
      <Sidebar
        user={user}
        conversations={conversations}
        currentConversation={currentConversation}
        onSelectConversation={selectConversation}
        onCreateDM={handleCreateDM}
        onCreateGroup={handleCreateGroup}
        onLogout={onLogout}
        isConnected={isConnected}
      />
      
      <ChatWindow
        conversation={currentConversation}
        messages={messages[currentConversation?.id] || []}
        user={user}
        onSendMessage={sendMessage}
        onStartTyping={startTyping}
        onStopTyping={stopTyping}
      />

      {/* Connection Status */}
      {!isConnected && (
        <div className="fixed bottom-4 left-1/2 transform -translate-x-1/2 bg-gray-900 text-white px-4 py-2 rounded-full shadow-lg flex items-center gap-2">
          <div className="w-2 h-2 bg-red-500 rounded-full animate-pulse"></div>
          <span className="text-sm">Reconnecting...</span>
        </div>
      )}
    </div>
  );
}
