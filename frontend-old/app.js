// TeaTime Chat Application
// Frontend for the Go backend chat API

(function() {
    'use strict';

    // Configuration
    const API_BASE = 'http://localhost:8080';
    const WS_BASE = 'ws://localhost:8080/ws';

    // State
    let state = {
        token: null,
        user: null,
        ws: null,
        conversations: [],
        currentConversation: null,
        messages: {},
        typingUsers: {}
    };

    // DOM Elements
    const elements = {};

    // Initialize
    function init() {
        cacheElements();
        bindEvents();
        checkAuth();
    }

    function cacheElements() {
        // Views
        elements.authView = document.getElementById('auth-view');
        elements.chatView = document.getElementById('chat-view');

        // Auth
        elements.loginForm = document.getElementById('login-form');
        elements.registerForm = document.getElementById('register-form');
        elements.loginError = document.getElementById('login-error');
        elements.registerError = document.getElementById('register-error');
        elements.authTabs = document.querySelectorAll('.tab');

        // Chat
        elements.conversationsList = document.getElementById('conversations-list');
        elements.currentUsername = document.getElementById('current-username');
        elements.emptyState = document.getElementById('empty-state');
        elements.chatContainer = document.getElementById('chat-container');
        elements.chatTitle = document.getElementById('chat-title');
        elements.chatTyping = document.getElementById('chat-typing');
        elements.messagesContainer = document.getElementById('messages-container');
        elements.messageForm = document.getElementById('message-form');
        elements.messageInput = document.getElementById('message-input');

        // Modal
        elements.newChatBtn = document.getElementById('new-chat-btn');
        elements.newChatModal = document.getElementById('new-chat-modal');
        elements.closeModal = document.getElementById('close-modal');
        elements.searchUsers = document.getElementById('search-users');
        elements.searchResults = document.getElementById('search-results');

        // Other
        elements.logoutBtn = document.getElementById('logout-btn');
        elements.connectionStatus = document.getElementById('connection-status');
    }

    function bindEvents() {
        // Auth tabs
        elements.authTabs.forEach(tab => {
            tab.addEventListener('click', () => switchAuthTab(tab.dataset.tab));
        });

        // Forms
        elements.loginForm.addEventListener('submit', handleLogin);
        elements.registerForm.addEventListener('submit', handleRegister);
        elements.messageForm.addEventListener('submit', handleSendMessage);

        // Modal
        elements.newChatBtn.addEventListener('click', () => toggleModal(true));
        elements.closeModal.addEventListener('click', () => toggleModal(false));
        elements.searchUsers.addEventListener('input', debounce(handleUserSearch, 300));

        // Logout
        elements.logoutBtn.addEventListener('click', handleLogout);

        // Typing indicator
        elements.messageInput.addEventListener('input', debounce(handleTyping, 500));

        // Enter key to send message
        elements.messageInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                elements.messageForm.dispatchEvent(new Event('submit'));
            }
        });

        // Click outside modal
        elements.newChatModal.addEventListener('click', (e) => {
            if (e.target === elements.newChatModal) toggleModal(false);
        });
    }

    // Auth Functions
    function checkAuth() {
        const token = localStorage.getItem('token');
        const user = localStorage.getItem('user');

        if (token && user) {
            state.token = token;
            state.user = JSON.parse(user);
            showChatView();
        }
    }

    async function handleLogin(e) {
        e.preventDefault();
        elements.loginError.textContent = '';

        const email = document.getElementById('login-email').value;
        const password = document.getElementById('login-password').value;

        try {
            const response = await fetch(`${API_BASE}/auth/login`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email, password })
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || 'Login failed');
            }

            saveAuth(data.access_token, data.user);
            showChatView();
        } catch (error) {
            elements.loginError.textContent = error.message;
        }
    }

    async function handleRegister(e) {
        e.preventDefault();
        elements.registerError.textContent = '';

        const username = document.getElementById('register-username').value;
        const email = document.getElementById('register-email').value;
        const password = document.getElementById('register-password').value;

        try {
            const response = await fetch(`${API_BASE}/auth/register`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, email, password })
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || 'Registration failed');
            }

            saveAuth(data.access_token, data.user);
            showChatView();
        } catch (error) {
            elements.registerError.textContent = error.message;
        }
    }

    function saveAuth(token, user) {
        state.token = token;
        state.user = user;
        localStorage.setItem('token', token);
        localStorage.setItem('user', JSON.stringify(user));
    }

    function handleLogout() {
        state.token = null;
        state.user = null;
        state.conversations = [];
        state.currentConversation = null;
        state.messages = {};
        
        localStorage.removeItem('token');
        localStorage.removeItem('user');

        if (state.ws) {
            state.ws.close();
            state.ws = null;
        }

        showAuthView();
    }

    function switchAuthTab(tab) {
        elements.authTabs.forEach(t => t.classList.toggle('active', t.dataset.tab === tab));
        elements.loginForm.classList.toggle('hidden', tab !== 'login');
        elements.registerForm.classList.toggle('hidden', tab !== 'register');
        elements.loginError.textContent = '';
        elements.registerError.textContent = '';
    }

    // View Functions
    function showAuthView() {
        elements.authView.classList.remove('hidden');
        elements.chatView.classList.add('hidden');
    }

    function showChatView() {
        elements.authView.classList.add('hidden');
        elements.chatView.classList.remove('hidden');
        elements.currentUsername.textContent = state.user.username;
        
        loadConversations();
        connectWebSocket();
    }

    // Conversations
    async function loadConversations() {
        try {
            const response = await fetch(`${API_BASE}/conversations`, {
                headers: { 'Authorization': `Bearer ${state.token}` }
            });

            if (!response.ok) {
                if (response.status === 401) return handleLogout();
                throw new Error('Failed to load conversations');
            }

            const data = await response.json();
            state.conversations = data.conversations || [];
            renderConversations();
        } catch (error) {
            console.error('Load conversations error:', error);
        }
    }

    function renderConversations() {
        elements.conversationsList.innerHTML = state.conversations.map(conv => {
            const name = conv.type === 'dm' 
                ? getDMName(conv) 
                : conv.name || 'Group Chat';
            const initial = name.charAt(0).toUpperCase();
            const isActive = state.currentConversation?.id === conv.id;

            return `
                <div class="conversation-item ${isActive ? 'active' : ''}" 
                     data-id="${conv.id}">
                    <div class="conversation-avatar">${initial}</div>
                    <div class="conversation-info">
                        <div class="conversation-name">${escapeHtml(name)}</div>
                        <div class="conversation-preview">${conv.last_message || 'No messages yet'}</div>
                    </div>
                </div>
            `;
        }).join('');

        // Bind click events
        elements.conversationsList.querySelectorAll('.conversation-item').forEach(item => {
            item.addEventListener('click', () => selectConversation(item.dataset.id));
        });
    }

    function getDMName(conv) {
        if (!conv.members) return 'Unknown';
        const other = conv.members.find(m => m.user_id !== state.user.id);
        return other ? other.username : 'Unknown';
    }

    async function selectConversation(id) {
        const conv = state.conversations.find(c => c.id === id);
        if (!conv) return;

        state.currentConversation = conv;
        renderConversations(); // Update active state

        elements.emptyState.classList.add('hidden');
        elements.chatContainer.classList.remove('hidden');

        const name = conv.type === 'dm' ? getDMName(conv) : conv.name || 'Group Chat';
        elements.chatTitle.textContent = name;

        // Join room via WebSocket
        sendWSMessage('room.join', { conversation_id: id });

        // Load messages
        await loadMessages(id);
    }

    async function loadMessages(conversationId) {
        try {
            const response = await fetch(`${API_BASE}/conversations/${conversationId}/messages`, {
                headers: { 'Authorization': `Bearer ${state.token}` }
            });

            if (!response.ok) throw new Error('Failed to load messages');

            const data = await response.json();
            state.messages[conversationId] = data.messages || [];
            renderMessages();
        } catch (error) {
            console.error('Load messages error:', error);
        }
    }

    function renderMessages() {
        if (!state.currentConversation) return;

        const messages = state.messages[state.currentConversation.id] || [];
        
        elements.messagesContainer.innerHTML = messages.map(msg => {
            const isSent = msg.sender_id === state.user.id;
            const time = new Date(msg.created_at).toLocaleTimeString([], { 
                hour: '2-digit', 
                minute: '2-digit' 
            });

            return `
                <div class="message ${isSent ? 'sent' : 'received'}">
                    ${!isSent ? `<div class="message-sender">${escapeHtml(msg.sender_username || 'User')}</div>` : ''}
                    <div class="message-content">${escapeHtml(msg.body_text || msg.content)}</div>
                    <div class="message-time">${time}</div>
                </div>
            `;
        }).join('');

        // Scroll to bottom
        elements.messagesContainer.scrollTop = elements.messagesContainer.scrollHeight;
    }

    // Send Message
    async function handleSendMessage(e) {
        e.preventDefault();
        
        const content = elements.messageInput.value.trim();
        if (!content || !state.currentConversation) return;

        elements.messageInput.value = '';

        // Send via WebSocket for real-time
        sendWSMessage('message.send', {
            conversation_id: state.currentConversation.id,
            body_text: content
        });
    }

    // User Search
    async function handleUserSearch() {
        const query = elements.searchUsers.value.trim();
        if (query.length < 2) {
            elements.searchResults.innerHTML = '';
            return;
        }

        try {
            const response = await fetch(`${API_BASE}/users/search?q=${encodeURIComponent(query)}`, {
                headers: { 'Authorization': `Bearer ${state.token}` }
            });

            if (!response.ok) throw new Error('Search failed');

            const data = await response.json();
            renderSearchResults(data.users || []);
        } catch (error) {
            console.error('User search error:', error);
        }
    }

    function renderSearchResults(users) {
        // Filter out current user
        const filteredUsers = users.filter(u => u.id !== state.user.id);

        if (filteredUsers.length === 0) {
            elements.searchResults.innerHTML = '<p class="text-muted">No users found</p>';
            return;
        }

        elements.searchResults.innerHTML = filteredUsers.map(user => `
            <div class="search-result-item" data-id="${user.id}" data-username="${escapeHtml(user.username)}">
                <div class="search-result-avatar">${user.username.charAt(0).toUpperCase()}</div>
                <div class="search-result-name">${escapeHtml(user.username)}</div>
            </div>
        `).join('');

        // Bind click events
        elements.searchResults.querySelectorAll('.search-result-item').forEach(item => {
            item.addEventListener('click', () => createDM(item.dataset.id));
        });
    }

    async function createDM(userId) {
        try {
            const response = await fetch(`${API_BASE}/conversations`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${state.token}`
                },
                body: JSON.stringify({
                    type: 'dm',
                    member_ids: [userId]
                })
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || 'Failed to create chat');
            }

            const data = await response.json();
            
            toggleModal(false);
            elements.searchUsers.value = '';
            elements.searchResults.innerHTML = '';

            // Reload conversations and select the new one
            await loadConversations();
            selectConversation(data.id);
        } catch (error) {
            console.error('Create DM error:', error);
            alert(error.message);
        }
    }

    // WebSocket
    function connectWebSocket() {
        if (state.ws?.readyState === WebSocket.OPEN) return;

        state.ws = new WebSocket(WS_BASE);

        state.ws.onopen = () => {
            console.log('WebSocket connected');
            elements.connectionStatus.classList.add('hidden');
            
            // Authenticate
            sendWSMessage('auth', { token: state.token });
        };

        state.ws.onmessage = (event) => {
            const msg = JSON.parse(event.data);
            handleWSMessage(msg);
        };

        state.ws.onclose = () => {
            console.log('WebSocket disconnected');
            elements.connectionStatus.classList.remove('hidden');
            
            // Reconnect after delay
            setTimeout(connectWebSocket, 3000);
        };

        state.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
    }

    function sendWSMessage(type, payload) {
        if (state.ws?.readyState !== WebSocket.OPEN) {
            console.warn('WebSocket not connected');
            return;
        }

        state.ws.send(JSON.stringify({ type, payload }));
    }

    function handleWSMessage(msg) {
        switch (msg.type) {
            case 'auth.success':
                console.log('WebSocket authenticated');
                // Join all conversation rooms
                state.conversations.forEach(conv => {
                    sendWSMessage('room.join', { conversation_id: conv.id });
                });
                break;

            case 'message.new':
                handleNewMessage(msg.payload);
                break;

            case 'typing':
                handleTypingIndicator(msg.payload);
                break;

            case 'error':
                console.error('WebSocket error:', msg.payload);
                break;
        }
    }

    function handleNewMessage(payload) {
        const convId = payload.conversation_id;
        
        // Add to local state
        if (!state.messages[convId]) {
            state.messages[convId] = [];
        }
        state.messages[convId].push(payload);

        // Re-render if current conversation
        if (state.currentConversation?.id === convId) {
            renderMessages();
        }

        // Update conversation preview
        const conv = state.conversations.find(c => c.id === convId);
        if (conv) {
            conv.last_message = payload.body_text;
            renderConversations();
        }
    }

    function handleTyping(e) {
        if (!state.currentConversation) return;

        const isTyping = e.target.value.length > 0;
        sendWSMessage(isTyping ? 'typing.start' : 'typing.stop', {
            conversation_id: state.currentConversation.id
        });
    }

    function handleTypingIndicator(payload) {
        if (!state.currentConversation) return;
        if (payload.conversation_id !== state.currentConversation.id) return;
        if (payload.user_id === state.user.id) return;

        if (payload.is_typing) {
            state.typingUsers[payload.user_id] = payload.username;
        } else {
            delete state.typingUsers[payload.user_id];
        }

        const typingNames = Object.values(state.typingUsers);
        if (typingNames.length > 0) {
            elements.chatTyping.textContent = `${typingNames.join(', ')} typing...`;
        } else {
            elements.chatTyping.textContent = '';
        }
    }

    // Modal
    function toggleModal(show) {
        elements.newChatModal.classList.toggle('hidden', !show);
        if (show) {
            elements.searchUsers.focus();
        }
    }

    // Utilities
    function debounce(fn, delay) {
        let timeoutId;
        return function(...args) {
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => fn.apply(this, args), delay);
        };
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    // Start
    document.addEventListener('DOMContentLoaded', init);
})();
