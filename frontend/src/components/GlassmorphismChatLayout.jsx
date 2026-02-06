import React from 'react';
import { IconSidebar } from './IconSidebar';
import { GlassChatSidebar } from './GlassChatSidebar';
import { GlassChatHeader } from './GlassChatHeader';
import { GlassChatMessages } from './GlassChatMessages';
import { GlassMessageInput } from './GlassMessageInput';
import { NewGroupModal } from './NewGroupModal';
import { NewChatModal } from './NewChatModal';
import { StarredMessagesModal } from './StarredMessagesModal';
import { SearchModal } from './SearchModal';
import { MembersPanel } from './MembersPanel';
import VideoCallModal from './VideoCallModal';
import { CallsTab } from './CallsTab';
import { IncomingCallModal } from './IncomingCallModal';
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
  const [loading, setLoading] = React.useState(true);
  const [showNewGroupModal, setShowNewGroupModal] = React.useState(false);
  const [showNewChatModal, setShowNewChatModal] = React.useState(false);
  const [showStarredModal, setShowStarredModal] = React.useState(false);
  const [showSearchModal, setShowSearchModal] = React.useState(false);
  const [showMembersPanel, setShowMembersPanel] = React.useState(false);
  const [showMobileChat, setShowMobileChat] = React.useState(false);
  
  const isMobile = useIsMobile();

  const {
    isConnected,
    messages,
    setMessages,
    sendMessage,
    joinRoom,
  } = useWebSocket(token);

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
          activeTab={activeTab} 
          onTabChange={setActiveTab}
          currentUser={user}
          onOpenStarred={() => setShowStarredModal(true)}
          onOpenSearch={() => setShowSearchModal(true)}
        />
      </div>

      {/* Chat List Panel / Calls Tab - Hidden on mobile when chat is open */}
      <div className={`${isMobile && showMobileChat ? 'hidden' : 'flex'} ${isMobile ? 'flex-1' : ''}`}>
        {activeTab === 'calls' ? (
          <div className="w-80 border-r border-border">
            <CallsTab
              currentUserId={user.id}
              onStartCall={(conversationId, callType) => {
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
            conversations={conversations}
            onLogout={onLogout}
            onNewGroup={handleNewGroup}
            onNewChat={handleNewChat}
            onOpenStarred={() => setShowStarredModal(true)}
            onMarkAllRead={handleMarkAllRead}
            isMobile={isMobile}
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
        onMemberAdded={(member) => {
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
    </div>
  )
}
