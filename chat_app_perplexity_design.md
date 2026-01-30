# design_plan.md — Realtime Web Chat MVP (Go-first, $0 budget)

## Product definition (MVP)
**Goal:** A consumer-facing realtime chat web app with username discovery and group chats, usable on day 1.

In-scope (MVP):
- OAuth login (GitHub, optionally Google) → issue your own JWTs.
- Usernames (unique), public profile page, user search.
- 1:1 chats + group chats from the start.
- Realtime messaging via WebSockets + message history.
- Basic “delivered/read” receipts (keep it simple).
- Basic moderation: block user, report user (store report record).

Out-of-scope (later):
- E2EE, voice/video, bots, federation, advanced moderation tools, multi-device crypto.

Non-goals (MVP):
- Perfect scalability beyond one VM.
- Complex event sourcing / CQRS.

---

## Stage 0 — Repo + local dev baseline

### Deliverables
- Monorepo with `backend/` and `web/`.
- One-command local run (Docker compose).

### Tasks
- (Agent) Create repo structure:
  - `backend/` (Go)
  - `web/` (React + TypeScript + Vite)
  - `infra/` (docker-compose, Caddyfile templates, scripts)
  - `docs/` (architecture notes, API docs)
- (Agent) Add `docker-compose.yml` for:
  - `postgres`
  - `backend` (Go app)
  - `web` (dev server) OR just run locally without container
- (Agent) Add basic Makefile:
  - `make dev`, `make test`, `make lint`, `make migrate`
- (Manual) Install local prerequisites:
  - Go, Node.js, Docker Desktop / Docker Engine
- (Agent) Add GitHub Actions:
  - Lint + test on PR
  - Build backend binary
  - Build web

Acceptance criteria
- `make dev` starts Postgres + backend and you can hit `GET /healthz`.
- Web runs locally and can call backend `/healthz`.

---

## Stage 1 — Data model + migrations (Postgres)

### Deliverables
- Postgres schema + migrations.
- Minimal data access layer in Go.

### Tasks
- (Agent) Choose migration tool (e.g., `golang-migrate`) and set up:
  - `backend/migrations/*.sql`
- (Agent) Define tables (minimal version):
  - `users`:
    - `id (uuid pk)`, `username (unique)`, `display_name`, `avatar_url`
    - `created_at`, `updated_at`
  - `oauth_identities`:
    - `id`, `user_id`, `provider`, `provider_user_id`, `created_at`
  - `conversations`:
    - `id`, `type` ENUM('dm','group')
    - `title` (nullable), `created_by`, `created_at`
  - `conversation_members`:
    - `conversation_id`, `user_id`, `role` ENUM('member','admin')
    - `joined_at`, composite unique
  - `messages`:
    - `id (uuid)`, `conversation_id`, `sender_id`
    - `body_text`, `created_at`
  - `message_receipts`:
    - `message_id`, `user_id`, `delivered_at`, `read_at`
  - `blocks`:
    - `blocker_id`, `blocked_id`, `created_at`
  - `reports`:
    - `id`, `reporter_id`, `reported_user_id`, `reason`, `created_at`
- (Agent) Add indexes:
  - `messages(conversation_id, created_at desc)`
  - `users(username)`
  - `conversation_members(user_id)`

Acceptance criteria
- Migrations run cleanly.
- Can create a user + create a conversation + insert a message in an integration test.

---

## Stage 2 — Auth (OAuth → JWT-only app sessions)

### Deliverables
- OAuth login endpoints.
- JWT issuing + verification middleware.
- Frontend login flow.

### Decisions
- “JWT only” means: no server-side session store required for *access* (stateless access JWT).
- Use refresh tokens (also JWT) + rotation (store refresh token `jti` hash in DB) to support logout/revocation without Redis.

### Tasks
- (Manual) Create OAuth apps:
  - GitHub OAuth App: set callback URL to `https://api.<your-domain>/auth/github/callback`
  - (Optional) Google OAuth Client
- (Manual) Set secrets in GitHub repo / server env:
  - `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`
  - `JWT_SIGNING_KEY` (or RSA private key path)
  - `APP_BASE_URL`, `API_BASE_URL`
- (Agent) Implement backend endpoints:
  - `GET /auth/github/start` (redirect to provider)
  - `GET /auth/github/callback` (exchange code → create/find user → issue tokens)
  - `POST /auth/refresh` (rotate refresh token)
  - `POST /auth/logout` (revoke refresh token)
  - `GET /me` (returns current user)
- (Agent) JWT storage strategy (web):
  - Set access token in memory (JS) OR short-lived cookie
  - Set refresh token in `HttpOnly; Secure; SameSite=Lax` cookie
- (Agent) Add username claiming step:
  - On first login, user is forced to pick a unique username.

Acceptance criteria
- New user can log in and set username.
- `/me` works with JWT.
- Refresh works and logout invalidates refresh.

---

## Stage 3 — Core chat API (REST)

### Deliverables
- Conversation + message APIs.
- Permission checks (member-only access).

