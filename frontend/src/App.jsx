import { useState, useEffect } from 'react';
import { useAuth } from './hooks/useAuth';
import AuthPage from './components/AuthPage';
import SetUsernamePage from './components/SetUsernamePage';
import GlassmorphismChatLayout from './components/GlassmorphismChatLayout';
import api from './services/api';

function App() {
  console.log('[APP] Rendering App component');
  const { user, token, loading, login, logout, isAuthenticated } = useAuth();
  const [oauthChecking, setOauthChecking] = useState(true);
  const [showUsernameSetup, setShowUsernameSetup] = useState(false);
  const [oauthError, setOauthError] = useState('');

  // Handle OAuth Redirect
  useEffect(() => {
    const handleOAuth = async () => {
      const hash = window.location.hash;
      if (!hash) {
        setOauthChecking(false);
        return;
      }

      console.log('[APP] Parsing URL hash for OAuth params');
      const params = new URLSearchParams(hash.substring(1)); // Remove #
      const accessToken = params.get('oauth_token');
      const error = params.get('oauth_error');
      const needsUsername = params.get('needs_username') === 'true';

      // Clear hash to clean up URL
      window.history.replaceState(null, '', window.location.pathname);

      if (error) {
        setOauthError(decodeURIComponent(error));
        setOauthChecking(false);
        return;
      }

      if (accessToken) {
        try {
          // Temporarily store token for API requests
          localStorage.setItem('token', accessToken);
          
          if (needsUsername) {
            console.log('[APP] User needs to set username');
            setShowUsernameSetup(true);
          } else {
            console.log('[APP] Fetching user profile');
            const userProfile = await api.getMe();
            login(accessToken, userProfile);
          }
        } catch (err) {
          console.error('[APP] Failed to complete OAuth login:', err);
          setOauthError('Failed to load user profile');
          localStorage.removeItem('token'); // Cleanup invalid token
        }
      }
      setOauthChecking(false);
    };

    handleOAuth();
  }, [login]); // Expect login to be stable

  console.log('[APP] Auth state:', { user, token, loading, isAuthenticated, oauthChecking });

  if (loading || oauthChecking) {
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

  if (showUsernameSetup) {
    return (
      <SetUsernamePage 
        onComplete={(token, user) => {
          login(token, user);
          setShowUsernameSetup(false);
        }} 
      />
    );
  }

  if (!isAuthenticated) {
    console.log('[APP] Showing auth page');
    return (
      <>
        {oauthError && (
          <div className="fixed top-4 left-1/2 transform -translate-x-1/2 z-50 bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded shadow-lg">
            <span className="block sm:inline">{oauthError}</span>
            <span className="absolute top-0 bottom-0 right-0 px-4 py-3" onClick={() => setOauthError('')}>
              <svg className="fill-current h-6 w-6 text-red-500" role="button" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><title>Close</title><path d="M14.348 14.849a1.2 1.2 0 0 1-1.697 0L10 11.819l-2.651 3.029a1.2 1.2 0 1 1-1.697-1.697l2.758-3.15-2.759-3.152a1.2 1.2 0 1 1 1.697-1.697L10 8.183l2.651-3.031a1.2 1.2 0 1 1 1.697 1.697l-2.758 3.152 2.758 3.15a1.2 1.2 0 0 1 0 1.698z"/></svg>
            </span>
          </div>
        )}
        <AuthPage onLogin={login} />
      </>
    );
  }

  console.log('[APP] Showing glassmorphism chat layout');
  return <GlassmorphismChatLayout user={user} token={token} onLogout={logout} />;
}

export default App;


