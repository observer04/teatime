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
      } catch {
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

  async createConversation(type, memberIds, title = null) {
    return this.request('/conversations', {
      method: 'POST',
      body: JSON.stringify({ type, member_ids: memberIds, title }),
    });
  }

  async updateConversation(conversationId, title) {
    return this.request(`/conversations/${conversationId}`, {
      method: 'PATCH',
      body: JSON.stringify({ title }),
    });
  }

  async addMember(conversationId, userId) {
    return this.request(`/conversations/${conversationId}/members`, {
      method: 'POST',
      body: JSON.stringify({ user_id: userId }),
    });
  }

  async removeMember(conversationId, userId) {
    return this.request(`/conversations/${conversationId}/members/${userId}`, {
      method: 'DELETE',
    });
  }

  async getMessages(conversationId, before = null, limit = 50) {
    let url = `/conversations/${conversationId}/messages?limit=${limit}`;
    if (before) {
      url += `&before=${encodeURIComponent(before)}`;
    }
    return this.request(url);
  }

  // Archive
  async archiveConversation(conversationId) {
    return this.request(`/conversations/${conversationId}/archive`, {
      method: 'POST',
    });
  }

  async unarchiveConversation(conversationId) {
    return this.request(`/conversations/${conversationId}/unarchive`, {
      method: 'POST',
    });
  }

  async getArchivedConversations() {
    return this.request('/conversations?archived=true');
  }

  // Starred Messages
  async starMessage(messageId) {
    return this.request(`/messages/${messageId}/star`, {
      method: 'POST',
    });
  }

  async unstarMessage(messageId) {
    return this.request(`/messages/${messageId}/star`, {
      method: 'DELETE',
    });
  }

  async getStarredMessages(limit = 50) {
    return this.request(`/messages/starred?limit=${limit}`);
  }

  // Search
  async searchMessages(conversationId, query, limit = 50) {
    return this.request(`/conversations/${conversationId}/messages/search?q=${encodeURIComponent(query)}&limit=${limit}`);
  }

  async searchAllMessages(query, limit = 50) {
    return this.request(`/messages/search?q=${encodeURIComponent(query)}&limit=${limit}`);
  }

  // Read Status
  async markConversationRead(conversationId, messageId = null) {
    return this.request(`/conversations/${conversationId}/read`, {
      method: 'POST',
      body: JSON.stringify({ message_id: messageId }),
    });
  }

  async markAllConversationsRead() {
    return this.request('/conversations/mark-all-read', {
      method: 'POST',
    });
  }
}

export const API_BASE_URL = API_BASE;
export default new ApiService();
