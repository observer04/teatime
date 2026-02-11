import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useWebSocket } from '../useWebSocket';
import wsService from '../../services/websocket';

// Mock the websocket service singleton
vi.mock('../../services/websocket', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    send: vi.fn(),
    on: vi.fn(),
  }
}));

describe('useWebSocket', () => {
  let connectionCallback;
  let messageCallback;
  let receiptCallback;

  beforeEach(() => {
    vi.clearAllMocks();

    // Setup default mock implementation for wsService.on
    wsService.on.mockImplementation((event, callback) => {
      if (event === 'connection') connectionCallback = callback;
      if (event === 'message.new') messageCallback = callback;
      if (event === 'receipt.updated') receiptCallback = callback;
      return vi.fn(); // unsubscribe function
    });
  });

  it('should connect when token is provided', () => {
    renderHook(() => useWebSocket('test-token', vi.fn()));
    expect(wsService.connect).toHaveBeenCalledWith('test-token');
    expect(wsService.on).toHaveBeenCalledWith('connection', expect.any(Function));
    expect(wsService.on).toHaveBeenCalledWith('message.new', expect.any(Function));
    expect(wsService.on).toHaveBeenCalledWith('receipt.updated', expect.any(Function));
  });

  it('should not connect when token is missing', () => {
    renderHook(() => useWebSocket(null, vi.fn()));
    expect(wsService.connect).not.toHaveBeenCalled();
  });

  it('should update isConnected state on connection events', () => {
    const { result } = renderHook(() => useWebSocket('token', vi.fn()));
    
    expect(result.current.isConnected).toBe(false);

    act(() => {
      connectionCallback({ status: 'connected' });
    });
    expect(result.current.isConnected).toBe(true);

    act(() => {
      connectionCallback({ status: 'disconnected' });
    });
    expect(result.current.isConnected).toBe(false);
  });

  it('should handle incoming messages and call onIncomingMessage', () => {
    const onIncomingMessage = vi.fn();
    const { result } = renderHook(() => useWebSocket('token', onIncomingMessage));

    const mockPayload = {
      id: 'msg-1',
      conversation_id: 'conv-1',
      sender_id: 'user-2',
      body_text: 'Hello',
      created_at: new Date().toISOString(),
      sender_username: 'bob'
    };

    act(() => {
      messageCallback(mockPayload);
    });

    // Check state update
    expect(result.current.messages['conv-1']).toHaveLength(1);
    expect(result.current.messages['conv-1'][0].id).toBe('msg-1');
    expect(result.current.messages['conv-1'][0].receipt_status).toBe('sent');

    // Check callback
    expect(onIncomingMessage).toHaveBeenCalledTimes(1);
    expect(onIncomingMessage).toHaveBeenCalledWith(expect.objectContaining({
      id: 'msg-1',
      body_text: 'Hello'
    }));
  });

  it('should handle receipt updates', () => {
    const { result } = renderHook(() => useWebSocket('token', vi.fn()));

    // First add a message
    const mockPayload = {
      id: 'msg-1',
      conversation_id: 'conv-1',
      sender_id: 'me',
      body_text: 'My message',
      created_at: new Date().toISOString()
    };

    act(() => {
      messageCallback(mockPayload);
    });

    expect(result.current.messages['conv-1'][0].receipt_status).toBe('sent');

    // Receive delivered update
    act(() => {
      receiptCallback({
        conversation_id: 'conv-1',
        message_id: 'msg-1',
        status: 'delivered'
      });
    });

    expect(result.current.messages['conv-1'][0].receipt_status).toBe('delivered');

    // Receive read update
    act(() => {
      receiptCallback({
        conversation_id: 'conv-1',
        message_id: 'msg-1',
        status: 'read'
      });
    });

    expect(result.current.messages['conv-1'][0].receipt_status).toBe('read');
  });

  it('sendMessage() should send correct payload', () => {
    const { result } = renderHook(() => useWebSocket('token', vi.fn()));

    result.current.sendMessage('conv-1', 'hello', 'att-1');

    expect(wsService.send).toHaveBeenCalledWith('message.send', {
      conversation_id: 'conv-1',
      body_text: 'hello',
      attachment_id: 'att-1'
    });
  });

  it('joinRoom/leaveRoom should send correct events', () => {
    const { result } = renderHook(() => useWebSocket('token', vi.fn()));

    result.current.joinRoom('conv-1');
    expect(wsService.send).toHaveBeenCalledWith('room.join', { conversation_id: 'conv-1' });

    result.current.leaveRoom('conv-1');
    expect(wsService.send).toHaveBeenCalledWith('room.leave', { conversation_id: 'conv-1' });
  });

  it('typing indicators should send correct events', () => {
    const { result } = renderHook(() => useWebSocket('token', vi.fn()));

    result.current.startTyping('conv-1');
    expect(wsService.send).toHaveBeenCalledWith('typing.start', { conversation_id: 'conv-1' });

    result.current.stopTyping('conv-1');
    expect(wsService.send).toHaveBeenCalledWith('typing.stop', { conversation_id: 'conv-1' });
  });

  it('should disconnect on unmount', () => {
    const unsub = vi.fn();
    wsService.on.mockReturnValue(unsub);

    const { unmount } = renderHook(() => useWebSocket('token', vi.fn()));
    
    unmount();

    expect(unsub).toHaveBeenCalled();
    expect(wsService.disconnect).toHaveBeenCalled();
  });
});
