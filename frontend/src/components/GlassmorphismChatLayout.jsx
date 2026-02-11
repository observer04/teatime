import React from 'react';
import { IconSidebar } from './IconSidebar';
import { GlassChatSidebar } from './GlassChatSidebar';
import { GlassChatHeader } from './GlassChatHeader';
import { GlassChatMessages } from './GlassChatMessages';
import { GlassMessageInput } from './GlassMessageInput';
// Lazy load modals to break circular dependencies and improve initial load
const NewGroupModal = React.lazy(() => import('./NewGroupModal').then(module => ({ default: module.NewGroupModal })));
const NewChatModal = React.lazy(() => import('./NewChatModal').then(module => ({ default: module.NewChatModal })));
const StarredMessagesModal = React.lazy(() => import('./StarredMessagesModal').then(module => ({ default: module.StarredMessagesModal })));
const SearchModal = React.lazy(() => import('./SearchModal').then(module => ({ default: module.SearchModal })));
const MembersPanel = React.lazy(() => import('./MembersPanel').then(module => ({ default: module.MembersPanel })));
const VideoCallModal = React.lazy(() => import('./VideoCallModal')); // Default export
const IncomingCallModal = React.lazy(() => import('./IncomingCallModal').then(module => ({ default: module.IncomingCallModal })));
const ProfileModal = React.lazy(() => import('./ProfileModal').then(module => ({ default: module.ProfileModal })));
import { CallsTab } from './CallsTab';
import api from '../services/api';
import { useWebSocket } from '../hooks/useWebSocket';
import { useWebRTC } from '../hooks/useWebRTC';

