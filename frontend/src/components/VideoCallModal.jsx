import React from 'react';
import { 
  X, 
  Mic, 
  MicOff, 
  Video, 
  VideoOff, 
  PhoneOff,
  Maximize2,
  Minimize2,
  Users,
  ScreenShare,
  Move
} from 'lucide-react';

/**
 * Remote video tile for main display (large)
 */
function RemoteVideoTile({ stream, username, isMuted, isVideoOff }) {
  const videoRef = React.useRef(null);

  React.useEffect(() => {
    const videoEl = videoRef.current;
    if (videoEl && stream) {
      videoEl.srcObject = stream;
      // Explicitly attempt to play to handle autoplay policies
      videoEl.play().catch(e => console.error("Error auto-playing remote video:", e));
    }
    return () => {
      if (videoEl) {
        videoEl.srcObject = null;
      }
    };
  }, [stream]);

  return (
    <div className="relative w-full h-full bg-neutral-900/80 overflow-hidden">
      {stream && !isVideoOff ? (
        <video
          ref={videoRef}
          autoPlay
          playsInline
          className="w-full h-full object-cover"
        />
      ) : (
        <div className="w-full h-full flex items-center justify-center bg-gradient-to-br from-blue-500/20 to-purple-500/20">
          <div className="w-24 h-24 rounded-full bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center text-white text-4xl font-bold uppercase shadow-2xl">
            {username?.charAt(0) || '?'}
          </div>
        </div>
      )}
      
      {/* Name badge */}
      <div className="absolute bottom-20 left-4 flex items-center gap-2">
        <span className="px-4 py-2 bg-black/50 backdrop-blur-sm rounded-full text-white text-base font-medium">
          {username || 'Unknown'}
        </span>
        {isMuted && (
          <span className="p-2 bg-red-500/80 rounded-full">
            <MicOff className="w-4 h-4 text-white" />
          </span>
        )}
      </div>
    </div>
  );
}

/**
 * Local video Picture-in-Picture component (small, draggable)
 */
