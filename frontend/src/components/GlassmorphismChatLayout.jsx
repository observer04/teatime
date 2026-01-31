import { useState, useEffect } from 'react';
import { IconSidebar } from './IconSidebar';
import { GlassChatSidebar } from './GlassChatSidebar';
import { GlassChatHeader } from './GlassChatHeader';
import { GlassChatMessages } from './GlassChatMessages';
import { GlassMessageInput } from './GlassMessageInput';
import { NewGroupModal } from './NewGroupModal';
import { StarredMessagesModal } from './StarredMessagesModal';
import { SearchModal } from './SearchModal';
import api from '../services/api';
import { useWebSocket } from '../hooks/useWebSocket';

export default function GlassmorphismChatLayout({ user, token, onLogout }) {
  const [activeTab, setActiveTab] = useState("chats");
  const [conversations, setConversations] = useState([]);
  const [currentConversation, setCurrentConversation] = useState(null);
  const [loading, setLoading] = useState(true);
  const [showNewGroupModal, setShowNewGroupModal] = useState(false);
  const [showStarredModal, setShowStarredModal] = useState(false);
  const [showSearchModal, setShowSearchModal] = useState(false);

  const {
    isConnected,
    messages,
    setMessages,
    sendMessage,
    joinRoom,
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
        setMessages(prev => ({
          ...prev,
          [conversation.id]: data.messages || []
        }));
      } catch (error) {
        console.error('Failed to load messages:', error);
      }
    }
  };

  const handleSendMessage = async (content, attachmentId = null) => {
    if (!currentConversation) return;
    
    try {
      await sendMessage(currentConversation.id, content, attachmentId);
    } catch (error) {
      console.error('Failed to send message:', error);
    }
  };

  const handleNewGroup = () => {
    setShowNewGroupModal(true);
  };

  const handleGroupCreated = (conversation) => {
    setConversations(prev => [conversation, ...prev]);
    setCurrentConversation(conversation);
    joinRoom(conversation.id);
    setShowNewGroupModal(false);
  };

  const handleStarredMessageClick = (message) => {
    // Navigate to the conversation containing this message
    const conv = conversations.find(c => c.id === message.conversation_id);
    if (conv) {
      handleChatSelect(conv.id);
    }
    setShowStarredModal(false);
  };

  const handleSearchMessageClick = (message) => {
    // Navigate to the conversation containing this message
    const conv = conversations.find(c => c.id === message.conversation_id);
    if (conv) {
      handleChatSelect(conv.id);
    }
    setShowSearchModal(false);
  };

  const handleMarkAllRead = async () => {
    try {
      await api.markAllConversationsRead();
      // Refresh conversations to update unread counts
      loadConversations();
    } catch (error) {
      console.error('Failed to mark all as read:', error);
    }
  };

  // Get messages for current conversation
  const currentMessages = currentConversation?.id 
    ? (messages[currentConversation.id] || [])
    : [];

  // Transform messages for GlassChatMessages component
  const transformedMessages = currentMessages.map(msg => ({
    id: msg.id,
    sender: {
      id: msg.user_id || msg.sender_id,
      name: msg.username || msg.sender?.username || 'Unknown',
      avatar: msg.avatar_url
    },
    content: msg.body_text || msg.content,
    timestamp: new Date(msg.created_at),
    isOwn: (msg.user_id || msg.sender_id) === user.id,
    isRead: true,
    attachment: msg.attachment ? {
      id: msg.attachment.id,
      mime_type: msg.attachment.mime_type,
      filename: msg.attachment.filename,
      size_bytes: msg.attachment.size_bytes
    } : null
  }));

  const currentChat = currentConversation 
    ? {
        name: currentConversation.type === 'group' 
          ? currentConversation.title 
          : (currentConversation.other_user?.username || 'Unknown'),
        status: isConnected ? "online" : "offline",
        avatar: currentConversation.type === 'group' 
          ? currentConversation.avatar_url 
          : currentConversation.other_user?.avatar_url,
        isChannel: currentConversation.type === 'group',
        memberCount: currentConversation.member_count
      }
    : null;

  return (
    <div className="flex h-screen bg-background overflow-hidden">
      {/* Left Icon Sidebar */}
      <IconSidebar 
        activeTab={activeTab} 
        onTabChange={setActiveTab}
        currentUser={user}
        onOpenStarred={() => setShowStarredModal(true)}
        onOpenSearch={() => setShowSearchModal(true)}
      />

      {/* Chat List Panel */}
      <GlassChatSidebar 
        activeChat={currentConversation?.id || ''} 
        onChatSelect={handleChatSelect}
        conversations={conversations}
        onLogout={onLogout}
        onNewGroup={handleNewGroup}
        onOpenStarred={() => setShowStarredModal(true)}
        onMarkAllRead={handleMarkAllRead}
      />

      {/* Main Chat Area */}
      <main className="flex-1 flex flex-col min-w-0">
        {currentChat ? (
          <>
            <GlassChatHeader 
              {...currentChat} 
              onSearch={() => setShowSearchModal(true)}
            />
            <GlassChatMessages messages={transformedMessages} />
            <GlassMessageInput 
              conversationId={currentConversation.id}
              onSend={handleSendMessage} 
              disabled={!isConnected} 
            />
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center bg-card">
            <div className="text-center">
              <div className="text-6xl mb-4">💬</div>
              <h2 className="text-2xl font-semibold text-foreground mb-2">Welcome to TeaTime</h2>
              <p className="text-muted-foreground">Select a conversation to start chatting</p>
            </div>
          </div>
        )}
      </main>

      {/* Modals */}
      <NewGroupModal
        isOpen={showNewGroupModal}
        onClose={() => setShowNewGroupModal(false)}
        onGroupCreated={handleGroupCreated}
      />

      <StarredMessagesModal
        isOpen={showStarredModal}
        onClose={() => setShowStarredModal(false)}
        onMessageClick={handleStarredMessageClick}
      />

      <SearchModal
        isOpen={showSearchModal}
        onClose={() => setShowSearchModal(false)}
        conversationId={currentConversation?.id}
        onMessageClick={handleSearchMessageClick}
      />
    </div>
  )
}
