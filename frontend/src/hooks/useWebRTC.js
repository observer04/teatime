import { useState, useEffect, useCallback, useRef } from 'react';
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
  const [isInCall, setIsInCall] = useState(false);
  const [callRoomId, setCallRoomId] = useState(null);
  const [callMode, setCallMode] = useState('p2p'); // 'p2p' or 'sfu'
  const [participants, setParticipants] = useState([]);
  const [localStream, setLocalStream] = useState(null);
  const [remoteStreams, setRemoteStreams] = useState({});
  const [isMuted, setIsMuted] = useState(false);
  const [isVideoOff, setIsVideoOff] = useState(false);
  const [iceServers, setIceServers] = useState(DEFAULT_ICE_CONFIG.iceServers);
  const [error, setError] = useState(null);
  const [callState, setCallState] = useState('idle'); // idle, connecting, connected, reconnecting
  
  // Incoming call state
  const [incomingCall, setIncomingCall] = useState(null); // { callId, conversationId, caller, callType, isGroup, conversationName }
  
  // P2P: Store peer connections by user ID
  const peerConnections = useRef(new Map());
  const pendingCandidates = useRef(new Map()); // Store ICE candidates until connection is ready
  
  // CRITICAL: Use refs for values that need to be accessed in callbacks without stale closures
  const localStreamRef = useRef(null);
  const callRoomIdRef = useRef(null);
  const iceServersRef = useRef(DEFAULT_ICE_CONFIG.iceServers);
  
  // Track if we've ever had other participants (for proper call ending)
  const hasEverHadParticipants = useRef(false);
  const maxParticipantCount = useRef(0);
  
  // Keep refs in sync with state
  useEffect(() => { localStreamRef.current = localStream; }, [localStream]);
  useEffect(() => { callRoomIdRef.current = callRoomId; }, [callRoomId]);
  useEffect(() => { iceServersRef.current = iceServers; }, [iceServers]);
  
  // Track max participants we've seen
  useEffect(() => {
    const otherCount = participants.filter(p => p.user_id !== userId).length;
    if (otherCount > 0) {
      hasEverHadParticipants.current = true;
      maxParticipantCount.current = Math.max(maxParticipantCount.current, otherCount);
    }
  }, [participants, userId]);
  
  // SFU: Single peer connection to server
  const sfuConnection = useRef(null);
  const sfuPendingCandidates = useRef([]);

  // Clean up peer connection (P2P mode)
  const closePeerConnection = useCallback((peerId) => {
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
  const closeSFUConnection = useCallback(() => {
    if (sfuConnection.current) {
      sfuConnection.current.close();
      sfuConnection.current = null;
    }
    sfuPendingCandidates.current = [];
    setRemoteStreams({});
  }, []);

  // Create SFU peer connection (one connection to server that handles all tracks)
  // Uses refs to avoid stale closures
  const createSFUConnection = useCallback(async (stream) => {
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
            candidate: JSON.stringify(event.candidate)
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
    pc.oniceconnectionstatechange = () => {
      console.log('SFU ICE connection state:', pc.iceConnectionState);
    };

    return pc;
  }, []);

  // Create a peer connection for a specific user
  // CRITICAL: Uses refs to avoid stale closure issues with localStream
  const createPeerConnection = useCallback((peerId, peerName, isInitiator) => {
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
    pc.oniceconnectionstatechange = () => {
      console.log('ICE connection state for', peerId, ':', pc.iceConnectionState);
      if (pc.iceConnectionState === 'connected' || pc.iceConnectionState === 'completed') {
        console.log('✅ ICE connection established with:', peerId);
      } else if (pc.iceConnectionState === 'failed') {
        console.error('❌ ICE connection failed with:', peerId);
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
    pc.onnegotiationneeded = () => {
      console.log('Negotiation needed for peer:', peerId, 'isInitiator:', isInitiator);
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

  // Initialize local media stream
  const getLocalMedia = useCallback(async (video = true) => {
    try {
      console.log('Requesting media permissions...');
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
  const getLocalMediaWithFallback = useCallback(async (preferVideo = true) => {
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
  const joinCall = useCallback(async (roomId, videoEnabled = true) => {
    try {
      setCallState('connecting');
      setError(null);
      
      // Reset tracking refs for new call
      hasEverHadParticipants.current = false;
      maxParticipantCount.current = 0;
      
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
    }
  }, [getLocalMediaWithFallback]);

  // Leave the call
  const leaveCall = useCallback(() => {
    console.log('leaveCall called');
    const roomId = callRoomIdRef.current;
    if (roomId) {
      wsService.send('call.leave', { room_id: roomId });
    }

    // Stop local stream
    const stream = localStreamRef.current;
    if (stream) {
      stream.getTracks().forEach(track => track.stop());
      setLocalStream(null);
      localStreamRef.current = null;
    }

    // Close connections based on mode
    if (callMode === 'sfu') {
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

    setIsInCall(false);
    setCallRoomId(null);
    callRoomIdRef.current = null;
    setCallMode('p2p');
    setParticipants([]);
    setRemoteStreams({});
    setCallState('idle');
  }, [callMode, closePeerConnection, closeSFUConnection]);

  // Toggle mute
  const toggleMute = useCallback(() => {
    if (localStream) {
      const audioTrack = localStream.getAudioTracks()[0];
      if (audioTrack) {
        audioTrack.enabled = !audioTrack.enabled;
        setIsMuted(!audioTrack.enabled);
      }
    }
  }, [localStream]);

  // Toggle video
  const toggleVideo = useCallback(() => {
    if (localStream) {
      const videoTrack = localStream.getVideoTracks()[0];
      if (videoTrack) {
        videoTrack.enabled = !videoTrack.enabled;
        setIsVideoOff(!videoTrack.enabled);
      }
    }
  }, [localStream]);

  // Handle incoming call config (after joining)
  useEffect(() => {
    const handleCallConfig = async (payload) => {
      console.log('=== CALL CONFIG RECEIVED ===');
      console.log('Payload:', payload);
      console.log('Local userId:', userId);
      console.log('Participants from backend:', payload.participants);
      console.log('Local stream ref available:', !!localStreamRef.current);
      
      // Update ICE servers ref immediately
      const newIceServers = payload.ice_servers || DEFAULT_ICE_CONFIG.iceServers;
      iceServersRef.current = newIceServers;
      setIceServers(newIceServers);
      
      setParticipants(payload.participants || []);
      setIsInCall(true);
      setCallState('connected');
      
      // Check if this is SFU mode
      const mode = payload.mode || 'p2p';
      setCallMode(mode);
      
      if (mode === 'sfu') {
        // SFU mode: wait for sfu.offer from server
        console.log('SFU mode: waiting for offer from server');
        // Create SFU connection with local stream
        await createSFUConnection(localStreamRef.current);
      } else {
        // P2P mode: Create offers to existing participants (we are the new joiner)
        const otherParticipants = (payload.participants || []).filter(p => p.user_id !== userId);
        
        console.log('P2P mode - Other participants:', otherParticipants);
        console.log('Will create offers to', otherParticipants.length, 'participants');
        console.log('Current stream ref:', !!localStreamRef.current, 'tracks:', localStreamRef.current?.getTracks().length);
        
        for (const participant of otherParticipants) {
          console.log('Creating peer connection and offer for:', participant.user_id, participant.username);
          const pc = createPeerConnection(participant.user_id, participant.username, true);
          
          // Verify tracks were added
          const senders = pc.getSenders();
          console.log('Peer connection senders after creation:', senders.length, senders.map(s => s.track?.kind));
          
          try {
            const offer = await pc.createOffer();
            await pc.setLocalDescription(offer);
            
            console.log('Sending offer to:', participant.user_id, 'SDP type:', offer.type);
            wsService.send('call.offer', {
              room_id: payload.room_id,
              target_id: participant.user_id,
              sdp: JSON.stringify(offer)
            });
          } catch (err) {
            console.error('Failed to create offer for', participant.user_id, ':', err);
          }
        }
      }
    };

    const unsubConfig = wsService.on('call.config', handleCallConfig);
    return () => unsubConfig();
  }, [userId, createPeerConnection, createSFUConnection]);

  // Handle participant joined
  useEffect(() => {
    const handleParticipantJoined = (payload) => {
      console.log('Participant joined:', payload);
      if (payload.user_id === userId) return; // Ignore self
      
      // Mark that we've had participants join (for call ending logic)
      hasEverHadParticipants.current = true;
      maxParticipantCount.current = Math.max(maxParticipantCount.current, 1);
      
      setParticipants(prev => {
        if (prev.find(p => p.user_id === payload.user_id)) return prev;
        const newParticipants = [...prev, { user_id: payload.user_id, username: payload.username }];
        maxParticipantCount.current = Math.max(maxParticipantCount.current, newParticipants.filter(p => p.user_id !== userId).length);
        return newParticipants;
      });
      
      // The new participant will send us an offer, so we just wait
    };

    const unsubJoined = wsService.on('call.participant_joined', handleParticipantJoined);
    return () => unsubJoined();
  }, [userId]);

  // Handle participant left
  useEffect(() => {
    const handleParticipantLeft = (payload) => {
      console.log('Participant left:', payload);
      closePeerConnection(payload.user_id);
      setParticipants(prev => {
        const newParticipants = prev.filter(p => p.user_id !== payload.user_id);
        // Count remaining participants (excluding ourselves)
        const othersRemaining = newParticipants.filter(p => p.user_id !== userId);
        console.log('After participant left - others remaining:', othersRemaining.length);
        
        // For P2P calls: if someone was in call and left, end the call for remaining user
        if (hasEverHadParticipants.current && othersRemaining.length === 0) {
          console.log('Last participant left P2P call - ending call after delay');
          setTimeout(() => {
            if (callRoomIdRef.current) {
              setCallState('ended');
              // Give user a moment to see the ended state before full cleanup
              setTimeout(() => leaveCall(), 2000);
            }
          }, 500);
        }
        
        return newParticipants;
      });
    };

    const unsubLeft = wsService.on('call.participant_left', handleParticipantLeft);
    return () => unsubLeft();
  }, [closePeerConnection, userId, leaveCall]);

  // Track if we've ever had a successful connection (used to prevent premature call ending)
  const hasEverConnected = useRef(false);
  
  // Update hasEverConnected when we get remote streams
  useEffect(() => {
    if (Object.keys(remoteStreams).length > 0) {
      hasEverConnected.current = true;
    }
  }, [remoteStreams]);
  
  // Reset hasEverConnected when call ends
  useEffect(() => {
    if (!isInCall) {
      hasEverConnected.current = false;
    }
  }, [isInCall]);

  // Auto-end call when all other participants leave (only after we've connected at least once)
  useEffect(() => {
    if (!isInCall) return;
    
    // Don't auto-end if we've never had a connection yet (still waiting for others to join)
    if (!hasEverConnected.current) return;
    
    // Check if there are any other participants besides ourselves
    const othersInCall = participants.filter(p => p.user_id !== userId);
    
    // If we're in a call but no other participants, and we have no remote streams, end the call
    if (othersInCall.length === 0 && Object.keys(remoteStreams).length === 0 && callState === 'connected') {
      console.log('All other participants left, ending call');
      // Small delay to ensure this isn't triggered during transient states
      const timer = setTimeout(() => {
        const roomId = callRoomIdRef.current;
        if (roomId) {
          wsService.send('call.leave', { room_id: roomId });
        }
        const stream = localStreamRef.current;
        if (stream) {
          stream.getTracks().forEach(track => track.stop());
        }
        setLocalStream(null);
        localStreamRef.current = null;
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
      return () => clearTimeout(timer);
    }
  }, [isInCall, participants, userId, remoteStreams, callState]);

  // Handle incoming offer - CRITICAL: This is where first user receives offer from second user
  useEffect(() => {
    const handleOffer = async (payload) => {
      console.log('=== RECEIVED OFFER ===');
      console.log('From:', payload.from_id, payload.from_name);
      console.log('Room:', payload.room_id);
      console.log('Local stream ref available:', !!localStreamRef.current);
      console.log('Local stream ref tracks:', localStreamRef.current?.getTracks().map(t => t.kind));
      
      // Create peer connection - uses refs internally to get current stream
      const pc = createPeerConnection(payload.from_id, payload.from_name, false);
      
      // Double-check tracks were added
      const senders = pc.getSenders();
      console.log('Peer connection senders after creation:', senders.length, senders.map(s => s.track?.kind));
      
      // If no tracks were added (edge case), add them now
      if (senders.length === 0 && localStreamRef.current) {
        console.warn('No senders found, manually adding tracks now');
        localStreamRef.current.getTracks().forEach(track => {
          console.log('Late-adding track:', track.kind, track.id);
          pc.addTrack(track, localStreamRef.current);
        });
      }
      
      try {
        const offer = JSON.parse(payload.sdp);
        console.log('Setting remote description (offer)...');
        await pc.setRemoteDescription(new RTCSessionDescription(offer));
        
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
      }
    };

    const unsubOffer = wsService.on('call.offer', handleOffer);
    return () => unsubOffer();
  }, [createPeerConnection]);

  // Handle incoming answer
  useEffect(() => {
    const handleAnswer = async (payload) => {
      console.log('Received answer from:', payload.from_id);
      
      const pc = peerConnections.current.get(payload.from_id);
      if (!pc) {
        console.error('No peer connection for:', payload.from_id);
        return;
      }
      
      try {
        const answer = JSON.parse(payload.sdp);
        await pc.setRemoteDescription(new RTCSessionDescription(answer));
      } catch (err) {
        console.error('Failed to handle answer:', err);
      }
    };

    const unsubAnswer = wsService.on('call.answer', handleAnswer);
    return () => unsubAnswer();
  }, []);

  // Handle incoming ICE candidate
  useEffect(() => {
    const handleIceCandidate = async (payload) => {
      const pc = peerConnections.current.get(payload.from_id);
      const candidate = JSON.parse(payload.candidate);
      
      if (pc && pc.remoteDescription) {
        try {
          await pc.addIceCandidate(new RTCIceCandidate(candidate));
        } catch (err) {
          console.error('Failed to add ICE candidate:', err);
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
  useEffect(() => {
    const handleSFUOffer = async (payload) => {
      console.log('Received SFU offer');
      
      const pc = sfuConnection.current;
      if (!pc) {
        console.error('No SFU connection');
        return;
      }
      
      try {
        const offer = JSON.parse(payload.sdp);
        await pc.setRemoteDescription(new RTCSessionDescription(offer));
        
        // Process any pending ICE candidates
        for (const candidate of sfuPendingCandidates.current) {
          await pc.addIceCandidate(new RTCIceCandidate(candidate));
        }
        sfuPendingCandidates.current = [];
        
        const answer = await pc.createAnswer();
        await pc.setLocalDescription(answer);
        
        wsService.send('sfu.answer', {
          room_id: payload.room_id,
          sdp: JSON.stringify(answer)
        });
      } catch (err) {
        console.error('Failed to handle SFU offer:', err);
      }
    };

    const unsubSFUOffer = wsService.on('sfu.offer', handleSFUOffer);
    return () => unsubSFUOffer();
  }, []);

  // SFU: Handle ICE candidate from server
  useEffect(() => {
    const handleSFUCandidate = async (payload) => {
      const pc = sfuConnection.current;
      const candidate = JSON.parse(payload.candidate);
      
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
  useEffect(() => {
    const handleSFUTracks = (payload) => {
      console.log('SFU tracks update:', payload);
      
      // Update remote streams with user info
      const tracks = payload.tracks || [];
      const trackMap = new Map();
      tracks.forEach(t => trackMap.set(t.id, { userId: t.user_id, username: t.username }));
      
      setRemoteStreams(prev => {
        const updated = { ...prev };
        for (const [streamId, data] of Object.entries(updated)) {
          const trackInfo = trackMap.get(streamId);
          if (trackInfo) {
            updated[streamId] = {
              ...data,
              userId: trackInfo.userId,
              username: trackInfo.username
            };
          }
        }
        return updated;
      });
    };

    const unsubSFUTracks = wsService.on('sfu.tracks', handleSFUTracks);
    return () => unsubSFUTracks();
  }, []);

  // Handle incoming call notification
  useEffect(() => {
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
        // Delay the cleanup to show the "ended" state
        setTimeout(() => leaveCall(), 1500);
      }
    };

    const unsubIncoming = wsService.on('call.incoming', handleIncomingCall);
    const unsubCancelled = wsService.on('call.cancelled', handleCallCancelled);
    const unsubEnded = wsService.on('call.ended', handleCallEnded);

    return () => {
      unsubIncoming();
      unsubCancelled();
      unsubEnded();
    };
  }, [isInCall, incomingCall, leaveCall]);

  // Accept incoming call
  const acceptCall = useCallback(async (withVideo = true) => {
    if (!incomingCall) return;

    try {
      // Join the call room
      await joinCall(incomingCall.conversationId, withVideo);
      setIncomingCall(null);
    } catch (err) {
      console.error('Failed to accept call:', err);
      setError(err.message);
    }
  }, [incomingCall, joinCall]);

  // Decline incoming call
  const declineCall = useCallback(() => {
    if (!incomingCall) return;

    // Send decline event
    wsService.send('call.declined', {
      call_id: incomingCall.callId,
      conversation_id: incomingCall.conversationId
    });

    setIncomingCall(null);
  }, [incomingCall]);

  // Cleanup on unmount
  useEffect(() => {
    // Copy refs to local variables for cleanup
    const peerConns = peerConnections.current;
    const sfuConn = sfuConnection.current;
    
    return () => {
      if (localStream) {
        localStream.getTracks().forEach(track => track.stop());
      }
      peerConns.forEach(pc => pc.close());
      if (sfuConn) {
        sfuConn.close();
      }
    };
  }, [localStream]);

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