function LocalVideoPiP({ stream, isMuted, isVideoOff }) {
  const videoRef = React.useRef(null);
  const pipRef = React.useRef(null);
  const [position, setPosition] = React.useState({ x: 16, y: 80 }); // Top-right with padding
  const [isDragging, setIsDragging] = React.useState(false);
  const dragOffset = React.useRef({ x: 0, y: 0 });

  React.useEffect(() => {
    const videoEl = videoRef.current;
    if (videoEl && stream) {
      videoEl.srcObject = stream;
      // Local video is muted by default (prop), so autoplay usually works, but good to be explicit
      videoEl.play().catch(e => console.error("Error auto-playing local video:", e));
    }
    return () => {
      if (videoEl) {
        videoEl.srcObject = null;
      }
    };
  }, [stream]);

  // Handle drag start
  const handleDragStart = React.useCallback((e) => {
    if (!pipRef.current) return;
    
    const clientX = e.type === 'touchstart' ? e.touches[0].clientX : e.clientX;
    const clientY = e.type === 'touchstart' ? e.touches[0].clientY : e.clientY;
    
    const rect = pipRef.current.getBoundingClientRect();
    dragOffset.current = {
      x: clientX - rect.left,
      y: clientY - rect.top
    };
    setIsDragging(true);
    e.preventDefault();
  }, []);

  // Handle drag move
  const handleDragMove = React.useCallback((e) => {
    if (!isDragging || !pipRef.current) return;
    
    const clientX = e.type === 'touchmove' ? e.touches[0].clientX : e.clientX;
    const clientY = e.type === 'touchmove' ? e.touches[0].clientY : e.clientY;
    
    const parent = pipRef.current.parentElement;
    if (!parent) return;
    
    const parentRect = parent.getBoundingClientRect();
    const pipRect = pipRef.current.getBoundingClientRect();
    
    // Calculate new position relative to parent right edge
    let newX = parentRect.right - clientX - (pipRect.width - dragOffset.current.x);
    let newY = clientY - parentRect.top - dragOffset.current.y;
    
    // Constrain within bounds
    newX = Math.max(16, Math.min(parentRect.width - pipRect.width - 16, newX));
    newY = Math.max(80, Math.min(parentRect.height - pipRect.height - 100, newY));
    
    setPosition({ x: newX, y: newY });
  }, [isDragging]);

  // Handle drag end
  const handleDragEnd = React.useCallback(() => {
    setIsDragging(false);
  }, []);

  // Add/remove event listeners
  React.useEffect(() => {
    if (isDragging) {
      window.addEventListener('mousemove', handleDragMove);
      window.addEventListener('mouseup', handleDragEnd);
      window.addEventListener('touchmove', handleDragMove);
      window.addEventListener('touchend', handleDragEnd);
    }
    return () => {
      window.removeEventListener('mousemove', handleDragMove);
      window.removeEventListener('mouseup', handleDragEnd);
      window.removeEventListener('touchmove', handleDragMove);
      window.removeEventListener('touchend', handleDragEnd);
    };
  }, [isDragging, handleDragMove, handleDragEnd]);

  return (
    <div
      ref={pipRef}
      className={`absolute z-20 w-32 h-24 sm:w-40 sm:h-30 md:w-48 md:h-36 rounded-xl overflow-hidden shadow-2xl border-2 border-white/20 bg-neutral-900 cursor-move transition-shadow ${
        isDragging ? 'shadow-lg shadow-blue-500/30 border-blue-400/50' : 'hover:border-white/40'
      }`}
      style={{
        right: `${position.x}px`,
        top: `${position.y}px`,
      }}
      onMouseDown={handleDragStart}
      onTouchStart={handleDragStart}
    >
      {stream && !isVideoOff ? (
        <video
          ref={videoRef}
          autoPlay
          playsInline
          muted
          className="w-full h-full object-cover"
        />
      ) : (
        <div className="w-full h-full flex items-center justify-center bg-gradient-to-br from-blue-500/20 to-purple-500/20">
          <div className="w-12 h-12 rounded-full bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center text-white text-lg font-bold">
            You
          </div>
        </div>
      )}
      
      {/* Drag indicator */}
      <div className="absolute top-1 right-1 p-1 bg-black/40 rounded-full opacity-60">
        <Move className="w-3 h-3 text-white" />
      </div>
      
      {/* Status indicators */}
      <div className="absolute bottom-1 left-1 flex gap-1">
        {isMuted && (
          <span className="p-1 bg-red-500/80 rounded-full">
            <MicOff className="w-2.5 h-2.5 text-white" />
          </span>
        )}
        {isVideoOff && (
          <span className="p-1 bg-red-500/80 rounded-full">
            <VideoOff className="w-2.5 h-2.5 text-white" />
          </span>
        )}
      </div>
    </div>
  );
}

/**
 * Grid layout for multiple remote participants
 */
function RemoteVideoGrid({ remoteStreams }) {
  const remoteEntries = Object.entries(remoteStreams);
  
  // Determine grid layout based on participant count
  const getGridClass = () => {
    if (remoteEntries.length <= 1) return 'grid-cols-1';
    if (remoteEntries.length === 2) return 'grid-cols-2';
    if (remoteEntries.length <= 4) return 'grid-cols-2 grid-rows-2';
    if (remoteEntries.length <= 6) return 'grid-cols-3 grid-rows-2';
    return 'grid-cols-3 grid-rows-3';
  };

  return (
    <div className={`w-full h-full grid ${getGridClass()} gap-2`}>
      {remoteEntries.map(([peerId, { stream, username }]) => (
        <div key={peerId} className="relative bg-neutral-900/80 rounded-lg overflow-hidden">
          <RemoteVideoTile
            stream={stream}
            username={username}
            isMuted={false}
            isVideoOff={!stream}
          />
        </div>
      ))}
    </div>
  );
}