### Tasks
- (Agent) Implement REST endpoints:
  - Users:
    - `GET /users/search?q=...`
    - `GET /users/:username`
    - `POST /users/me` (update profile: display_name, avatar_url)
  - Conversations:
    - `POST /conversations` (create group or DM)
    - `GET /conversations` (list user’s conversations)
    - `GET /conversations/:id` (metadata + members)
    - `POST /conversations/:id/members` (add member)
    - `DELETE /conversations/:id/members/:userId` (remove member)
  - Messages:
    - `GET /conversations/:id/messages?before=<ts>&limit=50`
    - `POST /conversations/:id/messages`
    - `POST /messages/:id/read` (set read receipt)
- (Agent) Add block logic:
  - `POST /blocks/:username` and prevent DM creation / message delivery from blocked users.
- (Agent) Add report endpoint:
  - `POST /reports` (store record, no automation yet)

Acceptance criteria
- Group chat can be created, members added, messages sent and fetched with pagination.

---

## Stage 4 — Realtime layer (WebSockets)

### Deliverables
- WebSocket gateway: join rooms, receive messages, typing, receipts.
- Client reconnect logic.

### Tasks
- (Agent) WebSocket endpoint:
  - `GET /ws` upgrades to WS
  - Auth: read JWT from cookie (or send as first message `{type:"auth"}`) and bind socket to `user_id`
- (Agent) Define event protocol (JSON for MVP):
  - Client → Server:
    - `auth`, `room.join`, `room.leave`
    - `message.send`, `typing.start`, `typing.stop`
    - `receipt.read`
  - Server → Client:
    - `message.new`, `typing`, `receipt.updated`, `room.member_joined`
- (Agent) Fanout design (single VM):
  - Maintain in-memory map: `conversation_id -> set of sockets`
  - On `message.send`:
    - Validate membership
    - Insert message in DB
    - Broadcast `message.new` to room sockets
- (Agent) Optional improvement:
  - When messages are sent via REST (not WS), notify the WS broadcaster using Postgres `NOTIFY` so all sockets get updates.

Acceptance criteria
- Two browser tabs in same group chat see messages instantly.
- Reconnect restores room subscriptions and continues receiving messages.

---

## Stage 5 — Frontend (React + TypeScript)

### Deliverables
- Production-quality UI (clean, responsive), consumer-facing.
- Core pages: login, username setup, chat list, chat room, user search/profile.

### Tasks
- (Agent) Web app routes:
  - `/login`
  - `/setup-username`
  - `/u/:username`
  - `/app` (shell: sidebar + main view)
  - `/app/c/:conversationId`
- (Agent) State management:
  - Query cache for REST (TanStack Query recommended)
  - WebSocket client singleton with:
    - exponential backoff reconnect
    - room resubscribe
- (Agent) UX features (MVP level):
  - Message list virtualization (optional)
  - Loading states, empty states
  - “Create group” modal, add members via username search
  - Typing indicator (optional)
- (Manual) Decide branding:
  - App name, logo, color palette (keep it simple)

Acceptance criteria
- A new user can discover another by username, create group, chat in realtime.

---

## Stage 6 — Deployment (Oracle VM) + ops hardening

### Deliverables
- Public deployment with HTTPS, domain, logs, monitoring basics.

### Tasks
- (Manual) Oracle Always Free VM setup:
  - Create instance, open ports 80/443, SSH access
  - Attach domain: DNS A record to VM IP
- (Manual) Install on VM:
  - Docker + docker compose OR systemd services (choose one)
  - Caddy (reverse proxy) with HTTPS
- (Agent) Provide `infra/` deploy scripts:
  - `deploy.sh` (build, rsync, restart)
  - `Caddyfile` template routing:
    - `api.<domain>` → Go backend
    - `<domain>` → static web build
- (Manual) Set production secrets on VM:
  - OAuth secrets, JWT signing key, DB password
- (Agent) Add basic observability:
  - structured logs (JSON)
  - request IDs
  - `/metrics` (Prometheus format) optional
- (Agent) Add safety controls:
  - rate limits (login, message send)
  - max message size, max group size for MVP
  - basic input validation + escaping rules for rendering

Acceptance criteria
- `https://<domain>` works and you can chat with a friend end-to-end.

---

## Stage 7 — “MVP polish” (what makes people keep it)
Pick 3–5 only, to avoid scope creep.

Options:
- Message search (per conversation).
- Reactions + reply-to message.
- Invite links for group chats.
- Upload: image thumbnails (client-side) + file size caps.
- Minimal admin panel for reports (view-only).

---

## What the coding agent should do vs you (rule of thumb)
Coding agent handles:
- Backend endpoints, DB migrations, WebSocket gateway, frontend UI, tests, CI.

You handle manually:
- Cloud accounts (Oracle VM), DNS, OAuth app creation, secrets, and one-time server bootstrap.

---

## Suggested initial milestones (fastest path)
Milestone 1: Auth + username + user search + basic group creation (no realtime yet).
Milestone 2: WebSocket realtime messaging for groups + history page.
Milestone 3: Deploy + invite a few friends + fix UX/bugs based on real usage.
