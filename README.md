# 🍵 TeaTime

A real-time chat application with group messaging and video calls, built with Go and React .

## Features

### ✅ Implemented (Stage 1)

- **Authentication**: JWT-based auth with refresh tokens
- **Direct Messages**: 1-on-1 private conversations
- **Group Chat**: Create groups, add/remove members, rename
- **Real-time Messaging**: WebSocket-based instant delivery
- **Typing Indicators**: See when others are typing
- **Interface-Driven PubSub**: Swappable message bus (in-memory, Redis-ready)

### 🚧 In Progress (Stage 2)

- **File Sharing**: Cloudflare R2 presigned uploads
- **Video Calls**: WebRTC with Pion SFU

## Architecture

### Backend (Go)

```
backend/
├── cmd/server/          # Application entrypoint
├── internal/
│   ├── api/             # HTTP handlers
│   ├── auth/            # JWT, middleware
│   ├── config/          # Environment config
│   ├── database/        # Postgres repositories
│   ├── domain/          # Domain models & errors
│   ├── pubsub/          # Interface-driven pub/sub
│   ├── server/          # HTTP server setup & routes
│   ├── webrtc/          # Video call manager (modular monolith)
│   └── websocket/       # WebSocket hub & handlers
└── migrations/          # SQL migrations
```

### Frontend (React + Vite)

```
frontend/
└── src/
    ├── components/      # React components
    ├── hooks/           # Custom hooks
    └── services/        # API & WebSocket clients
```

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.21+ (for local development)
- Node.js 20+ (for frontend development)

### Development

```bash
# Start all services (Postgres, Backend, Frontend, Coturn)
make dev

# View logs
make logs

# Stop services
make dev-down
```

### Access

- Frontend: http://localhost:5173
- Backend API: http://localhost:8080
- Health check: http://localhost:8080/healthz

## API Endpoints

### Authentication

| Method | Endpoint         | Description    |
| ------ | ---------------- | -------------- |
| POST   | `/auth/register` | Create account |
| POST   | `/auth/login`    | Login          |
| POST   | `/auth/refresh`  | Refresh token  |
| POST   | `/auth/logout`   | Logout         |

### Conversations

| Method | Endpoint                             | Description               |
| ------ | ------------------------------------ | ------------------------- |
| GET    | `/conversations`                     | List user's conversations |
| POST   | `/conversations`                     | Create DM or group        |
| GET    | `/conversations/:id`                 | Get conversation details  |
| PATCH  | `/conversations/:id`                 | Update group (title)      |
| POST   | `/conversations/:id/members`         | Add member to group       |
| DELETE | `/conversations/:id/members/:userId` | Remove member             |

### Messages

| Method | Endpoint                      | Description              |
| ------ | ----------------------------- | ------------------------ |
| GET    | `/conversations/:id/messages` | Get messages (paginated) |
| POST   | `/conversations/:id/messages` | Send message             |

### WebSocket Events

Connect to `/ws` with JWT auth token.

#### Client → Server

- `auth` - Authenticate connection
- `room.join` - Join conversation room
- `room.leave` - Leave room
- `message.send` - Send message
- `typing.start` / `typing.stop` - Typing indicators
- `call.join` / `call.leave` - Video call signaling

#### Server → Client

- `auth.success` - Authentication confirmed
- `message.new` - New message in room
- `typing` - User typing status
- `call.config` - ICE servers for WebRTC

## Configuration

See [backend/.env.example](backend/.env.example) for all options.

Key variables:

```bash
DATABASE_URL=postgres://...
JWT_SIGNING_KEY=...  # min 32 chars
ICE_STUN_URLS=stun:stun.l.google.com:19302
ICE_TURN_URLS=turn:your-server:3478
TURN_USERNAME=...
TURN_PASSWORD=...
```

## Testing

```bash
make test          # Run all tests
make lint          # Check code style
make lint-fix      # Auto-fix formatting
```

## Deployment

### Oracle Cloud Free Tier

1. Provision ARM VM (4 OCPU, 24GB RAM free tier)
2. Set up DNS: `yourdomain.com`, `api.yourdomain.com`, `calls.yourdomain.com`
3. Configure firewall: 80, 443 TCP + UDP range for WebRTC
4. Use Caddy for automatic HTTPS
5. Deploy with docker-compose or systemd

See [infra/](infra/) for Coturn config and deployment templates.

## Design Decisions

### Modular Monolith

WebRTC/SFU runs in the same process as the API server, sharing auth context and simplifying deployment.

### Interface-Driven PubSub

The `pubsub.PubSub` interface allows swapping implementations:

- **Development**: In-memory map (single instance)
- **Production**: Redis Pub/Sub (multi-instance scaling)

### Coturn for TURN

Self-hosted TURN server for reliable NAT traversal on Oracle Cloud.

## License

MIT
