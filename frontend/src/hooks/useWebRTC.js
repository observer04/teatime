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
 * Handles signaling via WebSocket and manages peer connections
 */
export function useWebRTC(userId) {
  const [isInCall, setIsInCall] = useState(false);
  const [callRoomId, setCallRoomId] = useState(null);
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
  
  // Store peer connections by user ID
  const peerConnections = useRef(new Map());
  const pendingCandidates = useRef(new Map()); // Store ICE candidates until connection is ready

  // Clean up peer connection
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

  // Create a peer connection for a specific user
  const createPeerConnection = useCallback((peerId, peerName, isInitiator) => {
    if (peerConnections.current.has(peerId)) {
      console.log('Peer connection already exists for:', peerId);
      return peerConnections.current.get(peerId);
    }

    console.log('Creating peer connection for:', peerId, 'isInitiator:', isInitiator);
    
    const pc = new RTCPeerConnection({ iceServers });
    peerConnections.current.set(peerId, pc);

    // Add local tracks to connection
    if (localStream) {
      localStream.getTracks().forEach(track => {
        pc.addTrack(track, localStream);
      });
    }

    // Handle ICE candidates
    pc.onicecandidate = (event) => {
      if (event.candidate) {
        wsService.send('call.ice_candidate', {
          room_id: callRoomId,
          target_id: peerId,
          candidate: JSON.stringify(event.candidate)
        });
      }
    };

    // Handle remote stream
    pc.ontrack = (event) => {
      console.log('Received remote track from:', peerId);
      setRemoteStreams(prev => ({
        ...prev,
        [peerId]: {
          stream: event.streams[0],
          username: peerName
        }
      }));
    };

    // Handle connection state changes
    pc.onconnectionstatechange = () => {
      console.log('Connection state for', peerId, ':', pc.connectionState);
      if (pc.connectionState === 'failed' || pc.connectionState === 'disconnected') {
        // Could implement reconnection logic here
      }
    };

    // Process any pending ICE candidates
    const pending = pendingCandidates.current.get(peerId);
    if (pending) {
      pending.forEach(candidate => {
        pc.addIceCandidate(new RTCIceCandidate(candidate));
      });
      pendingCandidates.current.delete(peerId);
    }

    return pc;
  }, [iceServers, localStream, callRoomId]);

  // Initialize local media stream
  const getLocalMedia = useCallback(async (video = true) => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: video ? { width: 640, height: 480 } : false,
        audio: true
      });
      setLocalStream(stream);
      return stream;
    } catch (err) {
      console.error('Failed to get local media:', err);
      setError('Could not access camera/microphone');
      throw err;
    }
  }, []);

  // Join a call
  const joinCall = useCallback(async (roomId, videoEnabled = true) => {
    try {
      setCallState('connecting');
      setError(null);
      setCallRoomId(roomId);

      // Get local media first
      await getLocalMedia(videoEnabled);
      setIsVideoOff(!videoEnabled);

      // Join via WebSocket
      wsService.send('call.join', { room_id: roomId });
    } catch (err) {
      setCallState('idle');
      setError(err.message);
    }
  }, [getLocalMedia]);

  // Leave the call
  const leaveCall = useCallback(() => {
    if (callRoomId) {
      wsService.send('call.leave', { room_id: callRoomId });
    }

    // Stop local stream
    if (localStream) {
      localStream.getTracks().forEach(track => track.stop());
      setLocalStream(null);
    }

    // Close all peer connections
    peerConnections.current.forEach((pc, peerId) => {
      closePeerConnection(peerId);
    });

    setIsInCall(false);
    setCallRoomId(null);
    setParticipants([]);
    setRemoteStreams({});
    setCallState('idle');
  }, [callRoomId, localStream, closePeerConnection]);

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
      console.log('Call config received:', payload);
      
      setIceServers(payload.ice_servers || DEFAULT_ICE_CONFIG.iceServers);
      setParticipants(payload.participants || []);
      setIsInCall(true);
      setCallState('connected');

      // Create offers to existing participants (we are the new joiner)
      const otherParticipants = (payload.participants || []).filter(p => p.user_id !== userId);
      
      for (const participant of otherParticipants) {
        const pc = createPeerConnection(participant.user_id, participant.username, true);
        
        try {
          const offer = await pc.createOffer();
          await pc.setLocalDescription(offer);
          
          wsService.send('call.offer', {
            room_id: payload.room_id,
            target_id: participant.user_id,
            sdp: JSON.stringify(offer)
          });
        } catch (err) {
          console.error('Failed to create offer:', err);
        }
      }
    };

    const unsubConfig = wsService.on('call.config', handleCallConfig);
    return () => unsubConfig();
  }, [userId, createPeerConnection]);

  // Handle participant joined
  useEffect(() => {
    const handleParticipantJoined = (payload) => {
      console.log('Participant joined:', payload);
      if (payload.user_id === userId) return; // Ignore self
      
      setParticipants(prev => {
        if (prev.find(p => p.user_id === payload.user_id)) return prev;
        return [...prev, { user_id: payload.user_id, username: payload.username }];
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
      setParticipants(prev => prev.filter(p => p.user_id !== payload.user_id));
    };

    const unsubLeft = wsService.on('call.participant_left', handleParticipantLeft);
    return () => unsubLeft();
  }, [closePeerConnection]);

  // Auto-end call when all other participants leave
  useEffect(() => {
    if (!isInCall) return;
    
    // Check if there are any other participants besides ourselves
    const othersInCall = participants.filter(p => p.user_id !== userId);
    
    // If we're in a call but no other participants, and we have no remote streams, end the call
    if (othersInCall.length === 0 && Object.keys(remoteStreams).length === 0 && callState === 'connected') {
      console.log('All other participants left, ending call');
      // Small delay to ensure this isn't triggered during initial join
      const timer = setTimeout(() => {
        if (callRoomId) {
          wsService.send('call.leave', { room_id: callRoomId });
        }
        if (localStream) {
          localStream.getTracks().forEach(track => track.stop());
        }
        setLocalStream(null);
        peerConnections.current.forEach((pc, peerId) => {
          pc.close();
          peerConnections.current.delete(peerId);
        });
        setIsInCall(false);
        setCallRoomId(null);
        setParticipants([]);
        setRemoteStreams({});
        setCallState('idle');
      }, 1000);
      return () => clearTimeout(timer);
    }
  }, [isInCall, participants, userId, remoteStreams, callState, callRoomId, localStream]);

  // Handle incoming offer
  useEffect(() => {
    const handleOffer = async (payload) => {
      console.log('Received offer from:', payload.from_id);
      
      const pc = createPeerConnection(payload.from_id, payload.from_name, false);
      
      try {
        const offer = JSON.parse(payload.sdp);
        await pc.setRemoteDescription(new RTCSessionDescription(offer));
        
        const answer = await pc.createAnswer();
        await pc.setLocalDescription(answer);
        
        wsService.send('call.answer', {
          room_id: payload.room_id,
          target_id: payload.from_id,
          sdp: JSON.stringify(answer)
        });
      } catch (err) {
        console.error('Failed to handle offer:', err);
      }
    };

    const unsubOffer = wsService.on('call.offer', handleOffer);
    return () => unsubOffer();
  }, [createPeerConnection, callRoomId]);

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
      // Call ended
      if (incomingCall?.callId === payload.call_id) {
        setIncomingCall(null);
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
  }, [isInCall, incomingCall]);

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
    return () => {
      if (localStream) {
        localStream.getTracks().forEach(track => track.stop());
      }
      peerConnections.current.forEach(pc => pc.close());
    };
  }, [localStream]);

  return {
    isInCall,
    callState,
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
