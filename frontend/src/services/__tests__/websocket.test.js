import { describe, it, expect, vi, beforeEach } from 'vitest';

// We need to test the WebSocketService class directly.
// The module exports a singleton, so we'll import the module and test the instance.
// To get a fresh instance per test, we re-import.

// Mock WebSocket globally
class MockWebSocket {
  static OPEN = 1;
  static CLOSED = 3;

  constructor(url) {
    this.url = url;
    this.readyState = MockWebSocket.OPEN;
    this.onopen = null;
    this.onmessage = null;
    this.onclose = null;
    this.onerror = null;
    this.sentMessages = [];
    this.closeCalled = false;

    // Simulate connection opening
    setTimeout(() => {
      if (this.onopen) this.onopen();
    }, 0);
  }

  send(data) {
    this.sentMessages.push(JSON.parse(data));
  }

  close() {
    this.closeCalled = true;
    this.readyState = MockWebSocket.CLOSED;
  }
}

describe('WebSocketService', () => {
  let wsService;

  beforeEach(async () => {
    vi.stubGlobal('WebSocket', MockWebSocket);
    // Dynamic import to get fresh module each time
    vi.resetModules();
    const mod = await import('../../services/websocket.js');
    wsService = mod.default;
    // Ensure clean state
    wsService.disconnect();
    wsService.listeners = new Map();
    wsService.reconnectAttempts = 0;
  });

  it('should have null ws initially', () => {
    expect(wsService.ws).toBeNull();
  });

  it('on() should register callbacks and return unsubscribe', () => {
    const cb = vi.fn();
    const unsub = wsService.on('test.event', cb);

    wsService.emit('test.event', { data: 'hello' });
    expect(cb).toHaveBeenCalledWith({ data: 'hello' });

    unsub();
    wsService.emit('test.event', { data: 'after unsub' });
    expect(cb).toHaveBeenCalledTimes(1);
  });

  it('emit() should call all listeners for event', () => {
    const cb1 = vi.fn();
    const cb2 = vi.fn();
    wsService.on('multi', cb1);
    wsService.on('multi', cb2);

    wsService.emit('multi', 'payload');
    expect(cb1).toHaveBeenCalledWith('payload');
    expect(cb2).toHaveBeenCalledWith('payload');
  });

  it('emit() should not throw for unknown event', () => {
    expect(() => wsService.emit('nonexistent', {})).not.toThrow();
  });

  it('send() should warn and skip when not connected', () => {
    const spy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    wsService.send('test', { data: 1 });
    expect(spy).toHaveBeenCalledWith('WebSocket not connected');
    spy.mockRestore();
  });

  it('send() should serialize and send when connected', () => {
    // Manually create a mock ws
    const mockWs = new MockWebSocket('ws://test');
    mockWs.readyState = MockWebSocket.OPEN;
    wsService.ws = mockWs;

    wsService.send('message.send', { body_text: 'hello' });

    expect(mockWs.sentMessages).toHaveLength(1);
    expect(mockWs.sentMessages[0]).toEqual({
      type: 'message.send',
      payload: { body_text: 'hello' },
    });
  });

  it('disconnect() should close ws and set to null', () => {
    const mockWs = new MockWebSocket('ws://test');
    wsService.ws = mockWs;

    wsService.disconnect();
    expect(mockWs.closeCalled).toBe(true);
    expect(wsService.ws).toBeNull();
  });

  it('disconnect() should be safe to call when already disconnected', () => {
    expect(() => wsService.disconnect()).not.toThrow();
  });
});
