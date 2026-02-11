import React from 'react';
import wsService from '../services/websocket';

// ICE configuration will be received from server
const DEFAULT_ICE_CONFIG = {
  iceServers: [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' }
  ]
};

/**
 * Custom hook for WebRTC video/audio calls
 * Supports both P2P mesh (1:1 calls) and SFU (group calls)
 */
export function useWebRTC(userId) {
  const [isInCall, setIsInCall] = React.useState(false);
  const [callRoomId, setCallRoomId] = React.useState(null);
  const [callMode, setCallMode] = React.useState('p2p'); // 'p2p' or 'sfu'
  const [participants, setParticipants] = React.useState([]);
  const [localStream, setLocalStream] = React.useState(null);
  const [remoteStreams, setRemoteStreams] = React.useState({});
  const [isMuted, setIsMuted] = React.useState(false);
  const [isVideoOff, setIsVideoOff] = React.useState(false);
  const [iceServers, setIceServers] = React.useState(DEFAULT_ICE_CONFIG.iceServers);
  const [error, setError] = React.useState(null);
  const [callState, setCallState] = React.useState('idle'); // idle, connecting, connected, reconnecting
  
  // Incoming call state
  const [incomingCall, setIncomingCall] = React.useState(null); // { callId, conversationId, caller, callType, isGroup, conversationName }
  
  const peerConnections = React.useRef(new Map());
  const pendingCandidates = React.useRef(new Map()); // Store ICE candidates until connection is ready
  const processedCandidates = React.useRef(new Set()); // Deduplication set for ICE candidates
  
  // CRITICAL: Use refs for values that need to be accessed in callbacks without stale closures
  const localStreamRef = React.useRef(null);
  const callRoomIdRef = React.useRef(null);
  const iceServersRef = React.useRef(DEFAULT_ICE_CONFIG.iceServers);
  const callModeRef = React.useRef('p2p');
  
  // Perfect Negotiation Refs
  const ignoreOfferRef = React.useRef(false);
  const isPoliteRef = React.useRef(false);
  const makingOfferRef = React.useRef(false); 
  const isSettingRemoteAnswerPendingRef = React.useRef(false);
  
  // Track if we've ever had other participants (for proper call ending)
  const hasEverHadParticipants = React.useRef(false);
  const maxParticipantCount = React.useRef(0);
  
  // Keep refs in sync with state
  React.useEffect(() => { localStreamRef.current = localStream; }, [localStream]);
  React.useEffect(() => { callRoomIdRef.current = callRoomId; }, [callRoomId]);
  React.useEffect(() => { iceServersRef.current = iceServers; }, [iceServers]);
  React.useEffect(() => { callModeRef.current = callMode; }, [callMode]);
  
  // Ref for isVideoOff state (needed in migration handler's setTimeout callback)
  const isVideoOffRef = React.useRef(false);
  React.useEffect(() => { isVideoOffRef.current = isVideoOff; }, [isVideoOff]);
  
  // Track max participants we've seen
  React.useEffect(() => {
    const otherCount = participants.filter(p => p.user_id !== userId).length;
    if (otherCount > 0) {
      hasEverHadParticipants.current = true;
      maxParticipantCount.current = Math.max(maxParticipantCount.current, otherCount);
    }
  }, [participants, userId]);
  
  // SFU: Single peer connection to server
  const sfuConnection = React.useRef(null);
  const sfuPendingCandidates = React.useRef([]);

  // Clean up peer connection (P2P mode)
  const closePeerConnection = React.useCallback((peerId) => {
    const pc = peerConnections.current.get(peerId);
    if (pc) {
      pc.close();
      peerConnections.current.delete(peerId);
      setRemoteStreams(prev => {
        const next = { ...prev };
        delete next[peerId];
        return next;
      });
    }
    pendingCandidates.current.delete(peerId);
  }, []);

  // Close SFU connection
  const closeSFUConnection = React.useCallback(() => {
    if (sfuConnection.current) {
      sfuConnection.current.close();
      sfuConnection.current = null;
    }
    sfuPendingCandidates.current = [];
    setRemoteStreams({});
  }, []);

  // Create SFU peer connection (one connection to server that handles all tracks)
  // Uses refs to avoid stale closures
  const createSFUConnection = React.useCallback(async (stream) => {
    console.log('Creating SFU connection');
    console.log('Stream provided:', !!stream, 'tracks:', stream?.getTracks().length);
    
    const currentIceServers = iceServersRef.current;
    const pc = new RTCPeerConnection({ iceServers: currentIceServers });
    sfuConnection.current = pc;

    // Add local tracks to connection
    const streamToUse = stream || localStreamRef.current;
    if (streamToUse) {
      console.log('Adding tracks to SFU connection:', streamToUse.getTracks().map(t => t.kind));
      streamToUse.getTracks().forEach(track => {
        pc.addTrack(track, streamToUse);
      });
    } else {
      console.error('CRITICAL: No stream available for SFU connection');
    }

    // Handle ICE candidates - send to server using ref
    pc.onicecandidate = (event) => {
      if (event.candidate) {
        const roomId = callRoomIdRef.current;
        if (roomId) {
          wsService.send('sfu.candidate', {
            room_id: roomId,
            candidate: event.candidate.toJSON()
          });
        }
      }
    };

    // Handle remote tracks from SFU (tracks from other participants)
    pc.ontrack = (event) => {
      console.log('🎉 SFU ONTRACK - Received remote track:', event.track.kind, 'streamId:', event.streams[0]?.id);
      const stream = event.streams[0];
      if (!stream) return;
      
      const streamId = stream.id;
      
      setRemoteStreams(prev => {
        // Check if we already have this stream
        const existing = Object.values(prev).find(v => v.stream?.id === streamId);
        if (existing) {
          return prev;
        }
        
        // For SFU, we store streams by stream ID until we get track info
        return {
          ...prev,
          [streamId]: {
            stream,
            username: 'Participant' // Will be updated when we get sfu.tracks
          }
        };
      });
    };

    // Handle connection state changes
    pc.onconnectionstatechange = () => {
      console.log('SFU connection state:', pc.connectionState);
      if (pc.connectionState === 'connected') {
        console.log('✅ SFU connection established');
      } else if (pc.connectionState === 'failed') {
        console.error('❌ SFU connection failed');
        setError('Connection to server failed');
      }
    };

    // Handle ICE connection state
    pc.oniceconnectionstatechange = async () => {
      console.log('SFU ICE connection state:', pc.iceConnectionState);
      if (pc.iceConnectionState === 'failed') {
        console.error('SFU ICE connection failed, attempting restart...');
        try {
          const roomId = callRoomIdRef.current;
          if (!roomId) {
            console.warn('Cannot restart ICE for SFU, no room ID.');
            return;
          }

          if (pc.restartIce) {
            pc.restartIce();
          } else {
            // Fallback for older browsers or if restartIce not available
            const offer = await pc.createOffer({ iceRestart: true });
            await pc.setLocalDescription(offer);
            wsService.send('sfu.offer', {
              room_id: roomId,
              sdp: pc.localDescription.sdp
            });
          }
        } catch (err) {
          console.error('Failed to restart ICE for SFU:', err);
        }
      }
    };

    return pc;
  }, []);

  // Create a peer connection for a specific user
  // CRITICAL: Uses refs to avoid stale closure issues with localStream
  const createPeerConnection = React.useCallback((peerId, peerName, isInitiator) => {
    if (peerConnections.current.has(peerId)) {
      console.log('Peer connection already exists for:', peerId);
      return peerConnections.current.get(peerId);
    }

    // Use refs to get current values (avoids stale closures)
    const currentStream = localStreamRef.current;
    const currentIceServers = iceServersRef.current;

    console.log('Creating peer connection for:', peerId, 'isInitiator:', isInitiator);
    console.log('Local stream available:', !!currentStream, 'tracks:', currentStream?.getTracks().length);
    
    const pc = new RTCPeerConnection({ iceServers: currentIceServers });
    peerConnections.current.set(peerId, pc);

    // Add local tracks to connection
    if (currentStream) {
      console.log('Adding local tracks to peer connection for:', peerId);
      currentStream.getTracks().forEach(track => {
        console.log('Adding track:', track.kind, track.id, 'to peer:', peerId);
        pc.addTrack(track, currentStream);
      });
    } else {
      console.error('CRITICAL: No local stream available when creating peer connection for:', peerId);
      // This should not happen - log state for debugging
      console.error('State debug - localStream:', localStream, 'localStreamRef.current:', localStreamRef.current);
    }

    // Handle ICE candidates - use ref for roomId to avoid stale closure
    pc.onicecandidate = (event) => {
      if (event.candidate) {
        const roomId = callRoomIdRef.current;
        if (roomId) {
          wsService.send('call.ice_candidate', {
            room_id: roomId,
            target_id: peerId,
            candidate: JSON.stringify(event.candidate)
          });
        }
      }
    };

    // Handle ICE connection state changes (more granular than connectionstatechange)
    pc.oniceconnectionstatechange = async () => {
      console.log('ICE connection state for', peerId, ':', pc.iceConnectionState);
      if (pc.iceConnectionState === 'connected' || pc.iceConnectionState === 'completed') {
        console.log('✅ ICE connection established with:', peerId);
      } else if (pc.iceConnectionState === 'failed') {
        console.error('❌ ICE connection failed with:', peerId);
        // Attempt ICE restart — standard WebRTC recovery for transient network issues
        console.log('Attempting ICE restart for:', peerId);
        try {
          const restartOffer = await pc.createOffer({ iceRestart: true });
          await pc.setLocalDescription(restartOffer);
          const roomId = callRoomIdRef.current;
          if (roomId) {
            wsService.send('call.offer', {
              room_id: roomId,
              target_id: peerId,
              sdp: JSON.stringify(restartOffer)
            });
            console.log('ICE restart offer sent to:', peerId);
          }
        } catch (err) {
          console.error('ICE restart failed:', err);
        }
      }
    };

    // Handle remote stream - this is the key event for receiving video/audio
    pc.ontrack = (event) => {
      console.log('🎉 ONTRACK FIRED - Received remote track from:', peerId, 
        'kind:', event.track.kind, 
        'streams:', event.streams.length);
      
      if (event.streams && event.streams[0]) {
        setRemoteStreams(prev => ({
          ...prev,
          [peerId]: {
            stream: event.streams[0],
            username: peerName
          }
        }));
      }
    };

    // Handle connection state changes
    pc.onconnectionstatechange = () => {
      console.log('Connection state for', peerId, ':', pc.connectionState);
      if (pc.connectionState === 'connected') {
        console.log('✅ Peer connection established with:', peerId);
      } else if (pc.connectionState === 'failed') {
        console.error('❌ Peer connection failed with:', peerId);
        // Could implement reconnection logic here
      } else if (pc.connectionState === 'disconnected') {
        console.warn('⚠️ Peer connection disconnected from:', peerId);
      }
    };

    // Log negotiation needed events
    pc.onnegotiationneeded = async () => {
      console.log('Negotiation needed for peer:', peerId, 'isInitiator:', isInitiator);
      if (callModeRef.current === 'sfu') return; // Only for P2P
      
      // Perfect negotiation logic
      try {
          makingOfferRef.current = true;
          await pc.setLocalDescription(); // Create and set local offer
          
          // Send offer to signaling server
          const roomId = callRoomIdRef.current;
          if (roomId) {
            wsService.send('call.offer', {
                room_id: roomId,
                sdp: JSON.stringify(pc.localDescription),
                target_id: peerId
            });
          }
      } catch (err) {
          console.error('Error during negotiation:', err);
      } finally {
          makingOfferRef.current = false;
      }
    };

    // Process any pending ICE candidates
    const pending = pendingCandidates.current.get(peerId);
    if (pending && pending.length > 0) {
      console.log('Processing', pending.length, 'pending ICE candidates for:', peerId);
      pending.forEach(candidate => {
        pc.addIceCandidate(new RTCIceCandidate(candidate))
          .catch(err => console.error('Failed to add pending ICE candidate:', err));
      });
      pendingCandidates.current.delete(peerId);
    }

    return pc;
  }, [localStream]); // Only depend on localStream for re-creation logging, actual values come from refs

  // Track cleanup timers to prevent race conditions
  const cleanupTimersRef = React.useRef({
    participantLeft: null,
    autoEnd: null,
    endedDelay: null
  });
  
  // Guard against concurrent join attempts
  const isJoiningRef = React.useRef(false);

  // Helper to clear all pending cleanup timers
  const clearCleanupTimers = React.useCallback(() => {
    if (cleanupTimersRef.current.participantLeft) {
      clearTimeout(cleanupTimersRef.current.participantLeft);
      cleanupTimersRef.current.participantLeft = null;
    }
    if (cleanupTimersRef.current.autoEnd) {
      clearTimeout(cleanupTimersRef.current.autoEnd);
      cleanupTimersRef.current.autoEnd = null;
    }
    if (cleanupTimersRef.current.endedDelay) {
      clearTimeout(cleanupTimersRef.current.endedDelay);
      cleanupTimersRef.current.endedDelay = null;
    }
  }, []);

  // Helper to stop media tracks and cleanup state
  const cleanupMedia = React.useCallback(() => {
    // Stop local stream tracks
    const stream = localStreamRef.current;
    if (stream) {
      console.log('Stopping local media tracks');
      stream.getTracks().forEach(track => {
        try {
          track.enabled = false;
          track.stop();
        } catch (e) {
          console.warn('Error stopping track:', e);
        }
      });
      setLocalStream(null);
      localStreamRef.current = null;
    }
  }, []);

  // Initialize local media stream
  const getLocalMedia = React.useCallback(async (video = true, retryCount = 0) => {
    try {
      console.log(`Requesting media permissions...${retryCount > 0 ? ` (Attempt ${retryCount + 1})` : ''}`);
      const stream = await navigator.mediaDevices.getUserMedia({
        video: video ? { width: 640, height: 480 } : false,
        audio: true
      });
      console.log('Media stream obtained:', stream.id, 'tracks:', stream.getTracks().map(t => t.kind));
      
      // Update both state and ref immediately
      localStreamRef.current = stream;
      setLocalStream(stream);
      return stream;
    } catch (err) {
      console.error('Failed to get local media:', err);
      
      // Provide specific error messages based on the error type
      let errorMessage = 'Could not access camera/microphone';
      
      if (err.name === 'NotAllowedError' || err.name === 'PermissionDeniedError') {
        errorMessage = 'Camera/microphone permission denied. Please allow access and try again.';
      } else if (err.name === 'NotFoundError' || err.name === 'DevicesNotFoundError') {
        errorMessage = 'No camera or microphone found. Please connect a device and try again.';
      } else if (err.name === 'NotReadableError' || err.name === 'TrackStartError') {
        if (retryCount < 2) {
          console.warn('Camera/mic in use, retrying in 1s...');
          await new Promise(resolve => setTimeout(resolve, 1000));
          return getLocalMedia(video, retryCount + 1);
        }
        errorMessage = 'Camera/microphone is already in use by another application.';
      } else if (err.name === 'OverconstrainedError') {
        errorMessage = 'Camera does not support the requested resolution.';
      } else if (err.name === 'AbortError') {
        errorMessage = 'Could not start audio source. Please check your microphone settings.';
      }
      
      setError(errorMessage);
      throw new Error(errorMessage);
    }
  }, []);

  // Try to get media with fallback options
  const getLocalMediaWithFallback = React.useCallback(async (preferVideo = true) => {
    // Try with video first
    if (preferVideo) {
      try {
        return await getLocalMedia(true);
      } catch (err) {
        console.warn('Failed to get video, trying audio only:', err.message);
        // Fall through to audio-only attempt
      }
    }
    
    // Try audio only
    try {
      console.log('Attempting audio-only call...');
      const stream = await navigator.mediaDevices.getUserMedia({
        video: false,
        audio: true
      });
      console.log('Audio-only stream obtained:', stream.id);
      localStreamRef.current = stream;
      setLocalStream(stream);
      return stream;
    } catch (err) {
      console.error('Failed to get audio:', err);
      
      // Last resort: try with any available device
      try {
        console.log('Attempting with any available device...');
        const stream = await navigator.mediaDevices.getUserMedia({
          video: preferVideo,
          audio: { echoCancellation: false, noiseSuppression: false, autoGainControl: false }
        });
        localStreamRef.current = stream;
        setLocalStream(stream);
        return stream;
      } catch (finalErr) {
        let errorMessage = 'Could not access any media device.';
        if (finalErr.name === 'NotAllowedError') {
          errorMessage = 'Permission denied. Please allow camera/microphone access in your browser settings.';
        }
        setError(errorMessage);
        throw new Error(errorMessage);
      }
    }
  }, [getLocalMedia]);

  // Join a call
  const joinCall = React.useCallback(async (roomId, videoEnabled = true) => {
    // Prevent concurrent joins
    if (isJoiningRef.current) {
      console.warn('Join call ignored - already joining');
      return;
    }
    isJoiningRef.current = true;

    try {
      // Clear any pending auto-end timers from previous calls
      clearCleanupTimers();

      // Ensure we start with a clean slate
      cleanupMedia();
      
      // Small delay to allow OS to release hardware devices from previous call
      await new Promise(resolve => setTimeout(resolve, 750));

      setCallState('connecting');
      setError(null);
      
      // Reset tracking refs for new call
      hasEverHadParticipants.current = false;
      maxParticipantCount.current = 0;
      ignoreOfferRef.current = false; // Reset glare state
      isPoliteRef.current = false; // Reset politeness

      // Set roomId in both state and ref immediately
      setCallRoomId(roomId);
      callRoomIdRef.current = roomId;

      // Get local media first with fallback - this must succeed before we join
      console.log('Requesting media permissions for room:', roomId);
      const stream = await getLocalMediaWithFallback(videoEnabled);
      console.log('Media permissions granted, stream:', stream.id, 'ref set:', !!localStreamRef.current);
      
      // Check if we got video or audio-only
      const hasVideo = stream.getVideoTracks().length > 0;
      setIsVideoOff(!hasVideo);

      // CRITICAL: Ensure refs are set before sending join
      // This prevents race conditions where call.config arrives before refs are ready
      await new Promise(resolve => setTimeout(resolve, 50));
      
      console.log('Pre-join check - localStreamRef:', !!localStreamRef.current, 'callRoomIdRef:', callRoomIdRef.current);

      // Join via WebSocket only after media is ready
      console.log('Sending call.join to server for room:', roomId);
      wsService.send('call.join', { room_id: roomId });
    } catch (err) {
      console.error('Failed to join call:', err);
      setCallState('idle');
      setCallRoomId(null);
      callRoomIdRef.current = null;
      // Error is already set by getLocalMediaWithFallback
      // Rethrow so acceptCall can handle it
      throw err;
    } finally {
      isJoiningRef.current = false;
    }
  }, [getLocalMediaWithFallback, cleanupMedia, clearCleanupTimers]);

  // Leave the call
  const leaveCall = React.useCallback(() => {
    console.log('leaveCall called');
    isJoiningRef.current = false;
    
    // Clear any pending timers
    clearCleanupTimers();

    // Send leave message
    if (callRoomIdRef.current) {
      if (callModeRef.current === 'sfu') {
        wsService.send('sfu.leave', { room_id: callRoomIdRef.current });
      } else {
        wsService.send('call.leave', { room_id: callRoomIdRef.current });
      }
    }

    // Stop local stream
    cleanupMedia();

    // Close connections based on mode
    if (callModeRef.current === 'sfu') {
      closeSFUConnection();
    } else {
      // Close all P2P peer connections
      peerConnections.current.forEach((pc, peerId) => {
        closePeerConnection(peerId);
      });
    }

    // Reset all tracking refs
    hasEverHadParticipants.current = false;
    maxParticipantCount.current = 0;
    ignoreOfferRef.current = false;
    isPoliteRef.current = false;
    makingOfferRef.current = false;
    isSettingRemoteAnswerPendingRef.current = false;

    setIsInCall(false);
    setCallRoomId(null);
    callRoomIdRef.current = null;
    setCallMode('p2p');
    callModeRef.current = 'p2p';
    setParticipants([]);
    setRemoteStreams({});
    setCallState('idle');
  }, [closePeerConnection, closeSFUConnection, cleanupMedia, clearCleanupTimers]);

  // Toggle mute
  const toggleMute = React.useCallback(() => {
    if (localStream) {
      const audioTrack = localStream.getAudioTracks()[0];
      if (audioTrack) {
        const newEnabled = !audioTrack.enabled;
        audioTrack.enabled = newEnabled;
        setIsMuted(!newEnabled);
        
        // Signal mute state to others
        if (callRoomIdRef.current) {
          wsService.send('call.mute_update', {
            room_id: callRoomIdRef.current,
            kind: 'audio',
            muted: !newEnabled
          });
        }
      }
    }
  }, [localStream]);

  // Toggle video
  const toggleVideo = React.useCallback(() => {
    if (localStream) {
      const videoTrack = localStream.getVideoTracks()[0];
      if (videoTrack) {
        const newEnabled = !videoTrack.enabled;
        videoTrack.enabled = newEnabled;
        setIsVideoOff(!newEnabled);
        
        // Signal video state to others
        if (callRoomIdRef.current) {
          wsService.send('call.mute_update', {
            room_id: callRoomIdRef.current,
            kind: 'video',
            muted: !newEnabled
          });
        }
      }
    }
  }, [localStream]);

  // Handle SFU Device Switching
  React.useEffect(() => {
      const handleDeviceSwitch = async () => {
          if (callModeRef.current !== 'sfu' || !sfuConnection.current || !localStream) return;
          
          console.log('SFU: Local stream changed (device switch), updating tracks...');
          const pc = sfuConnection.current;
          const senders = pc.getSenders();
          
          for (const track of localStream.getTracks()) {
              const sender = senders.find(s => s.track && s.track.kind === track.kind);
              if (sender) {
                  console.log(`SFU: Replacing ${track.kind} track`);
                  try {
                    await sender.replaceTrack(track);
                  } catch (err) {
                      console.error('SFU: Failed to replace track:', err);
                  }
              } else {
                  // If no sender exists, we might need to addTrack (renegotiation)
                  // But for simple camera switch, replaceTrack is usually enough if we started with video
                  console.warn(`SFU: No sender found for ${track.kind} track`);
              }
          }
      };
      
      handleDeviceSwitch();
  }, [localStream]);

  // Handle incoming call config (after joining)
  React.useEffect(() => {
    const handleCallConfig = async (payload) => {
      console.log('=== CALL CONFIG RECEIVED ===');
      console.log('Payload:', payload);
      console.log('Local userId:', userId);
      console.log('Participants from backend:', payload.participants);
      console.log('Local stream ref available:', !!localStreamRef.current);
      
      // Update ICE servers ref immediately with sanitization
      let newIceServers = payload.ice_servers || [];
      
      // If we are not on localhost, filter out internal docker hostnames like 'coturn'
      const isLocalhost = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1';
      if (!isLocalhost) {
        console.log('Running remotely, filtering out internal ICE servers...');
        newIceServers = newIceServers.filter(server => {
          const urls = Array.isArray(server.urls) ? server.urls : [server.urls];
          // Keep server only if NONE of its URLs contain 'coturn'
          return !urls.some(url => url.includes('coturn'));
        });
      }

      // Ensure default public STUN servers are always present
      const defaultStun = DEFAULT_ICE_CONFIG.iceServers;
      const hasStun = newIceServers.some(server => {
        const urls = Array.isArray(server.urls) ? server.urls : [server.urls];
        return urls.some(url => url.includes('stun.l.google.com'));
      });

      if (!hasStun) {
        console.log('Adding default public STUN servers');
        newIceServers = [...newIceServers, ...defaultStun];
      }
      
      console.log('Sanitized ICE servers:', newIceServers);
      iceServersRef.current = newIceServers;
      setIceServers(newIceServers);
      
      setParticipants(payload.participants || []);
      setIsInCall(true);
      setCallState('connected');
      
      // Check if this is SFU mode
      const mode = payload.mode || 'p2p';
      setCallMode(mode);
      callModeRef.current = mode; // Keep ref in sync

      // Determine Politeness for P2P calls
      if (mode === 'p2p') {
        // Find the other participant (assuming 1:1 P2P for now)
        const myId = userId;
        const otherPart = (payload.participants || []).find(p => p.user_id !== myId);
        if (otherPart) {
             // Lexicographical comparison: Lower ID = Polite (Yields)
             // Higher ID = Impolite (Persists)
             const isPolite = myId < otherPart.user_id;
             isPoliteRef.current = isPolite;
             console.log("Determined politeness:", isPolite ? 'Polite' : 'Impolite', "My ID:", myId, "Other:", otherPart.user_id);
        } else {
          // If no other participant, we are effectively the initiator and can be impolite by default
          isPoliteRef.current = false;
          console.log("No other participant found, defaulting to Impolite.");
        }
      }
      
      if (mode === 'sfu') {
        // SFU mode: wait for sfu.offer from server
        console.log('SFU mode: initializing connection');
        
        // Create SFU connection with local stream
        // Pass payload.sdp if available (optimization to avoid race condition)
        const pc = await createSFUConnection(localStreamRef.current);
        
        if (payload.sdp) {
            console.log('Received initial SFU offer in config, processing immediately...');
            try {
                const sdp = payload.sdp;
                console.log('Setting SFU remote description (initial offer)...');
                await pc.setRemoteDescription(new RTCSessionDescription({
                  type: 'offer',
                  sdp: sdp
                }));
                
                // Process any pending ICE candidates that might have arrived
                for (const candidate of sfuPendingCandidates.current) {
                  await pc.addIceCandidate(new RTCIceCandidate(candidate));
                }
                sfuPendingCandidates.current = [];
                
                const answer = await pc.createAnswer();
                await pc.setLocalDescription(answer);
                
                wsService.send('sfu.answer', {
                  room_id: payload.room_id,
                  sdp: answer.sdp
                });
                console.log('Sent initial SFU answer');
            } catch (err) {
                console.error('Failed to handle initial SFU offer:', err);
            }
        } else {
            console.log('No initial SFU offer in config, waiting for sfu.offer event...');
        }
      } else {
        // P2P mode:
        // Initialize connections for all existing peers
        // Ensure strictly lexicographical politeness
        const myId = userId;
        const otherParticipants = (payload.participants || []).filter(p => p.user_id !== myId);
        
        console.log('P2P mode - Other participants:', otherParticipants);
        
        // Initialize connections for all existing peers
        for (const participant of otherParticipants) {
            console.log('Initializing peer connection for:', participant.user_id);
            // Initiate connection. If we are IMPOLITE (Initiator logic), we might start negotiation.
            // If we are POLITE (Joiner), we might also trigger negotiation but will yield if glarin.
            createPeerConnection(participant.user_id, participant.username, payload.is_initiator);
        }
      }
    };

    const unsubConfig = wsService.on('call.config', handleCallConfig);
    return () => unsubConfig();
  }, [userId, createPeerConnection, createSFUConnection]);

  // Handle mute updates
  React.useEffect(() => {
      const handleMuteUpdate = (payload) => {
          console.log('Received mute update:', payload);
          setParticipants(prev => prev.map(p => {
              // Note: payload structure depends on backend, assuming { user_id, kind, muted } added to payload by sender or broadcast?
              // Looking at protocol.go, the EventType is call.mute_update.
              // The payload sent by toggleMute has { room_id, kind, muted }.
              // The backend needs to relay this WITH user_id.
              // Assuming backend uses generic relay or we need to update backend too?
              // Wait, protocol.go doesn't define a specific payload struct for this, so it might just be the raw map?
              // If backend relays it as-is, it won't have user_id unless backend adds it or we send it.
              // We should probably rely on `from_id` if using the generic relay, but `wsService` might strip it?
              // The `wsService` usually gives us the parsed payload.
              // Let's assume standard broadcast behavior sets `from_id` or we can't identify who muted.
              // Actually, existing events like `call.offer` have `from_id`.
              // Standard relay usually includes `user_id` or `from_id`.
              // We'll check payload properties.
              
              // If payload uses `user_id` (like `participant_joined`) or `from_id` (like `call.offer`)
              const targetId = payload.user_id || payload.from_id;
              
              if (p.user_id === targetId) {
                  return {
                      ...p,
                      isMuted: payload.kind === 'audio' ? payload.muted : p.isMuted,
                      isVideoOff: payload.kind === 'video' ? payload.muted : p.isVideoOff
                  };
              }
              return p;
          }));
      };
      
      const unsubMute = wsService.on('call.mute_update', handleMuteUpdate);
      return () => unsubMute();
  }, []);

  // Handle participant joined
  React.useEffect(() => {
    const handleParticipantJoined = (payload) => {
      console.log('Participant joined:', payload);
      if (payload.user_id === userId) return; // Ignore self
      
      // Mark that we've had participants join (for call ending logic)
      hasEverHadParticipants.current = true;
      maxParticipantCount.current = Math.max(maxParticipantCount.current, 1);
      
      setParticipants(prev => {
        if (prev.find(p => p.user_id === payload.user_id)) return prev;
        return [...prev, { user_id: payload.user_id, username: payload.username }];
      });

      // P2P: Initialize peer connection for the new participant immediately
      // This will assume WE are the impolite peer (existing user) vs the polite peer (joiner)
      // Actually, we use ID-based politeness or just let 'perfect negotiation' handle it.
      // But we MUST create the PC to be able to receive/send offers.
      if (callModeRef.current !== 'sfu') {
          console.log('P2P: New participant joined, creating connection:', payload.user_id);
          createPeerConnection(payload.user_id, payload.username, true); 
      }
    };

    const unsubJoined = wsService.on('call.participant_joined', handleParticipantJoined);
    return () => unsubJoined();
  }, [userId, createPeerConnection]);

  // Handle participant left
  React.useEffect(() => {
    const handleParticipantLeft = (payload) => {
      console.log('Participant left:', payload);
      
      if (callMode === 'sfu') {
        // In SFU mode, finding the stream by user_id and removing it
        setRemoteStreams(prev => {
          const next = { ...prev };
          // Try to find by userId (set by sfu.tracks handler)
          let streamIdToRemove = Object.keys(next).find(
            key => next[key].userId === payload.user_id
          );
          
          // Fallback: if sfu.tracks hasn't mapped this stream yet,
          // check if there's an unmatched stream that's no longer active
          if (!streamIdToRemove) {
            // Remove any orphaned streams - streams whose userId doesn't match
            // any remaining participant
            const remainingUserIds = new Set(
              participants
                .filter(p => p.user_id !== payload.user_id)
                .map(p => p.user_id)
            );
            for (const key of Object.keys(next)) {
              if (next[key].userId && !remainingUserIds.has(next[key].userId)) {
                streamIdToRemove = key;
                break;
              }
            }
          }
          
          if (streamIdToRemove) {
            console.log('Removing SFU stream for user:', payload.user_id);
            delete next[streamIdToRemove];
          }
          return next;
        });
      } else {
        // P2P mode
        closePeerConnection(payload.user_id);
      }

      setParticipants(prev => {
        const newParticipants = prev.filter(p => p.user_id !== payload.user_id);
        // Count remaining participants (excluding ourselves)
        const othersRemaining = newParticipants.filter(p => p.user_id !== userId);
        console.log('After participant left - others remaining:', othersRemaining.length);
        
        // For P2P calls: if someone was in call and left, end the call for remaining user
        if (hasEverHadParticipants.current && othersRemaining.length === 0) {
          console.log('Last participant left P2P call - ending call after delay');
          
          // Clear any existing timer
          if (cleanupTimersRef.current.participantLeft) {
            clearTimeout(cleanupTimersRef.current.participantLeft);
          }
          
          cleanupTimersRef.current.participantLeft = setTimeout(() => {
            // Only end if we are still in the same room
            if (callRoomIdRef.current) {
              setCallState('ended');
              // Give user a moment to see the ended state before full cleanup
              cleanupTimersRef.current.endedDelay = setTimeout(() => leaveCall(), 2000);
            }
          }, 500);
        }
        
        return newParticipants;
      });
    };

    const unsubLeft = wsService.on('call.participant_left', handleParticipantLeft);
    return () => unsubLeft();
  }, [closePeerConnection, userId, leaveCall, callMode, participants]);

  // Track if we've ever had a successful connection (used to prevent premature call ending)
  const hasEverConnected = React.useRef(false);
  
  // Update hasEverConnected when we get remote streams
  React.useEffect(() => {
    if (Object.keys(remoteStreams).length > 0) {
      hasEverConnected.current = true;
    }
  }, [remoteStreams]);
  
  // Reset hasEverConnected when call ends
  React.useEffect(() => {
    if (!isInCall) {
      hasEverConnected.current = false;
    }
  }, [isInCall]);

  // Auto-end call when all other participants leave (only after we've connected at least once)
  React.useEffect(() => {
    if (!isInCall) return;
    
    // Don't auto-end if we've never had a connection yet (still waiting for others to join)
    if (!hasEverConnected.current) return;
    
    // Check if there are any other participants besides ourselves
    const othersInCall = participants.filter(p => p.user_id !== userId);
    
    // If we're in a call but no other participants, and we have no remote streams, end the call
    if (othersInCall.length === 0 && Object.keys(remoteStreams).length === 0 && callState === 'connected') {
      console.log('All other participants left, ending call');
      
      if (cleanupTimersRef.current.autoEnd) {
        clearTimeout(cleanupTimersRef.current.autoEnd);
      }
      
      cleanupTimersRef.current.autoEnd = setTimeout(() => {
        // Only proceed if still in the same state (user didn't join a new call in the meantime)
        const roomId = callRoomIdRef.current;
        if (roomId) {
          wsService.send('call.leave', { room_id: roomId });
        }
        
        // Use our cleanup helper
        cleanupMedia();
        
        peerConnections.current.forEach((pc, peerId) => {
          pc.close();
          peerConnections.current.delete(peerId);
        });
        setIsInCall(false);
        setCallRoomId(null);
        callRoomIdRef.current = null;
        setParticipants([]);
        setRemoteStreams({});
        setCallState('idle');
      }, 1000);
      
      const timers = cleanupTimersRef.current;
      return () => {
        if (timers.autoEnd) {
          clearTimeout(timers.autoEnd);
          timers.autoEnd = null;
        }
      };
    }
  }, [isInCall, participants, userId, remoteStreams, callState, cleanupMedia]);

  // Handle incoming offer - CRITICAL: This is where first user receives offer from second user
  React.useEffect(() => {
    const handleOffer = async (payload) => {
      console.log('=== RECEIVED OFFER ===');
      console.log('From:', payload.from_id, payload.from_name);
      console.log('Room:', payload.room_id);
      console.log('Local stream ref available:', !!localStreamRef.current);
      console.log('Local stream ref tracks:', localStreamRef.current?.getTracks().map(t => t.kind));
      
      if (callModeRef.current === 'sfu') return; // Only for P2P

      const pc = peerConnections.current.get(payload.from_id);
      if (!pc) {
        console.error('No peer connection for:', payload.from_id, 'when receiving offer.');
        return;
      }

      // Perfect Negotiation / Glare Fix
      const offerCollision = makingOfferRef.current || pc.signalingState !== 'stable';
      ignoreOfferRef.current = false; // Reset for this offer

      if (offerCollision) {
          if (!isPoliteRef.current) {
              // Impolite: Ignore their offer, keep trying to send ours
              ignoreOfferRef.current = true;
              console.log("Glare detected: Ignoring incoming offer (Impolite)");
              return;
          }
          // Polite: Rollback and accept their offer
          console.log("Glare detected: Yielding to incoming offer (Polite)");
          // Rollback local description if we have one
          if (pc.localDescription) {
            await pc.setLocalDescription({ type: 'rollback' });
          }
      }
      
      // If we are polite, or if there is no collision, proceed.
      try {
        const offer = JSON.parse(payload.sdp);
        console.log('Setting remote description (offer)...');
        isSettingRemoteAnswerPendingRef.current = true; // Indicate we are processing remote description
        await pc.setRemoteDescription(new RTCSessionDescription(offer));
        isSettingRemoteAnswerPendingRef.current = false;

        // Flush any ICE candidates that arrived after createPeerConnection
        // but before setRemoteDescription completed
        const pendingOffer = pendingCandidates.current.get(payload.from_id);
        if (pendingOffer && pendingOffer.length > 0) {
          console.log('Flushing', pendingOffer.length, 'pending ICE candidates after setRemoteDescription (offer)');
          for (const c of pendingOffer) {
            await pc.addIceCandidate(new RTCIceCandidate(c))
              .catch(err => console.error('Failed to add buffered ICE candidate:', err));
          }
          pendingCandidates.current.delete(payload.from_id);
        }
        
        console.log('Creating answer...');
        const answer = await pc.createAnswer();
        await pc.setLocalDescription(answer);
        
        console.log('Sending answer to:', payload.from_id);
        wsService.send('call.answer', {
          room_id: payload.room_id,
          target_id: payload.from_id,
          sdp: JSON.stringify(answer)
        });
        
        console.log('Answer sent successfully');
      } catch (err) {
        console.error('Failed to handle offer:', err);
        isSettingRemoteAnswerPendingRef.current = false;
      }
    };

    const unsubOffer = wsService.on('call.offer', handleOffer);
    return () => unsubOffer();
  }, [createPeerConnection]);

  // Handle incoming answer
  React.useEffect(() => {
    const handleAnswer = async (payload) => {
      console.log('Received answer from:', payload.from_id);
      if (callModeRef.current === 'sfu') return; // Only for P2P
      
      const pc = peerConnections.current.get(payload.from_id);
      if (!pc) {
        console.error('No peer connection for:', payload.from_id);
        return;
      }
      
      try {
        const answer = JSON.parse(payload.sdp);
        await pc.setRemoteDescription(new RTCSessionDescription(answer));
        
        // CRITICAL: Flush any ICE candidates that arrived before the answer
        const pendingAnswer = pendingCandidates.current.get(payload.from_id);
        if (pendingAnswer && pendingAnswer.length > 0) {
          console.log('Flushing', pendingAnswer.length, 'pending ICE candidates after setRemoteDescription (answer)');
          for (const c of pendingAnswer) {
            await pc.addIceCandidate(new RTCIceCandidate(c))
              .catch(err => console.error('Failed to add buffered ICE candidate:', err));
          }
          pendingCandidates.current.delete(payload.from_id);
        }
      } catch (err) {
        console.error('Failed to handle answer:', err);
      }
    };

    const unsubAnswer = wsService.on('call.answer', handleAnswer);
    return () => unsubAnswer();
  }, []);

  // Handle incoming ICE candidate
  React.useEffect(() => {
    const handleIceCandidate = async (payload) => {
      if (callModeRef.current === 'sfu') return; // Only for P2P

      // FIX: Deduplicate candidates
      const candidateKey = payload.candidate; // Raw JSON string is a good key
      if (processedCandidates.current.has(candidateKey)) {
          console.log('Ignoring duplicate ICE candidate');
          return;
      }
      processedCandidates.current.add(candidateKey);

      const pc = peerConnections.current.get(payload.from_id);
      const candidate = JSON.parse(payload.candidate);
      
      if (pc && pc.remoteDescription && !isSettingRemoteAnswerPendingRef.current) {
        try {
          // Ignore candidates if we are currently ignoring offers (glare state)
          if (ignoreOfferRef.current) {
               console.log("Ignoring candidate during glare resolution");
               return;
          }
          await pc.addIceCandidate(new RTCIceCandidate(candidate));
        } catch (err) {
          if (!ignoreOfferRef.current) {
            console.error('Failed to add ICE candidate:', err);
          }
        }
      } else {
        // Queue the candidate for later
        if (!pendingCandidates.current.has(payload.from_id)) {
          pendingCandidates.current.set(payload.from_id, []);
        }
        pendingCandidates.current.get(payload.from_id).push(candidate);
      }
    };

    const unsubIce = wsService.on('call.ice_candidate', handleIceCandidate);
    return () => unsubIce();
  }, []);

  // SFU: Handle offer from server
  React.useEffect(() => {
    const handleSFUOffer = async (payload) => {
      console.log('Received SFU offer');
      
      const pc = sfuConnection.current;
      if (!pc) {
        console.error('No SFU connection');
        return;
      }
      
      try {
        // Server sends raw SDP string, NOT a JSON object
        const sdp = payload.sdp;
        
        console.log('Setting SFU remote description (offer)...');
        await pc.setRemoteDescription(new RTCSessionDescription({
          type: 'offer',
          sdp: sdp
        }));
        
        // Process any pending ICE candidates
        for (const candidate of sfuPendingCandidates.current) {
          await pc.addIceCandidate(new RTCIceCandidate(candidate));
        }
        sfuPendingCandidates.current = [];
        
        const answer = await pc.createAnswer();
        await pc.setLocalDescription(answer);
        
        wsService.send('sfu.answer', {
          room_id: payload.room_id,
          sdp: answer.sdp // Send raw SDP string
        });
      } catch (err) {
        console.error('Failed to handle SFU offer:', err);
      }
    };

    const unsubSFUOffer = wsService.on('sfu.offer', handleSFUOffer);
    return () => unsubSFUOffer();
  }, []);

  // SFU: Handle answer from server
  React.useEffect(() => {
    const handleSFUAnswer = async (payload) => {
      console.log('handleSFUAnswer:', payload);
      const pc = sfuConnection.current;
      if (!pc) {
        console.warn('Received SFU answer but no peer connection');
        return;
      }

      try {
        const sdp = payload.sdp;
        console.log('Setting SFU remote description (answer)...');
        await pc.setRemoteDescription(new RTCSessionDescription({
          type: 'answer',
          sdp: sdp
        }));
        
        // Process any queued candidates
        while (sfuPendingCandidates.current.length > 0) {
          const candidate = sfuPendingCandidates.current.shift();
          try {
            await pc.addIceCandidate(new RTCIceCandidate(candidate));
            console.log('Added queued SFU candidate');
          } catch (e) {
            console.error('Error adding queued SFU candidate:', e);
          }
        }
      } catch (err) {
        console.error('Error handling SFU answer:', err);
      }
    };

    const unsubSFUAnswer = wsService.on('sfu.answer', handleSFUAnswer);
    return () => unsubSFUAnswer();
  }, []);

  // SFU: Handle ICE candidate from server
  React.useEffect(() => {
    const handleSFUCandidate = async (payload) => {
      const pc = sfuConnection.current;
      // Candidate is now sent as a JSON object (aligned with P2P format)
      const candidate = typeof payload.candidate === 'string' 
        ? JSON.parse(payload.candidate) 
        : payload.candidate;
      
      if (pc && pc.remoteDescription) {
        try {
          await pc.addIceCandidate(new RTCIceCandidate(candidate));
        } catch (err) {
          console.error('Failed to add SFU ICE candidate:', err);
        }
      } else {
        // Queue the candidate for later
        sfuPendingCandidates.current.push(candidate);
      }
    };

    const unsubSFUCandidate = wsService.on('sfu.candidate', handleSFUCandidate);
    return () => unsubSFUCandidate();
  }, []);

  // SFU: Handle track info updates (maps streams to user info)
  React.useEffect(() => {
    const handleSFUTracks = (payload) => {
      console.log('SFU tracks update:', payload);
      
      // Update remote streams with user info
      // Payload contains { tracks: [{ id, kind, user_id, username }] }
      // NOTE: backend now sends the ACTUAL WebRTC Track ID in `id`.
      
      const tracks = payload.tracks || [];
      const trackMap = new Map(); // TrackID -> UserInfo
      tracks.forEach(t => trackMap.set(t.id, { userId: t.user_id, username: t.username }));
      
      setRemoteStreams(prev => {
        const updated = { ...prev };
        let hasChanges = false;
        
        // Iterate through all our active streams
        for (const [streamId, streamData] of Object.entries(updated)) {
            // Check each track in this stream to see if it matches one of our identified tracks
            const stream = streamData.stream;
            if (!stream) continue;
            
            for (const track of stream.getTracks()) {
                const info = trackMap.get(track.id);
                if (info) {
                    // Match found! Map this stream to this user
                    if (updated[streamId].userId !== info.userId) {
                        updated[streamId] = {
                            ...updated[streamId],
                            userId: info.userId,
                            username: info.username
                        };
                        hasChanges = true;
                    }
                }
            }
        }
        
        return hasChanges ? updated : prev;
      });
    };

    const unsubSFUTracks = wsService.on('sfu.tracks', handleSFUTracks);
    return () => unsubSFUTracks();
  }, []);

  // Handle incoming call notification
  React.useEffect(() => {
    const handleIncomingCall = (payload) => {
      console.log('Incoming call:', payload);
      
      // Don't show incoming call if already in a call
      if (isInCall) {
        console.log('Ignoring incoming call - already in a call');
        return;
      }

      setIncomingCall({
        callId: payload.call_id,
        conversationId: payload.conversation_id,
        caller: {
          id: payload.caller_id,
          username: payload.caller_name,
          avatar_url: payload.caller_avatar
        },
        callType: payload.call_type,
        isGroup: payload.is_group,
        conversationName: payload.conversation_name
      });
    };

    const handleCallCancelled = (payload) => {
      // Caller cancelled the call
      if (incomingCall?.callId === payload.call_id) {
        setIncomingCall(null);
      }
    };

    const handleCallEnded = (payload) => {
      console.log('Call ended event received:', payload);
      // Clear incoming call notification if applicable
      if (incomingCall?.callId === payload.call_id) {
        setIncomingCall(null);
      }
      
      // If we're in this call, end it
      if (isInCall && callRoomIdRef.current === payload.room_id) {
        console.log('Our active call was ended by server/other participant');
        setCallState('ended');
        
        // Use ref to track this timeout so it can be cancelled if a new call starts
        if (cleanupTimersRef.current.endedDelay) {
          clearTimeout(cleanupTimersRef.current.endedDelay);
        }
        
        cleanupTimersRef.current.endedDelay = setTimeout(() => leaveCall(), 1500);
      }
    };

    const handleCallDeclined = (payload) => {
      console.log('Call declined event received:', payload);
      // If we are the caller (in call) and the call was declined
      if (isInCall && callRoomIdRef.current === payload.conversation_id) {
         console.log('Our call was declined by', payload.decliner_name);
         setCallState('ended');
         
         // Use ref to track this timeout so it can be cancelled
         if (cleanupTimersRef.current.endedDelay) {
           clearTimeout(cleanupTimersRef.current.endedDelay);
         }
         
         cleanupTimersRef.current.endedDelay = setTimeout(() => leaveCall(), 1000); 
      }
    };

    const handleCallMigration = (payload) => {
      console.log('Call migration requested:', payload);
      // Verify we are in the correct room
      if (isInCall && callRoomIdRef.current === payload.room_id) {
         console.log('Migrating call to Group/SFU mode...');
         
         // 1. Leave current P2P call (cleans up peer connections and state)
         leaveCall();
         
         // 2. Re-join after a short delay to allow cleanup to finish
         // The new join will be routed to SFU by the backend because of the split-brain fix
         // Capture isVideoOff via ref to avoid stale closure in setTimeout
         const videoEnabled = !isVideoOffRef.current;
         setTimeout(() => {
           console.log('Re-joining call in SFU mode...');
           joinCall(payload.room_id, videoEnabled);
         }, 800);
      }
    };

    const unsubIncoming = wsService.on('call.incoming', handleIncomingCall);
    const unsubCancelled = wsService.on('call.cancelled', handleCallCancelled);
    const unsubEnded = wsService.on('call.ended', handleCallEnded);
    const unsubDeclined = wsService.on('call.declined', handleCallDeclined);
    const unsubMigration = wsService.on('call.migration', handleCallMigration);

    return () => {
      unsubIncoming();
      unsubCancelled();
      unsubEnded();
      unsubDeclined();
      unsubMigration();
    };
  }, [isInCall, incomingCall, leaveCall, userId, joinCall]);

  // Accept incoming call
  const acceptCall = React.useCallback(async (withVideo = true) => {
    if (!incomingCall) return;

    // Optimistically clear the incoming call UI to prevent double-clicks
    const callInfo = { ...incomingCall };
    setIncomingCall(null);

    try {
      // Join the call room
      await joinCall(callInfo.conversationId, withVideo);
    } catch (err) {
      console.error('Failed to accept call:', err);
      setError(err.message);
      // We don't restore incomingCall here to prevent getting stuck in a loop if media fails
    }
  }, [incomingCall, joinCall]);

  // Decline incoming call
  const declineCall = React.useCallback(() => {
    if (!incomingCall) return;

    // Send decline event
    wsService.send('call.declined', {
      call_id: incomingCall.callId,
      conversation_id: incomingCall.conversationId
    });

    setIncomingCall(null);
  }, [incomingCall]);

  // Handle WebSocket connection changes (Reconnection Logic)
  React.useEffect(() => {
    const handleConnectionChange = (payload) => {
      console.log('WebSocket connection status changed:', payload.status);
      
      if (payload.status === 'connected') {
        const currentRoomId = callRoomIdRef.current;
        // If we were in a call and lost connection, try to rejoin
        if (currentRoomId && isInCall) {
          console.log('Regained WebSocket connection, attempting to re-join room:', currentRoomId);
          // Re-send join to ensure server knows we're here
          wsService.send('call.join', { room_id: currentRoomId });
          
          // If SFU, we might need to restart ICE or re-negotiate
          if (callModeRef.current === 'sfu' && sfuConnection.current) {
            console.log('Triggering SFU ICE restart after reconnection');
            if (sfuConnection.current.restartIce) {
                sfuConnection.current.restartIce();
            }
          }
        }
      }
    };

    const unsubConnection = wsService.on('connection', handleConnectionChange);
    return () => unsubConnection();
  }, [isInCall]);

  // Cleanup on unmount
  React.useEffect(() => {
    // Copy refs to local variables for cleanup
    const peerConns = peerConnections.current;
    const sfuConn = sfuConnection.current;
    
    return () => {
      console.log('Component unmounting - cleaning up call resources');
      
      // FIX: Send leave signal on unmount!
      if (callRoomIdRef.current) {
         // Use the ref directly since we might be unmounting
         const msgType = callModeRef.current === 'sfu' ? 'sfu.leave' : 'call.leave';
         // Attempt to send if socket is still open
         try {
            wsService.send(msgType, { room_id: callRoomIdRef.current });
         } catch (e) {
             console.warn('Failed to send leave on unmount:', e);
         }
      }

      if (localStreamRef.current) {
        localStreamRef.current.getTracks().forEach(track => {
            try {
                track.stop();
            } catch { /* ignore */ }
        });
      }
      peerConns.forEach(pc => pc.close());
      if (sfuConn) {
        sfuConn.close();
      }
      
      // Reset critical refs
      callRoomIdRef.current = null;
      localStreamRef.current = null;
    };
  }, []); // Empty dependency array = runs only on mount/unmount

  return {
    isInCall,
    callState,
    callMode, // 'p2p' or 'sfu'
    callRoomId,
    participants,
    localStream,
    remoteStreams,
    isMuted,
    isVideoOff,
    error,
    incomingCall,
    joinCall,
    leaveCall,
    toggleMute,
    toggleVideo,
    acceptCall,
    declineCall
  };
}