/**
 * Waiting/connecting screen when no remote participants
 */
function WaitingScreen({ conversationName, callState }) {
  return (
    <div className="w-full h-full flex flex-col items-center justify-center bg-gradient-to-br from-neutral-900 to-black">
      <div className="w-24 h-24 rounded-full bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center text-white text-4xl font-bold uppercase mb-6 animate-pulse">
        {conversationName?.charAt(0) || '?'}
      </div>
      <h3 className="text-white text-xl font-semibold mb-2">{conversationName}</h3>
      <p className="text-white/60 text-sm flex items-center gap-2">
        {callState === 'connecting' ? (
          <>
            <span className="relative flex h-3 w-3">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-yellow-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-3 w-3 bg-yellow-500"></span>
            </span>
            Connecting...
          </>
        ) : callState === 'ringing' ? (
          <>
            <span className="relative flex h-3 w-3">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-3 w-3 bg-green-500"></span>
            </span>
            Ringing...
          </>
        ) : (
          <>
            <span className="relative flex h-3 w-3">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-3 w-3 bg-blue-500"></span>
            </span>
            Waiting for others to join...
          </>
        )}
      </p>
    </div>
  );
}

/**
 * Call controls component
 */
function CallControls({ 
  isMuted, 
  isVideoOff, 
  onToggleMute, 
  onToggleVideo, 
  onEndCall,
  onScreenShare 
}) {
  return (
    <div className="absolute bottom-8 left-1/2 -translate-x-1/2 flex items-center gap-4">
      <div className="flex items-center gap-3 px-6 py-3 bg-black/60 backdrop-blur-xl rounded-full border border-white/10">
        {/* Mute button */}
        <button
          onClick={onToggleMute}
          className={`p-3 rounded-full transition-all ${
            isMuted 
              ? 'bg-red-500 hover:bg-red-600 text-white' 
              : 'bg-white/10 hover:bg-white/20 text-white'
          }`}
          title={isMuted ? 'Unmute' : 'Mute'}
        >
          {isMuted ? <MicOff className="w-5 h-5" /> : <Mic className="w-5 h-5" />}
        </button>

        {/* Video button */}
        <button
          onClick={onToggleVideo}
          className={`p-3 rounded-full transition-all ${
            isVideoOff 
              ? 'bg-red-500 hover:bg-red-600 text-white' 
              : 'bg-white/10 hover:bg-white/20 text-white'
          }`}
          title={isVideoOff ? 'Turn on camera' : 'Turn off camera'}
        >
          {isVideoOff ? <VideoOff className="w-5 h-5" /> : <Video className="w-5 h-5" />}
        </button>

        {/* Screen share button */}
        <button
          onClick={onScreenShare}
          className="p-3 rounded-full bg-white/10 hover:bg-white/20 text-white transition-all"
          title="Share screen"
        >
          <ScreenShare className="w-5 h-5" />
        </button>

        {/* End call button */}
        <button
          onClick={onEndCall}
          className="p-3 rounded-full bg-red-500 hover:bg-red-600 text-white transition-all ml-4"
          title="Leave call"
        >
          <PhoneOff className="w-5 h-5" />
        </button>
      </div>
    </div>
  );
}

/**
 * Main video call modal component
 */
