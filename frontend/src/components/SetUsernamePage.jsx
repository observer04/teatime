import { useState } from 'react';
import api from '../services/api';

export default function SetUsernamePage({ onComplete }) {
  const [username, setUsername] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const data = await api.setUsername(username);
      onComplete(data.access_token, data.user);
    } catch (err) {
      setError(err.message || 'Failed to set username');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-green-50 via-emerald-50 to-teal-50 p-4">
      <div className="w-full max-w-md">
        <div className="bg-white rounded-2xl shadow-xl p-8">
          <div className="text-center mb-8">
            <div className="inline-flex items-center justify-center w-16 h-16 bg-green-500 rounded-2xl mb-4">
              <span className="text-3xl">👤</span>
            </div>
            <h1 className="text-2xl font-bold text-gray-900">Choose a Username</h1>
            <p className="text-gray-600 mt-2">Almost there! Pick a unique username to continue.</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Username
              </label>
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="input"
                required
                minLength={3}
                maxLength={32}
                pattern="[a-zA-Z0-9_]+"
                title="Letters, numbers, and underscores only"
                placeholder="e.g. tea_lover_42"
              />
            </div>

            {error && (
              <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg text-sm">
                {error}
              </div>
            )}

            <button
              type="submit"
              disabled={loading}
              className="btn btn-primary w-full disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? 'Setting Username...' : 'Continue'}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
