import { useAuth } from './hooks/useAuth';
import AuthPage from './components/AuthPage';
import GlassmorphismChatLayout from './components/GlassmorphismChatLayout';

function App() {
  console.log('[APP] Rendering App component');
  const { user, token, loading, login, logout, isAuthenticated } = useAuth();
  
  console.log('[APP] Auth state:', { user, token, loading, isAuthenticated });

  if (loading) {
    console.log('[APP] Showing loading state');
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="text-center">
          <div className="text-6xl mb-4 animate-bounce">🍵</div>
          <p className="text-muted-foreground">Loading...</p>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    console.log('[APP] Showing auth page');
    return <AuthPage onLogin={login} />;
  }

  console.log('[APP] Showing glassmorphism chat layout');
  return <GlassmorphismChatLayout user={user} token={token} onLogout={logout} />;
}

export default App;