export default function VideoCallModal({
  isOpen,
  onClose,
  conversationName,
  localStream,
  remoteStreams,
  isMuted,
  isVideoOff,
  onToggleMute,
  onToggleVideo,
  onEndCall,
  participants,
  callState
}) {
  const [isFullscreen, setIsFullscreen] = React.useState(false);
  const modalRef = React.useRef(null);

  // Handle fullscreen
  const toggleFullscreen = async () => {
    if (!document.fullscreenElement) {
      await modalRef.current?.requestFullscreen();
      setIsFullscreen(true);
    } else {
      await document.exitFullscreen();
      setIsFullscreen(false);
    }
  };

  // Listen for fullscreen changes
  React.useEffect(() => {
    const handleFullscreenChange = () => {
      setIsFullscreen(!!document.fullscreenElement);
    };
    document.addEventListener('fullscreenchange', handleFullscreenChange);
    return () => document.removeEventListener('fullscreenchange', handleFullscreenChange);
  }, []);

  // Close on escape
  React.useEffect(() => {
    const handleEscape = (e) => {
      if (e.key === 'Escape' && !document.fullscreenElement) {
        onClose?.();
      }
    };
    window.addEventListener('keydown', handleEscape);
    return () => window.removeEventListener('keydown', handleEscape);
  }, [onClose]);

  if (!isOpen) return null;

  const remoteStreamEntries = Object.entries(remoteStreams);
  const hasRemoteParticipants = remoteStreamEntries.length > 0;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/80 backdrop-blur-sm" />
      
      {/* Modal */}
      <div 
        ref={modalRef}
        className="relative w-full h-full flex flex-col bg-gradient-to-br from-neutral-900/95 to-black overflow-hidden"
      >
        {/* Header */}
        <div className="absolute top-0 left-0 right-0 z-30 flex items-center justify-between p-4 bg-gradient-to-b from-black/60 to-transparent">
          <div className="flex items-center gap-3">
            <h2 className="text-white font-semibold text-lg">{conversationName}</h2>
            <span className="flex items-center gap-1 px-2 py-1 bg-white/10 rounded-full text-white/70 text-sm">
              <Users className="w-4 h-4" />
              {participants?.length || (remoteStreamEntries.length + 1)}
            </span>
            {(callState === 'connecting' || callState === 'ringing') && (
              <span className={`px-2 py-1 rounded-full text-sm animate-pulse ${
                callState === 'ringing' 
                  ? 'bg-green-500/20 text-green-400' 
                  : 'bg-yellow-500/20 text-yellow-400'
              }`}>
                {callState === 'ringing' ? 'Ringing...' : 'Connecting...'}
              </span>
            )}
          </div>
          
          <div className="flex items-center gap-2">
            <button
              onClick={toggleFullscreen}
              className="p-2 rounded-full bg-white/10 hover:bg-white/20 text-white transition-all"
              title={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
            >
              {isFullscreen ? (
                <Minimize2 className="w-5 h-5" />
              ) : (
                <Maximize2 className="w-5 h-5" />
              )}
            </button>
          </div>
        </div>

        {/* Main video area - WhatsApp style PiP layout */}
        <div className="flex-1 relative">
          {/* Remote video(s) - full screen or grid */}
          {hasRemoteParticipants ? (
            remoteStreamEntries.length === 1 ? (
              // Single remote participant - full screen
              <RemoteVideoTile
                stream={remoteStreamEntries[0][1].stream}
                username={remoteStreamEntries[0][1].username}
                isMuted={false}
                isVideoOff={!remoteStreamEntries[0][1].stream}
              />
            ) : (
              // Multiple remote participants - grid
              <RemoteVideoGrid remoteStreams={remoteStreams} />
            )
          ) : (
            // No remote participants yet - show waiting screen
            <WaitingScreen conversationName={conversationName} callState={callState} />
          )}
          
          {/* Local video PiP - always visible */}
          <LocalVideoPiP
            stream={localStream}
            isMuted={isMuted}
            isVideoOff={isVideoOff}
          />
        </div>

        {/* Controls */}
        <CallControls
          isMuted={isMuted}
          isVideoOff={isVideoOff}
          onToggleMute={onToggleMute}
          onToggleVideo={onToggleVideo}
          onEndCall={() => {
            onEndCall();
            onClose?.();
          }}
          onScreenShare={() => {
            // Screen sharing could be implemented here
            console.log('Screen share not yet implemented');
          }}
        />
      </div>
    </div>
  );
}
