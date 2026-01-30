import { useState } from 'react';
import api from '../services/api';

export default function Sidebar({
  user,
  conversations,
  currentConversation,
  onSelectConversation,
  onCreateDM,
  onLogout,
  isConnected
}) {
  const [showNewChat, setShowNewChat] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState([]);
  const [searching, setSearching] = useState(false);

  const handleSearch = async (query) => {
    setSearchQuery(query);
    if (query.length < 2) {
      setSearchResults([]);
      return;
    }

    setSearching(true);
    try {
      const data = await api.searchUsers(query);
      setSearchResults((data.users || []).filter(u => u.id !== user.id));
    } catch (error) {
      console.error('Search failed:', error);
    } finally {
      setSearching(false);
    }
  };

  const handleCreateDM = async (userId) => {
    try {
      await onCreateDM(userId);
      setShowNewChat(false);
      setSearchQuery('');
      setSearchResults([]);
    } catch (error) {
      alert(error.message);
    }
  };

  const getDMName = (conv) => {
    if (!conv.members) return 'Unknown';
    const other = conv.members.find(m => m.user_id !== user.id);
    if (other?.user) return other.user.username;
    if (other?.username) return other.username;
    return 'Unknown';
  };

  return (
    <>
      <div className="w-80 bg-white border-r border-gray-200 flex flex-col">
        {/* Header */}
        <div className="p-4 border-b border-gray-200">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <span className="text-2xl">🍵</span>
              <h2 className="text-xl font-bold text-gray-900">TeaTime</h2>
            </div>
            <button
              onClick={() => setShowNewChat(true)}
              className="p-2 hover:bg-gray-100 rounded-lg transition-colors"
              title="New Chat"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
            </button>
          </div>

          {/* Connection Status */}
          <div className="flex items-center gap-2 text-xs text-gray-500">
            <div className={`w-2 h-2 rounded-full ${isConnected ? 'bg-green-500' : 'bg-gray-300'}`}></div>
            <span>{isConnected ? 'Connected' : 'Offline'}</span>
          </div>
        </div>

        {/* Conversations List */}
        <div className="flex-1 overflow-y-auto">
          {conversations.length === 0 ? (
            <div className="p-4 text-center text-gray-500 text-sm">
              No conversations yet. Start a new chat!
            </div>
          ) : (
            conversations.map(conv => {
              const name = conv.type === 'dm' ? getDMName(conv) : conv.name || 'Group Chat';
              const isActive = currentConversation?.id === conv.id;

              return (
                <button
                  key={conv.id}
                  onClick={() => onSelectConversation(conv)}
                  className={`w-full p-4 flex items-center gap-3 border-b border-gray-100 hover:bg-gray-50 transition-colors ${
                    isActive ? 'bg-green-50' : ''
                  }`}
                >
                  <div className={`w-12 h-12 rounded-full flex items-center justify-center text-white font-semibold ${
                    isActive ? 'bg-green-600' : 'bg-green-500'
                  }`}>
                    {name.charAt(0).toUpperCase()}
                  </div>
                  <div className="flex-1 text-left min-w-0">
                    <div className="font-semibold text-gray-900 truncate">{name}</div>
                    {conv.last_message && (
                      <div className="text-sm text-gray-500 truncate">{conv.last_message}</div>
                    )}
                  </div>
                </button>
              );
            })
          )}
        </div>

        {/* User Footer */}
        <div className="p-4 border-t border-gray-200 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full bg-green-500 flex items-center justify-center text-white font-semibold">
              {user.username.charAt(0).toUpperCase()}
            </div>
            <span className="font-medium text-gray-900">{user.username}</span>
          </div>
          <button
            onClick={onLogout}
            className="p-2 hover:bg-gray-100 rounded-lg transition-colors text-gray-600"
            title="Logout"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
            </svg>
          </button>
        </div>
      </div>

      {/* New Chat Modal */}
      {showNewChat && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50" onClick={() => setShowNewChat(false)}>
          <div className="bg-white rounded-2xl w-full max-w-md mx-4 shadow-2xl" onClick={e => e.stopPropagation()}>
            <div className="p-6 border-b border-gray-200">
              <div className="flex items-center justify-between">
                <h3 className="text-xl font-semibold text-gray-900">New Chat</h3>
                <button
                  onClick={() => setShowNewChat(false)}
                  className="p-1 hover:bg-gray-100 rounded-lg transition-colors"
                >
                  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            </div>

            <div className="p-6">
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Search Users
              </label>
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => handleSearch(e.target.value)}
                placeholder="Enter username..."
                className="input"
                autoFocus
              />

              <div className="mt-4 space-y-2">
                {searching ? (
                  <div className="text-center py-4 text-gray-500">Searching...</div>
                ) : searchResults.length === 0 && searchQuery.length >= 2 ? (
                  <div className="text-center py-4 text-gray-500">No users found</div>
                ) : (
                  searchResults.map(user => (
                    <button
                      key={user.id}
                      onClick={() => handleCreateDM(user.id)}
                      className="w-full p-3 flex items-center gap-3 hover:bg-gray-50 rounded-lg transition-colors"
                    >
                      <div className="w-10 h-10 rounded-full bg-green-500 flex items-center justify-center text-white font-semibold">
                        {user.username.charAt(0).toUpperCase()}
                      </div>
                      <span className="font-medium text-gray-900">{user.username}</span>
                    </button>
                  ))
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
