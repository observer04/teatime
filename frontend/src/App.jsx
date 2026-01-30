import { useAuth } from './hooks/useAuth';
import AuthPage from './components/AuthPage';
import ChatLayout from './components/ChatLayout';

function App() {
  const { user, token, loading, login, logout, isAuthenticated } = useAuth();

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="text-center">
          <div className="text-6xl mb-4 animate-bounce">🍵</div>
          <p className="text-gray-600">Loading...</p>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <AuthPage onLogin={login} />;
  }

  return <ChatLayout user={user} token={token} onLogout={logout} />;
}

export default App;
