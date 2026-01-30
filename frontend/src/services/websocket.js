const _RAW_WS_BASE = import.meta.env.VITE_WS_BASE;
let WS_BASE;
if (typeof window !== 'undefined') {
  const hostname = window.location.hostname;
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
  
  // If accessing via app.ommprakash.cloud, use the api subdomain
  if (hostname === 'app.ommprakash.cloud') {
    WS_BASE = 'wss://api.ommprakash.cloud/ws';
  } else if (_RAW_WS_BASE) {
    // For localhost or other hostnames, use the configured base
    // Replace localhost with current hostname for WiFi/network access
    WS_BASE = _RAW_WS_BASE.replace('localhost', hostname).replace('127.0.0.1', hostname).replace('0.0.0.0', hostname);
  } else {
    WS_BASE = `${protocol}://${hostname}:8080/ws`;
  }
} else {
  WS_BASE = _RAW_WS_BASE || 'ws://localhost:8080/ws';
}

class WebSocketService {
  constructor() {
    this.ws = null;
    this.listeners = new Map();
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
    this.reconnectDelay = 3000;
  }

  connect(token) {
    if (this.ws?.readyState === WebSocket.OPEN) return;

    this.ws = new WebSocket(WS_BASE);

    this.ws.onopen = () => {
      console.log('WebSocket connected');
      this.reconnectAttempts = 0;
      this.send('auth', { token });
      this.emit('connection', { status: 'connected' });
    };

    this.ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      this.emit(msg.type, msg.payload);
    };

    this.ws.onclose = () => {
      console.log('WebSocket disconnected');
      this.emit('connection', { status: 'disconnected' });
      
      if (this.reconnectAttempts < this.maxReconnectAttempts) {
        setTimeout(() => {
          this.reconnectAttempts++;
          this.connect(token);
        }, this.reconnectDelay);
      }
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
      this.emit('error', error);
    };
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  send(type, payload) {
    if (this.ws?.readyState !== WebSocket.OPEN) {
      console.warn('WebSocket not connected');
      return;
    }

    this.ws.send(JSON.stringify({ type, payload }));
  }

  on(event, callback) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, []);
    }
    this.listeners.get(event).push(callback);

    // Return unsubscribe function
    return () => {
      const callbacks = this.listeners.get(event);
      if (callbacks) {
        const index = callbacks.indexOf(callback);
        if (index > -1) {
          callbacks.splice(index, 1);
        }
      }
    };
  }

  emit(event, data) {
    const callbacks = this.listeners.get(event);
    if (callbacks) {
      callbacks.forEach(callback => callback(data));
    }
  }
}

export default new WebSocketService();
