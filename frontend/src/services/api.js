const _RAW_API_BASE = import.meta.env.VITE_API_BASE;
console.log('[API Config] VITE_API_BASE:', _RAW_API_BASE);
console.log('[API Config] Current hostname:', typeof window !== 'undefined' ? window.location.hostname : 'N/A');

let API_BASE;
if (typeof window !== 'undefined') {
  const hostname = window.location.hostname;
  
  // If accessing via the production subdomain, use the /api path on same origin
  if (hostname === 'teatime.ommprakash.cloud') {
    API_BASE = window.location.origin + '/api';
    console.log('[API Config] Using production relative API:', API_BASE);
  } else if (_RAW_API_BASE) {
    // For localhost or other hostnames, use the configured base
    if (_RAW_API_BASE.includes('localhost') || _RAW_API_BASE.includes('127.0.0.1') || _RAW_API_BASE.includes('0.0.0.0')) {
      API_BASE = _RAW_API_BASE.replace('localhost', hostname).replace('127.0.0.1', hostname).replace('0.0.0.0', hostname);
    } else {
      API_BASE = _RAW_API_BASE;
    }
  } else {
    // Fallback for dev environments
    API_BASE = window.location.origin.replace(':5173', ':8080');
    console.warn('[API Config] VITE_API_BASE is undefined, falling back to:', API_BASE);
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
        credentials: 'include', // Send cookies (refresh token) with requests
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
        // Handle token expiry - dispatch event before throwing
        if (response.status === 401) {
          console.log('[API] Auth error detected, dispatching logout event');
          // Clear auth data immediately
          localStorage.removeItem('token');
          localStorage.removeItem('user');
          // Dispatch custom event for App.jsx to handle redirect
          window.dispatchEvent(new CustomEvent('auth:expired'));
          // Don't throw an error for token expiry - let the redirect handle it
          return {};
        }
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

  // OAuth - Get Google auth URL (redirect user here)
  getGoogleAuthURL() {
    return `${this.baseURL}/auth/google`;
  }

  // OAuth - Set username for new OAuth users
  async setUsername(username) {
    return this.request('/auth/set-username', {
      method: 'POST',
      body: JSON.stringify({ username }),
    });
  }

  // Users
  async getMe() {
    return this.request('/users/me');
  }

  async searchUsers(query) {
    return this.request(`/users/search?q=${encodeURIComponent(query)}`);
  }

  async updateProfile(displayName, avatarUrl) {
    return this.request('/users/me', {
      method: 'PUT',
      body: JSON.stringify({ display_name: displayName, avatar_url: avatarUrl }),
    });
  }

  async updatePreferences(showOnlineStatus, readReceiptsEnabled) {
    return this.request('/users/me/preferences', {
      method: 'PATCH',
      body: JSON.stringify({ show_online_status: showOnlineStatus, read_receipts_enabled: readReceiptsEnabled }),
    });
  }

  async deleteAccount() {
    return this.request('/users/me', {
      method: 'DELETE',
    });
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

  async deleteMessage(messageId) {
    return this.request(`/messages/${messageId}`, {
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

  // =========================================================================
  // Call History
  // =========================================================================
  
  async getCallHistory(limit = 50, offset = 0) {
    return this.request(`/calls?limit=${limit}&offset=${offset}`);
  }

  async getCall(callId) {
    return this.request(`/calls/${callId}`);
  }

  async getMissedCallCount() {
    return this.request('/calls/missed/count');
  }

  async createCall(conversationId, callType = 'video') {
    return this.request('/calls', {
      method: 'POST',
      body: JSON.stringify({ conversation_id: conversationId, call_type: callType }),
    });
  }

  async updateCall(callId, status) {
    return this.request(`/calls/${callId}`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    });
  }
}

export const API_BASE_URL = API_BASE;
export default new ApiService();