// Custom hook for detecting mobile viewport
function useIsMobile(breakpoint = 768) {
  const [isMobile, setIsMobile] = React.useState(window.innerWidth < breakpoint);

  React.useEffect(() => {
    const handleResize = () => setIsMobile(window.innerWidth < breakpoint);
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [breakpoint]);

  return isMobile;
}

export default function GlassmorphismChatLayout({ user, token, onLogout }) {
  const [activeTab, setActiveTab] = React.useState("chats");
  const [conversations, setConversations] = React.useState([]);
  const [currentConversation, setCurrentConversation] = React.useState(null);
  const [_loading, setLoading] = React.useState(true);
  const [showNewGroupModal, setShowNewGroupModal] = React.useState(false);
  const [showNewChatModal, setShowNewChatModal] = React.useState(false);
  const [showStarredModal, setShowStarredModal] = React.useState(false);
  const [showSearchModal, setShowSearchModal] = React.useState(false);
  const [showArchivedChats, setShowArchivedChats] = React.useState(false);
  const [archivedConversations, setArchivedConversations] = React.useState([]);
  const [showMembersPanel, setShowMembersPanel] = React.useState(false);
  const [showMobileChat, setShowMobileChat] = React.useState(false);
  const [showProfileModal, setShowProfileModal] = React.useState(false);
  const [currentUser, setCurrentUser] = React.useState(user);
  
  const isMobile = useIsMobile();

  // Handle incoming messages for unread counts
  const handleIncomingMessage = React.useCallback((message) => {
    setConversations(prev => {
      return prev.map(conv => {
        if (conv.id === message.conversation_id) {
          const isCurrentChat = currentConversation && currentConversation.id === message.conversation_id;
          
          if (isCurrentChat) {
            // Mark as read immediately if chat is open
            api.markConversationRead(message.conversation_id, message.id).catch(console.error);
          }

          return {
            ...conv,
            last_message: message,
            unread_count: isCurrentChat ? 0 : (conv.unread_count || 0) + 1,
            updated_at: message.created_at
          };
        }
        return conv;
      });
    });
  }, [currentConversation]);

  const {
    isConnected,
    messages,
    setMessages,
    sendMessage,
    joinRoom,
  } = useWebSocket(token, handleIncomingMessage);

  // WebRTC video call hook
  const {
    isInCall,
    callState,
    localStream,
    remoteStreams,
    isMuted,
    isVideoOff,
    participants,
    incomingCall,
    error: webrtcError,
    joinCall,
    leaveCall,
    toggleMute,
    toggleVideo,
    acceptCall,
    declineCall
  } = useWebRTC(user.id);

  const [showVideoCall, setShowVideoCall] = React.useState(false);

  // Show alert when WebRTC error occurs
  React.useEffect(() => {
    if (webrtcError) {
      alert(`Call Error: ${webrtcError}`);
    }
  }, [webrtcError]);

  React.useEffect(() => {
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
      
      // Clear unread count locally
      setConversations(prev => prev.map(c => 
        c.id === chatId ? { ...c, unread_count: 0 } : c
      ));

      // Mark as read on backend
      api.markConversationRead(chatId).catch(console.error);

      joinRoom(conversation.id);
      
      // Show chat on mobile
      if (isMobile) {
        setShowMobileChat(true);
      }
      
      try {
        const data = await api.getMessages(conversation.id);
        // Reverse messages so oldest are first (API returns DESC order)
        const orderedMessages = (data.messages || []).reverse();
        setMessages(prev => ({
          ...prev,
          [conversation.id]: orderedMessages
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

  const handleNewChat = () => {
    setShowNewChatModal(true);
  };

  const handleChatStarted = (conversation) => {
    // Check if conversation already exists in list
    const existing = conversations.find(c => c.id === conversation.id);
    if (!existing) {
      setConversations(prev => [conversation, ...prev]);
    }
    setCurrentConversation(conversation);
    joinRoom(conversation.id);
    setShowNewChatModal(false);
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

  // Load archived conversations
  const loadArchivedConversations = async () => {
    try {
      const data = await api.getArchivedConversations();
      setArchivedConversations(data.conversations || []);
    } catch (error) {
      console.error('Failed to load archived conversations:', error);
    }
  };

  // Handle opening archived chats view
  const handleOpenArchived = async () => {
    await loadArchivedConversations();
    setShowArchivedChats(true);
  };

  // Archive a conversation
  const handleArchiveConversation = async (conversationId) => {
    try {
      await api.archiveConversation(conversationId);
      // Remove from active list, add to archived
      const conv = conversations.find(c => c.id === conversationId);
      if (conv) {
        setConversations(prev => prev.filter(c => c.id !== conversationId));
        setArchivedConversations(prev => [conv, ...prev]);
      }
      // If currently viewing this chat, clear it
      if (currentConversation?.id === conversationId) {
        setCurrentConversation(null);
      }
    } catch (error) {
      console.error('Failed to archive conversation:', error);
    }
  };

  // Unarchive a conversation
  const handleUnarchiveConversation = async (conversationId) => {
    try {
      await api.unarchiveConversation(conversationId);
      // Remove from archived list, add to active
      const conv = archivedConversations.find(c => c.id === conversationId);
      if (conv) {
        setArchivedConversations(prev => prev.filter(c => c.id !== conversationId));
        setConversations(prev => [conv, ...prev]);
      }
    } catch (error) {
      console.error('Failed to unarchive conversation:', error);
    }
  };

  // Start a video call in the current conversation
  const handleStartVideoCall = async () => {
    if (!currentConversation) return;
    try {
      await joinCall(currentConversation.id, true);
      setShowVideoCall(true);
    } catch (error) {
      console.error('Failed to start video call:', error);
    }
  };

  // Start an audio call in the current conversation
  const handleStartAudioCall = async () => {
    if (!currentConversation) return;
    try {
      await joinCall(currentConversation.id, false); // false = audio only
      setShowVideoCall(true); // Reuse the same modal, video will be off by default
    } catch (error) {
      console.error('Failed to start audio call:', error);
    }
  };

  const handleEndVideoCall = () => {
    leaveCall();
    setShowVideoCall(false);
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
    receiptStatus: msg.receipt_status || 'sent', // "sent", "delivered", "read"
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
        status: null, // Don't show misleading online status - we don't track real presence yet
        avatar: currentConversation.type === 'group' 
          ? currentConversation.avatar_url 
          : currentConversation.other_user?.avatar_url,
        isChannel: currentConversation.type === 'group',
        memberCount: currentConversation.member_count
      }
    : null;

  const handleMobileBack = () => {
    setShowMobileChat(false);
  };

  return (
    <div className="flex h-screen bg-background overflow-hidden">
      {/* Left Icon Sidebar - Hidden on mobile when chat is open */}
      <div className={`${isMobile && showMobileChat ? 'hidden' : 'flex'}`}>
        <IconSidebar 
          activeTab={showArchivedChats ? 'archived' : activeTab} 
          onTabChange={(tab) => {
            if (tab === 'archived') {
              handleOpenArchived();
            } else {
              setShowArchivedChats(false);
              setActiveTab(tab);
            }
          }}
          currentUser={currentUser}
          onOpenStarred={() => setShowStarredModal(true)}
          onOpenSearch={() => setShowSearchModal(true)}
          onOpenProfile={() => setShowProfileModal(true)}
        />
      </div>

      {/* Chat List Panel / Calls Tab - Hidden on mobile when chat is open */}
      <div className={`${isMobile && showMobileChat ? 'hidden' : 'flex'} ${isMobile ? 'flex-1' : ''}`}>
        {activeTab === 'calls' ? (
          <div className="w-80 border-r border-border">
            <CallsTab
              currentUserId={user.id}
              onStartCall={(conversationId, _callType) => {
                // Find the conversation and start a call
                const conv = conversations.find(c => c.id === conversationId);
                if (conv) {
                  setCurrentConversation(conv);
                  handleStartVideoCall();
                }
              }}
            />
          </div>
        ) : (
          <GlassChatSidebar 
            activeChat={currentConversation?.id || ''} 
            onChatSelect={handleChatSelect}
            conversations={showArchivedChats ? archivedConversations : conversations}
            onLogout={onLogout}
            onNewGroup={handleNewGroup}
            onNewChat={handleNewChat}
            onOpenStarred={() => setShowStarredModal(true)}
            onMarkAllRead={handleMarkAllRead}
            isMobile={isMobile}
            isArchiveView={showArchivedChats}
            onArchive={handleArchiveConversation}
            onUnarchive={handleUnarchiveConversation}
          />
        )}
      </div>

      {/* Main Chat Area - Full screen on mobile when chat is open */}
      <main className={`flex-1 flex flex-col min-w-0 ${isMobile && !showMobileChat ? 'hidden' : ''}`}>
        {currentChat ? (
          <>
            <GlassChatHeader 
              {...currentChat} 
              onSearch={() => setShowSearchModal(true)}
              onBack={isMobile ? handleMobileBack : undefined}
              onMembersClick={() => setShowMembersPanel(true)}
              onVideoCall={handleStartVideoCall}
              onAudioCall={handleStartAudioCall}
            />
            <GlassChatMessages 
              messages={transformedMessages}
              onMessageDeleted={(msg) => {
                // Remove deleted message from local state
                if (currentConversation) {
                  setMessages(prev => ({
                    ...prev,
                    [currentConversation.id]: (prev[currentConversation.id] || []).filter(m => m.id !== msg.id)
                  }));
                }
              }}
            />
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
      <React.Suspense fallback={null}>
        <NewGroupModal
          isOpen={showNewGroupModal}
          onClose={() => setShowNewGroupModal(false)}
          onGroupCreated={handleGroupCreated}
        />

        <NewChatModal
          isOpen={showNewChatModal}
          onClose={() => setShowNewChatModal(false)}
          onChatStarted={handleChatStarted}
          currentUserId={user.id}
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

        <MembersPanel
          isOpen={showMembersPanel}
          onClose={() => setShowMembersPanel(false)}
          conversation={currentConversation}
          currentUserId={user.id}
          onMemberAdded={(_member) => {
            // Update member count in current conversation
            if (currentConversation) {
              setCurrentConversation(prev => ({
                ...prev,
                member_count: (prev.member_count || 0) + 1
              }));
            }
          }}
          onMemberRemoved={(member) => {
            if (member.left) {
              // User left the group - navigate away
              setCurrentConversation(null);
              setShowMobileChat(false);
              loadConversations();
            } else {
              // Someone was removed - update count
              if (currentConversation) {
                setCurrentConversation(prev => ({
                  ...prev,
                  member_count: Math.max((prev.member_count || 1) - 1, 1)
                }));
              }
            }
          }}
        />

        {/* Video Call Modal */}
        <VideoCallModal
          isOpen={showVideoCall || isInCall}
          onClose={handleEndVideoCall}
          conversationName={currentChat?.name || 'Video Call'}
          localStream={localStream}
          remoteStreams={remoteStreams}
          isMuted={isMuted}
          isVideoOff={isVideoOff}
          onToggleMute={toggleMute}
          onToggleVideo={toggleVideo}
          onEndCall={handleEndVideoCall}
          participants={participants}
          callState={callState}
        />

        {/* Incoming Call Modal */}
        <IncomingCallModal
          isOpen={!!incomingCall}
          caller={incomingCall?.caller}
          callType={incomingCall?.callType}
          isGroup={incomingCall?.isGroup}
          conversationName={incomingCall?.conversationName}
          onAccept={async (withVideo) => {
            try {
              await acceptCall(withVideo);
              // Only show video call UI if accept succeeded
              setShowVideoCall(true);
            } catch (err) {
              console.error('Failed to accept call:', err);
              // Error is already set in useWebRTC hook, just log here
            }
          }}
          onDecline={declineCall}
        />

        {/* Profile Modal */}
        <ProfileModal
          isOpen={showProfileModal}
          onClose={() => setShowProfileModal(false)}
          user={currentUser}
          onLogout={onLogout}
          onProfileUpdated={async () => {
            try {
              const updated = await api.getMe();
              setCurrentUser(updated);
            } catch (err) {
              console.error('Failed to refresh user:', err);
            }
          }}
        />
      </React.Suspense>
    </div>
  )
}
