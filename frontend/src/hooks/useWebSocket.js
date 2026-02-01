import { useState, useEffect, useCallback } from 'react';
import wsService from '../services/websocket';

export function useWebSocket(token) {
  const [isConnected, setIsConnected] = useState(false);
  const [messages, setMessages] = useState({});

  useEffect(() => {
    if (!token) return;

    wsService.connect(token);

    const unsubConnection = wsService.on('connection', ({ status }) => {
      setIsConnected(status === 'connected');
    });

    const unsubMessage = wsService.on('message.new', (payload) => {
      console.log('Received message.new:', payload);
      // Normalize the message format to match API response
      const normalizedMessage = {
        id: payload.id,
        conversation_id: payload.conversation_id,
        sender_id: payload.sender_id,
        body_text: payload.body_text,
        created_at: payload.created_at,
        attachment_id: payload.attachment_id,
        attachment: payload.attachment,
        receipt_status: 'sent', // New messages start as "sent"
        sender: {
          id: payload.sender_id,
          username: payload.sender_username
        }
      };
      
      setMessages(prev => ({
        ...prev,
        [payload.conversation_id]: [
          ...(prev[payload.conversation_id] || []),
          normalizedMessage
        ]
      }));
    });

    // Handle receipt updates (single and batch)
    const unsubReceipt = wsService.on('receipt.updated', (payload) => {
      console.log('Received receipt.updated:', payload);
      
      // Handle both single message and batch updates
      const messageIDs = payload.message_ids || [payload.message_id];
      const status = payload.status; // "delivered" or "read"
      const conversationId = payload.conversation_id;
      
      setMessages(prev => {
        const conversationMessages = prev[conversationId];
        if (!conversationMessages) return prev;
        
        return {
          ...prev,
          [conversationId]: conversationMessages.map(msg => {
            if (messageIDs.includes(msg.id)) {
              // Only upgrade status (sent -> delivered -> read)
              const currentStatus = msg.receipt_status || 'sent';
              const shouldUpgrade = 
                (currentStatus === 'sent' && (status === 'delivered' || status === 'read')) ||
                (currentStatus === 'delivered' && status === 'read');
              
              if (shouldUpgrade) {
                return { ...msg, receipt_status: status };
              }
            }
            return msg;
          })
        };
      });
    });

    return () => {
      unsubConnection();
      unsubMessage();
      unsubReceipt();
      wsService.disconnect();
    };
  }, [token]);

  const sendMessage = useCallback((conversationId, bodyText, attachmentId = null) => {
    const payload = {
      conversation_id: conversationId,
      body_text: bodyText || ''
    };
    
    if (attachmentId) {
      payload.attachment_id = attachmentId;
    }
    
    wsService.send('message.send', payload);
  }, []);

  const joinRoom = useCallback((conversationId) => {
    wsService.send('room.join', { conversation_id: conversationId });
  }, []);

  const leaveRoom = useCallback((conversationId) => {
    wsService.send('room.leave', { conversation_id: conversationId });
  }, []);

  const startTyping = useCallback((conversationId) => {
    wsService.send('typing.start', { conversation_id: conversationId });
  }, []);

  const stopTyping = useCallback((conversationId) => {
    wsService.send('typing.stop', { conversation_id: conversationId });
  }, []);

  // Mark a message as read
  const markAsRead = useCallback((messageId) => {
    wsService.send('receipt.read', { message_id: messageId });
  }, []);

  return {
    isConnected,
    messages,
    setMessages,
    sendMessage,
    joinRoom,
    leaveRoom,
    startTyping,
    stopTyping,
    markAsRead
  };
}
