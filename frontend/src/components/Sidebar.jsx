import { useState } from 'react';
import api from '../services/api';

export default function Sidebar({
  user,
  conversations,
  currentConversation,
  onSelectConversation,
  onCreateDM,
  onCreateGroup,
  onLogout,
  isConnected
}) {
  const [showNewChat, setShowNewChat] = useState(false);
  const [chatMode, setChatMode] = useState('dm'); // 'dm' or 'group'
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState([]);
  const [searching, setSearching] = useState(false);
  const [groupTitle, setGroupTitle] = useState('');
  const [selectedMembers, setSelectedMembers] = useState([]);

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
      closeModal();
    } catch (error) {
      alert(error.message);
    }
  };

  const handleSelectMember = (member) => {
    if (chatMode === 'dm') {
      handleCreateDM(member.id);
    } else {
      // Toggle selection for group
      if (selectedMembers.find(m => m.id === member.id)) {
        setSelectedMembers(selectedMembers.filter(m => m.id !== member.id));
      } else {
        setSelectedMembers([...selectedMembers, member]);
      }
    }
  };

  const handleCreateGroup = async () => {
    if (!groupTitle.trim()) {
      alert('Please enter a group name');
      return;
    }
    if (selectedMembers.length === 0) {
      alert('Please select at least one member');
      return;
    }
    try {
      await onCreateGroup(groupTitle, selectedMembers.map(m => m.id));
      closeModal();
    } catch (error) {
      alert(error.message);
    }
  };

  const closeModal = () => {
    setShowNewChat(false);
    setSearchQuery('');
    setSearchResults([]);
    setGroupTitle('');
    setSelectedMembers([]);
    setChatMode('dm');
  };

  const getConversationName = (conv) => {
    if (conv.type === 'group') {
      return conv.title || 'Group Chat';
    }
    // DM - get the other person's name
    if (!conv.members) return 'Unknown';
    const other = conv.members.find(m => m.user_id !== user.id);
    if (other?.user) return other.user.username;
    if (other?.username) return other.username;
    return 'Unknown';
  };

  const getConversationAvatar = (conv) => {
    if (conv.type === 'group') {
      return '👥';
    }
    const name = getConversationName(conv);
    return name.charAt(0).toUpperCase();
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
              const name = getConversationName(conv);
              const avatar = getConversationAvatar(conv);
              const isActive = currentConversation?.id === conv.id;
              const memberCount = conv.members?.length || 0;

              return (
                <button
                  key={conv.id}
                  onClick={() => onSelectConversation(conv)}
                  className={`w-full p-4 flex items-center gap-3 border-b border-gray-100 hover:bg-gray-50 transition-colors ${
                    isActive ? 'bg-green-50' : ''
                  }`}
                >
                  <div className={`w-12 h-12 rounded-full flex items-center justify-center text-white font-semibold ${
                    conv.type === 'group' ? 'bg-purple-500 text-lg' : (isActive ? 'bg-green-600' : 'bg-green-500')
                  }`}>
                    {avatar}
                  </div>
                  <div className="flex-1 text-left min-w-0">
                    <div className="font-semibold text-gray-900 truncate">{name}</div>
                    <div className="text-sm text-gray-500 truncate">
                      {conv.type === 'group' ? `${memberCount} members` : conv.last_message || ''}
                    </div>
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
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50" onClick={closeModal}>
          <div className="bg-white rounded-2xl w-full max-w-md mx-4 shadow-2xl" onClick={e => e.stopPropagation()}>
            <div className="p-6 border-b border-gray-200">
              <div className="flex items-center justify-between">
                <h3 className="text-xl font-semibold text-gray-900">New Chat</h3>
                <button
                  onClick={closeModal}
                  className="p-1 hover:bg-gray-100 rounded-lg transition-colors"
                >
                  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
              
              {/* Chat Mode Toggle */}
              <div className="flex mt-4 bg-gray-100 rounded-lg p-1">
                <button
                  onClick={() => { setChatMode('dm'); setSelectedMembers([]); }}
                  className={`flex-1 py-2 rounded-md text-sm font-medium transition-colors ${
                    chatMode === 'dm' ? 'bg-white shadow text-gray-900' : 'text-gray-600'
                  }`}
                >
                  Direct Message
                </button>
                <button
                  onClick={() => setChatMode('group')}
                  className={`flex-1 py-2 rounded-md text-sm font-medium transition-colors ${
                    chatMode === 'group' ? 'bg-white shadow text-gray-900' : 'text-gray-600'
                  }`}
                >
                  New Group
                </button>
              </div>
            </div>

            <div className="p-6">
              {/* Group Title Input */}
              {chatMode === 'group' && (
                <div className="mb-4">
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Group Name
                  </label>
                  <input
                    type="text"
                    value={groupTitle}
                    onChange={(e) => setGroupTitle(e.target.value)}
                    placeholder="Enter group name..."
                    className="input"
                  />
                </div>
              )}

              {/* Selected Members for Group */}
              {chatMode === 'group' && selectedMembers.length > 0 && (
                <div className="mb-4">
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Selected Members ({selectedMembers.length})
                  </label>
                  <div className="flex flex-wrap gap-2">
                    {selectedMembers.map(member => (
                      <span 
                        key={member.id}
                        className="inline-flex items-center gap-1 px-3 py-1 bg-green-100 text-green-800 rounded-full text-sm"
                      >
                        {member.username}
                        <button 
                          onClick={() => setSelectedMembers(selectedMembers.filter(m => m.id !== member.id))}
                          className="hover:text-green-600"
                        >
                          ×
                        </button>
                      </span>
                    ))}
                  </div>
                </div>
              )}

              <label className="block text-sm font-medium text-gray-700 mb-2">
                {chatMode === 'group' ? 'Add Members' : 'Search Users'}
              </label>
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => handleSearch(e.target.value)}
                placeholder="Enter username..."
                className="input"
                autoFocus
              />

              <div className="mt-4 space-y-2 max-h-48 overflow-y-auto">
                {searching ? (
                  <div className="text-center py-4 text-gray-500">Searching...</div>
                ) : searchResults.length === 0 && searchQuery.length >= 2 ? (
                  <div className="text-center py-4 text-gray-500">No users found</div>
                ) : (
                  searchResults.map(foundUser => {
                    const isSelected = selectedMembers.find(m => m.id === foundUser.id);
                    return (
                      <button
                        key={foundUser.id}
                        onClick={() => handleSelectMember(foundUser)}
                        className={`w-full p-3 flex items-center gap-3 rounded-lg transition-colors ${
                          isSelected ? 'bg-green-50 border border-green-200' : 'hover:bg-gray-50'
                        }`}
                      >
                        <div className="w-10 h-10 rounded-full bg-green-500 flex items-center justify-center text-white font-semibold">
                          {foundUser.username.charAt(0).toUpperCase()}
                        </div>
                        <span className="font-medium text-gray-900 flex-1 text-left">{foundUser.username}</span>
                        {chatMode === 'group' && isSelected && (
                          <svg className="w-5 h-5 text-green-600" fill="currentColor" viewBox="0 0 20 20">
                            <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                          </svg>
                        )}
                      </button>
                    );
                  })
                )}
              </div>

              {/* Create Group Button */}
              {chatMode === 'group' && (
                <button
                  onClick={handleCreateGroup}
                  disabled={!groupTitle.trim() || selectedMembers.length === 0}
                  className="w-full mt-4 btn-primary disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Create Group ({selectedMembers.length} members)
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
