const _RAW_API_BASE = import.meta.env.VITE_API_BASE;
console.log('[API Config] VITE_API_BASE:', _RAW_API_BASE);
console.log('[API Config] Current hostname:', typeof window !== 'undefined' ? window.location.hostname : 'N/A');

let API_BASE;
if (typeof window !== 'undefined') {
  const hostname = window.location.hostname;
  const protocol = window.location.protocol;
  
  // If accessing via app.ommprakash.cloud, use the api subdomain
  if (hostname === 'app.ommprakash.cloud') {
    API_BASE = 'https://api.ommprakash.cloud';
    console.log('[API Config] Using tunnel API endpoint');
  } else if (_RAW_API_BASE) {
    // For localhost or other hostnames, use the configured base
    // Replace localhost with current hostname for WiFi/network access
    if (_RAW_API_BASE.includes('localhost') || _RAW_API_BASE.includes('127.0.0.1') || _RAW_API_BASE.includes('0.0.0.0')) {
      API_BASE = _RAW_API_BASE.replace('localhost', hostname).replace('127.0.0.1', hostname).replace('0.0.0.0', hostname);
      console.log('[API Config] Using hostname-replaced API:', API_BASE);
    } else {
      API_BASE = _RAW_API_BASE;
    }
  } else {
    console.warn('[API Config] VITE_API_BASE is undefined, using fallback');
    API_BASE = `${protocol}//${hostname}:8080`;
  }
} else {
  API_BASE = _RAW_API_BASE || 'http://localhost:8080';
}
console.log('[API Config] Final API_BASE:', API_BASE);

class ApiService {
  constructor() {
    this.baseURL = API_BASE;
    console.debug('ApiService base URL:', this.baseURL);
  }

  getToken() {
    return localStorage.getItem('token');
  }

  async request(endpoint, options = {}) {
    const token = this.getToken();
    const headers = {
      'Content-Type': 'application/json',
      ...options.headers,
    };

    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    try {
      const response = await fetch(`${this.baseURL}${endpoint}`, {
        ...options,
        headers,
      });

      let data = {};
      try {
        data = await response.json();
      } catch (err) {
        // ignore JSON parse errors for empty responses
        data = {};
      }

      if (!response.ok) {
        throw new Error(data.error || 'Request failed');
      }

      return data;
    } catch (err) {
      // Network errors are reported as TypeError by fetch
      if (err instanceof TypeError || err.message === 'Failed to fetch') {
        throw new Error('Network error: Failed to reach API. Ensure your device is on the same network and reload the app');
      }
      throw err;
    }
  }

  // Auth
  async register(username, email, password) {
    return this.request('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username, email, password }),
    });
  }

  async login(email, password) {
    return this.request('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
  }

  // Users
  async searchUsers(query) {
    return this.request(`/users/search?q=${encodeURIComponent(query)}`);
  }

  // Conversations
  async getConversations() {
    return this.request('/conversations');
  }

  async createConversation(type, memberIds) {
    return this.request('/conversations', {
      method: 'POST',
      body: JSON.stringify({ type, member_ids: memberIds }),
    });
  }

  async getMessages(conversationId) {
    return this.request(`/conversations/${conversationId}/messages`);
  }
}

export default new ApiService();
