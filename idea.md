# Project: "GopherChat" (Working Title)
**Goal:** Build a production-grade, distributed chat application that serves as both a usable consumer product and a high-level software engineering portfolio piece.

**Core Philosophy:**
1.  **User First:** The app must feel snappy, look good, and handle errors gracefully.
2.  **Engineering Depth:** We choose implementation paths that demonstrate system design skills (concurrency, caching, protocol design) over "easy" BaaS solutions.
3.  **Modular Monolith:** We build a single binary initially but structured with strict interfaces to allow splitting into microservices later (the "Senior Engineer" approach).

---

## Phase 0: The "Ops-First" Foundation
*Goal: A reproducible development environment that mimics production.*

### Stage 0.1: Infrastructure & Tools
- [ ] **Monorepo Setup:** Establish `backend/` (Go), `web/` (React/TS), and `infra/`.
- [ ] **Dockerization:**
    - Write a multi-stage `Dockerfile` for Go (build vs. run).
    - Create `docker-compose.yml` for local dev (Postgres 16, Redis, App, UI).
- [ ] **Make-driven Dev:** Create a `Makefile` for one-command startup (`make up`, `make migrate`, `make test`).
- [ ] **CI Pipeline:** Setup GitHub Actions to run linting (`golangci-lint`) and unit tests on every push.

---

## Phase 1: Identity & Security (The Gatekeeper)
*Goal: Secure authentication demonstrating knowledge of crypto basics.*

### Stage 1.1: Database Schema (Postgres)
- [ ] **Migration System:** Set up `golang-migrate`.
- [ ] **Core Tables:** `users`, `credentials` (sep from users), `sessions`.
- [ ] **Indexes:** Add unique constraints on email/username.

### Stage 1.2: Authentication Service
- [ ] **Email/Password Flow:**
    - Implement `bcrypt` hashing.
    - Validate password strength.
- [ ] **JWT Implementation:**
    - Issue Access Tokens (Short-lived) & Refresh Tokens (Long-lived, stored in HttpOnly cookies).
    - Implement "Token Rotation" for security (detect token reuse).
- [ ] **OAuth 2.0 (Optional/Later):** GitHub/Google login (adds ease of use).

---

## Phase 2: The Real-Time Engine (MVP)
*Goal: Two users exchanging text messages instantly.*

### Stage 2.1: The WebSocket Hub (Go)
- [ ] **Connection Handling:** Upgrade HTTP to WS using `nhooyr` or standard lib.
- [ ] **The Hub Pattern:** Implement a central structure managing active client connections.
- [ ] **Concurrency Safety:** Use Mutexes (`sync.RWMutex`) or Channels to safely add/remove users from the hub.
- [ ] **Graceful Shutdown:** Ensure the server waits for active writes to finish before killing connections on restart.

### Stage 2.2: Persistence & History
- [ ] **Message Schema:** `messages` table with `conversation_id`.
- [ ] **Optimized Queries:** Fetch history using **Cursor Pagination** (not Offset) to handle large chat logs efficiently.
- [ ] **Fan-Out Logic:**
    - Receive WS message -> Save to DB (Async) -> Broadcast to recipient.

### Stage 2.3: Basic Frontend (React)
- [ ] **State Management:** Use a lightweight store (Zustand or Context) for the message list.
- [ ] **Socket Logic:** robust "Reconnection" logic (exponential backoff) if internet drops.
- [ ] **UI:** Simple, clean chat interface (Tailwind CSS).

---

## Phase 3: The "Solid Engineering" Upgrade
*Goal: Moving from a toy app to a scalable system.*

### Stage 3.1: Group Chats & Advanced Routing
- [ ] **Database Model:** `conversations` (many-to-many with users).
- [ ] **Pub/Sub Layer:**
    - Refactor the In-Memory Hub to use an **Interface** (`Publisher`/`Subscriber`).
    - *Why?* Allows swapping memory for Redis later.
- [ ] **Broadcast Logic:** Efficiently route a message to 50+ group members without blocking the main thread.

### Stage 3.2: Media Handling (S3 Compatible)
- [ ] **Presigned URLs:** Do not stream files through your Go server. Generate "Upload URLs" so the frontend uploads directly to Object Storage (R2/S3/MinIO).
- [ ] **Go Worker Pool:** Implement a background worker to process image metadata or generate thumbnails *after* upload.

### Stage 3.3: User Presence
- [ ] **Online/Offline Status:**
    - Use Redis (with TTL) to store "last heartbeat".
    - Show "Typing..." indicators (ephemeral events, do not store in DB).

---

## Phase 4: Reliability & Polish
*Goal: Features that make the app feel professional.*

### Stage 4.1: Message Reliability
- [ ] **Delivery Status:** Implement "Sent", "Delivered", "Read" status updates.
- [ ] **Offline Queue (Frontend):** If user is offline, queue messages in LocalStorage and sync when back online.

### Stage 4.2: Performance Tuning
- [ ] **Caching:** Cache User Profiles and Conversation Metadata in Redis.
- [ ] **Query Optimization:** Analyze SQL `EXPLAIN` plans for the message history query.

### Stage 4.3: Notifications
- [ ] **Event Bus:** Trigger notifications (Email/Push) on new messages if the user is not connected via WS.

---

## Phase 5: The "Expert" Challenges (Optional/Future)
*Goal: Specific features to target Senior/Specialist roles.*

- [ ] **E2EE (Security):** Implement Signal Protocol (Double Ratchet) for client-side encryption.
- [ ] **Voice/Video (Systems):** Integrate `Pion` (Go WebRTC) for P2P calling.
- [ ] **Full Text Search (Data):** Integrate a lightweight search engine (Bleve or Postgres FTS) to search chat history.

---

## Technical Constraints & Standards
- **Language:** Go (Backend), TypeScript (Frontend).
- **Style:** strict `gofmt`, strict implementation of Interfaces.
- **Budget:** Designed for Free Tier (Oracle Cloud / Render / Supabase).
- **Testing:** Unit tests for Auth/Hub logic required before moving to next stage.