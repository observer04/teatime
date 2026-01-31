# TeaTime Frontend

A React-based chat application frontend built with Vite.

## Features

- **Direct Messages (DM)**: 1-on-1 private conversations
- **Group Chat**: Create and manage group conversations
  - Create groups with custom names
  - Add/remove members
  - View member list with roles (admin/member)
- **Real-time Messaging**: WebSocket-based instant messaging
- **Typing Indicators**: See when others are typing
- **Responsive Design**: Tailwind CSS-based styling

## Development

```bash
# Install dependencies
npm install

# Start dev server
npm run dev

# Build for production
npm run build
```

## Environment Variables

Create a `.env` file with:

```
VITE_API_BASE=http://localhost:8080
VITE_WS_BASE=ws://localhost:8080/ws
```

## Architecture

- `src/components/` - React components
  - `AuthPage.jsx` - Login/Register forms
  - `ChatLayout.jsx` - Main chat layout orchestrator
  - `Sidebar.jsx` - Conversation list + new chat modal
  - `ChatWindow.jsx` - Message display + input
- `src/hooks/` - Custom React hooks
  - `useAuth.js` - Authentication state management
  - `useWebSocket.js` - WebSocket connection + message handling
- `src/services/` - API and WebSocket clients
  - `api.js` - REST API client
  - `websocket.js` - WebSocket wrapper

## Tech Stack

- React 18
- Vite
- Tailwind CSS
- WebSocket (native)
