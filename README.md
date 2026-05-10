# Mini Exchange - Realtime Trading System

Golang backend untuk mini realtime trading system yang mendukung REST API, WebSocket realtime updates, matching engine, dan price simulation.

## 📋 Daftar Isi

- [Cara Menjalankan](#cara-menjalankan)
- [Design Arsitektur](#design-arsitektur)
- [Flow System](#flow-system)
- [API Documentation](#api-documentation)
- [WebSocket Documentation](#websocket-documentation)
- [Concurrency & Race Condition](#concurrency--race-condition)
- [Broadcast Non-Blocking Strategy](#broadcast-non-blocking-strategy)
- [Tiga Bottleneck Utama](#tiga-bottleneck-utama)
- [Asumsi yang Digunakan](#asumsi-yang-digunakan)
- [Price Simulation](#price-simulation)

---

## Cara Menjalankan

### Prerequisites

- Go 1.21 atau lebih baru
- Git
- PostgreSQL (jika ingin pakai persistent storage)

### Langkah (Mode In-Memory — tanpa database)

```bash
# Clone repository
git clone https://github.com/ilhaamms/mini-exchangee.git
cd mini-exchangee

# Download dependencies
go mod tidy

# Jalankan server (default: semua storage in-memory)
go run cmd/server/main.go
```

### Langkah (Mode PostgreSQL — data persisten)

> **Wajib buat database terlebih dahulu sebelum menjalankan server.**

**1. Buat database di PostgreSQL:**
```sql
CREATE DATABASE mini_exchange;
```

Atau lewat terminal:
```bash
psql -U postgres -c "CREATE DATABASE mini_exchange;"
```

**2. Salin dan isi file konfigurasi:**
```bash
cp .env.example .env
```

Edit `.env` dan sesuaikan:
```env
POSTGRES_ENABLED=true
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your-password-here
POSTGRES_DB=mini_exchange
POSTGRES_SSLMODE=disable
```

**3. Jalankan server — tabel dibuat otomatis (GORM AutoMigrate):**
```bash
go run cmd/server/main.go
```

Server akan mencetak:
```
POSTGRES: connected successfully
POSTGRES: auto-migration completed successfully
```

Tabel `orders`, `trades`, dan `users` akan dibuat otomatis di database.

---

**Build binary:**
```bash
go build -o mini-exchange cmd/server/main.go
./mini-exchange
```

Server berjalan di `http://localhost:8080` (default). Gunakan env `PORT` untuk ubah port:

```bash
PORT=9090 go run cmd/server/main.go
```

### Menjalankan Unit Test

```bash
go test ./... -v
```

---

## Design Arsitektur

Menggunakan **Clean Architecture** dengan separation of concerns yang jelas:

```
d:\trading\
├── cmd/server/main.go           # Entry point & dependency injection
├── internal/
│   ├── config/
│   │   └── config.go            # Environment-based configuration
│   ├── domain/                  # Enterprise business entities (paling dalam)
│   │   ├── order.go             # Order entity
│   │   ├── trade.go             # Trade entity
│   │   ├── market.go            # MarketData, Ticker, OrderBook entities
│   │   ├── event.go             # Event types untuk broadcast
│   │   └── user.go              #  User entity + JWT claims
│   ├── repository/              # Data access layer (in-memory)
│   │   ├── order_repository.go
│   │   ├── trade_repository.go
│   │   ├── market_repository.go
│   │   └── user_repository.go   #  In-memory user store
│   ├── engine/                  # Core business logic
│   │   ├── matching_engine.go   # Price-time priority matching
│   │   └── matching_engine_test.go
│   ├── delivery/
│   │   ├── http/                # REST API layer
│   │   │   ├── router.go        # Routes + middleware chain
│   │   │   ├── order_handler.go
│   │   │   ├── trade_handler.go
│   │   │   ├── market_handler.go
│   │   │   ├── auth_handler.go  #  Login/Register endpoints
│   │   │   └── middleware/
│   │   │       ├── auth.go      #  JWT middleware
│   │   │       └── rate_limiter.go #  Token bucket rate limiter
│   │   └── websocket/           # WebSocket layer
│   │       ├── hub.go           # Central event hub
│   │       ├── client.go        # Per-client connection manager
│   │       └── handler.go       # HTTP upgrade handler (JWT support)
│   ├── infrastructure/          #  External service integrations
│   │   ├── redis/
│   │   │   ├── redis.go         # Redis client wrapper
│   │   │   ├── pubsub.go        # Cross-node event broadcasting
│   │   │   └── cache.go         # Ticker/orderbook cache
│   │   ├── postgres/
│   │   │   ├── postgres.go      # DB connection + auto-migration
│   │   │   ├── order_repository.go
│   │   │   └── trade_repository.go
│   │   └── nats/
│   │       └── nats.go          # NATS pub/sub broker
│   └── simulator/               # Price simulation
│       ├── price_simulator.go   # Random walk simulator
│       └── binance_feed.go      #  Binance real market data
├── migrations/
│   └── 001_create_tables.sql    # PostgreSQL schema
└── pkg/response/                # Shared utilities
    └── response.go
```

### Layer Dependency Flow

```
Delivery (HTTP/WS) → UseCase/Engine → Repository → Domain
```

- **Domain** tidak depend ke layer lain manapun
- **Repository** hanya depend ke Domain
- **Engine** depend ke Domain & Repository
- **Delivery** depend ke Engine & Repository
- **main.go** melakukan dependency injection

---

## Flow System

### 1. Order Creation & Matching Flow

```
Client POST /api/orders
  │
  ├─→ OrderHandler (validate input)
  │     │
  │     ├─→ OrderRepository.Save() (persist order)
  │     │
  │     └─→ MatchingEngine.ProcessOrder() ← synchronous, dalam request handler
  │           │
  │           ├─→ Lock per-stock mutex
  │           ├─→ Get opposite side open orders (FIFO sorted)
  │           ├─→ For each compatible order:
  │           │     ├─→ Fill both orders (atomic update)
  │           │     ├─→ Update order status (UpdateStatus)
  │           │     ├─→ Create Trade record
  │           │     ├─→ Update MarketData (price, volume)
  │           │     └─→ Emit events → WebSocket Hub
  │           └─→ Unlock mutex
  │
  └─→ Return 201 Created (order + status sudah final setelah matching)
```

> **Catatan desain**: Matching dijalankan **synchronous** di dalam request handler.
> Per-stock mutex memastikan BBCA tidak blocking BBRI, sehingga tetap paralel antar stock.
> Pendekatan ini menjamin konsistensi — order book dan status order selalu up-to-date
> saat response 201 dikembalikan ke client.

### 2. WebSocket Event Flow

```
WebSocket Client connects to ws://localhost:8080/ws
  │
  ├─→ Send: {"action":"subscribe","channel":"market.ticker","stock_code":"BBCA"}
  │
  │   ┌───────────────────────────────────┐
  │   │  Event Sources:                   │
  │   │  • Matching Engine (trade events) │
  │   │  • Price Simulator (tick events)  │
  │   └───────────────┬───────────────────┘
  │                   │
  │                   ▼
  │           Hub.BroadcastEvent()
  │                   │
  │                   ▼
  │         Hub event loop (goroutine)
  │                   │
  │                   ├─→ Check each client subscription
  │                   └─→ Non-blocking send to client.send channel
  │                         │
  │                         ▼
  │                 Client.WritePump() (goroutine per client)
  │                         │
  └─── ← ──────────────────┘  (JSON message via WebSocket)
```

### 3. Price Simulation Flow

```
PriceSimulator.Start()
  │
  ├─→ goroutine per stock (BBCA, BBRI, TLKM, ASII, BMRI)
  │     │
  │     └─→ Loop:
  │           ├─→ Wait random interval (500ms - 3000ms)
  │           ├─→ Generate random price change (random walk model)
  │           ├─→ Update MarketData
  │           └─→ Emit market.ticker event → Hub
  │
  └─→ PriceSimulator.Stop() → close(done) → all goroutines exit
```

---

## API Documentation

Base URL: `http://localhost:8080`

### 1. Create Order

```
POST /api/orders
Content-Type: application/json
```

**Request Body:**
```json
{
  "stock_code": "BBCA",
  "side": "BUY",
  "price": 9500.00,
  "quantity": 100
}
```

**Response (201 Created):**
```json
{
  "success": true,
  "message": "order created successfully",
  "data": {
    "id": "ORD0000000001",
    "stock_code": "BBCA",
    "side": "BUY",
    "price": 9500,
    "quantity": 100,
    "filled_qty": 0,
    "remaining_qty": 100,
    "status": "OPEN",
    "created_at": "2026-05-08T10:00:00Z",
    "updated_at": "2026-05-08T10:00:00Z"
  }
}
```

**Validasi:**
- `stock_code`: wajib, string
- `side`: wajib, `"BUY"` atau `"SELL"`
- `price`: wajib, > 0
- `quantity`: wajib, > 0

### 2. Get Order List

```
GET /api/orders?stock=BBCA&status=OPEN
```

**Query Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `stock`   | No       | Filter by stock code |
| `status`  | No       | Filter by status: `OPEN`, `FILLED`, `PARTIAL`, `CANCELLED` |

**Response (200 OK):**
```json
{
  "success": true,
  "message": "orders retrieved successfully",
  "data": {
    "orders": [...],
    "count": 5
  }
}
```

### 3. Get Trade History

```
GET /api/trades?stock=BBCA
```

**Query Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `stock`   | No       | Filter by stock code |

**Response (200 OK):**
```json
{
  "success": true,
  "message": "trade history retrieved successfully",
  "data": {
    "trades": [
      {
        "id": "T0000000001",
        "stock_code": "BBCA",
        "buy_order_id": "ORD0000000001",
        "sell_order_id": "ORD0000000002",
        "price": 9500,
        "quantity": 100,
        "created_at": "2026-05-08T10:00:01Z"
      }
    ],
    "count": 1
  }
}
```

### 4. Get Ticker/Price Snapshot

```
GET /api/market/ticker?stock=BBCA
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "ticker retrieved successfully",
  "data": {
    "stock_code": "BBCA",
    "last_price": 9525.50,
    "prev_price": 9500.00,
    "change": 25.50,
    "change_pct": 0.268,
    "high": 9550.00,
    "low": 9480.00,
    "volume": 15000,
    "updated_at": "2026-05-08T10:00:00Z"
  }
}
```

Tanpa parameter `stock`, mengembalikan semua ticker.

### 5. Get Order Book

```
GET /api/market/orderbook?stock=BBCA
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "order book retrieved successfully",
  "data": {
    "stock_code": "BBCA",
    "bids": [
      {"price": 9500, "quantity": 500, "count": 3},
      {"price": 9490, "quantity": 200, "count": 1}
    ],
    "asks": [
      {"price": 9510, "quantity": 300, "count": 2},
      {"price": 9520, "quantity": 100, "count": 1}
    ],
    "updated_at": "2026-05-08T10:00:00Z"
  }
}
```

### 6. Get Recent Trades

```
GET /api/market/trades?stock=BBCA&limit=10
```

**Query Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `stock`   | Yes      | Stock code |
| `limit`   | No       | Max trades to return (default: 20) |

### 7. Health Check

```
GET /health
```

**Response:** `{"status":"ok","service":"mini-exchange"}`

---

## WebSocket Documentation

### Cara Connect

```
ws://localhost:8080/ws
```

Setelah connect, server mengirim welcome message:
```json
{
  "type": "connected",
  "message": "Welcome to Mini Exchange WebSocket. Send {\"action\":\"subscribe\",\"channel\":\"market.ticker\",\"stock_code\":\"BBCA\"} to start receiving updates."
}
```

### Cara Subscribe

**Subscribe ke channel:**
```json
{
  "action": "subscribe",
  "channel": "market.ticker",
  "stock_code": "BBCA"
}
```

**Unsubscribe dari channel:**
```json
{
  "action": "unsubscribe",
  "channel": "market.ticker",
  "stock_code": "BBCA"
}
```

### Available Channels

| Channel | Description | Data |
|---------|-------------|------|
| `market.ticker` | Realtime price updates per stock | Ticker object (last_price, change, volume, dll) |
| `market.trade` | Stream trades per stock | Trade object (price, quantity, buy/sell order IDs) |
| `market.orderbook` | Order book depth updates (bonus) | OrderBook object (bids, asks arrays) |
| `order.update` | Status update per order (bonus) | Order object (status, filled_qty, remaining_qty) |

### Format Message

**Incoming event (server → client):**
```json
{
  "type": "market.ticker",
  "channel": "market.ticker",
  "stock_code": "BBCA",
  "data": {
    "stock_code": "BBCA",
    "last_price": 9525.50,
    "prev_price": 9500.00,
    "change": 25.50,
    "change_pct": 0.268,
    "high": 9550.00,
    "low": 9480.00,
    "volume": 15000,
    "updated_at": "2026-05-08T10:00:00Z"
  }
}
```

**Trade event:**
```json
{
  "type": "market.trade",
  "channel": "market.trade",
  "stock_code": "BBCA",
  "data": {
    "id": "T0000000001",
    "stock_code": "BBCA",
    "buy_order_id": "ORD0000000001",
    "sell_order_id": "ORD0000000002",
    "price": 9500,
    "quantity": 100,
    "created_at": "2026-05-08T10:00:01Z"
  }
}
```

### Rekomendasi Tools untuk Testing WebSocket

#### 1. **wscat** (CLI)
```bash
# Install
npm install -g wscat

# Connect
wscat -c ws://localhost:8080/ws

# Kirim subscribe
{"action":"subscribe","channel":"market.ticker","stock_code":"BBCA"}
```

#### 2. **Postman**
1. Buka Postman → New → WebSocket Request
2. URL: `ws://localhost:8080/ws`
3. Click "Connect"
4. Di message field, kirim:
   ```json
   {"action":"subscribe","channel":"market.ticker","stock_code":"BBCA"}
   ```

#### 3. **websocat** (CLI, Rust-based)
```bash
websocat ws://localhost:8080/ws
# Lalu ketik subscribe message
```

#### 4. **Browser Console**
```javascript
const ws = new WebSocket("ws://localhost:8080/ws");
ws.onmessage = (e) => console.log(JSON.parse(e.data));
ws.onopen = () => ws.send(JSON.stringify({
  action: "subscribe",
  channel: "market.ticker",
  stock_code: "BBCA"
}));
```

---

## Concurrency & Race Condition

### Potensi Race Condition dan Penanganannya

| Skenario | Masalah | Solusi |
|----------|---------|--------|
| 2 buy order masuk bersamaan untuk stock yang sama | Bisa match sell order yang sama 2x (double fill) | **Per-stock mutex** di matching engine. Matching dijalankan synchronous — hanya 1 request memproses matching per stock dalam satu waktu |
| Order field di-update concurrent (Fill) | Data inconsistency pada `filled_qty`, `remaining_qty`, `status` | **sync.RWMutex** di struct Order untuk atomic field update |
| Repository read/write concurrent | Concurrent map read/write panic | **sync.RWMutex** di setiap repository |
| WebSocket client map concurrent access | Panic saat iterating & modifying map | **sync.RWMutex** di Hub untuk clients map |
| Client subscribe/unsubscribe while broadcast | Race pada subscriptions map | **sync.RWMutex** di Client untuk subscriptions |
| Order book tidak konsisten saat langsung dicek setelah submit | Matching belum selesai saat response dikirim (async race) | Matching **synchronous** — response 201 hanya dikirim setelah matching dan `UpdateStatus` selesai |

### Strategi Concurrency

```
┌──────────────────────────────────────────────────────┐
│                  Stock-Level Locking                  │
│                                                      │
│  BBCA Mutex ──→ Orders BBCA diprocess serial         │
│  BBRI Mutex ──→ Orders BBRI diprocess serial         │  ← Paralel antar stock
│  TLKM Mutex ──→ Orders TLKM diprocess serial         │
│                                                      │
│  Keuntungan: Order BBCA tidak blocking order BBRI    │
└──────────────────────────────────────────────────────┘
```

Desain ini memungkinkan **paralelisme antar stock** sekaligus **konsistensi per stock**, sesuai asumsi 1000 orders/menit.

---

## Broadcast Non-Blocking Strategy

### Masalah

Jika 1 dari 500 client lambat (slow consumer), broadcast ke semua client akan terhenti.

### Solusi: Buffered Channel + Select-Default Pattern

```go
// Di Hub.broadcastEvent():
for client := range h.clients {
    if client.IsSubscribed(event.Channel, event.StockCode) {
        select {
        case client.send <- message:
            // berhasil kirim
        default:
            // client buffer penuh → skip (non-blocking)
            log.Printf("client %s too slow, dropping message", client.id)
        }
    }
}
```

**Mekanisme:**

1. **Buffered send channel (256 messages per client)**: Memberikan ruang buffer sehingga burst traffic bisa ditangani
2. **Select-default pattern**: Jika buffer penuh, message di-drop untuk client tersebut tanpa blocking client lain
3. **Goroutine per client (WritePump)**: Setiap client punya dedicated goroutine untuk write ke connection, sehingga write ke 1 client tidak memblok write ke client lain
4. **Hub broadcast channel (1024 buffer)**: Producer (matching engine, simulator) tidak blocking meskipun hub sedang sibuk broadcast

### Pencegahan Goroutine Leak

1. **ReadPump** exit → trigger `hub.unregister`
2. **Hub** removes client → close `client.send` channel
3. **WritePump** detects closed channel → exit goroutine
4. **PriceSimulator** → `close(done)` + `WaitGroup.Wait()` untuk shutdown bersih

---

## Tiga Bottleneck Utama

### 1. Per-Stock Matching Lock Contention

**Problem:** Semua order untuk stock yang sama diproses serial karena per-stock mutex. Jika 80% order untuk BBCA, maka 80% order mengantre di satu mutex.

**Solusi:**
- Saat ini sudah per-stock (bukan global lock), jadi order BBCA tidak memblok BBRI
- Untuk scaling lebih lanjut: **sharding** berdasarkan price level, atau gunakan **lock-free order book** (concurrent skip list)
- Gunakan **order queue per stock** (channel-based) untuk memastikan ordering tanpa contention

### 2. In-Memory Storage Limit

**Problem:** Semua data disimpan di memory. Seiring waktu, trades dan orders akan terus bertambah dan menghabiskan RAM.

**Solusi:**
- **Redis** untuk caching hot data (ticker, recent trades, order book)
- **Database** (PostgreSQL) untuk persistent storage orders & trades
- **Archival strategy**: Pindahkan data lama (>24h) ke database, simpan data aktif di memory
- **Ringbuffer** untuk recent trades (simpan hanya N trades terakhir di memory)

### 3. WebSocket Broadcast Fan-Out

**Problem:** Setiap event harus dikirim ke potentially 500 clients. Hub menjadi single point of contention untuk broadcast.

**Solusi:**
- Saat ini Hub menggunakan **single goroutine** untuk serialized broadcast → konsisten tapi bisa lambat
- **Horizontal scaling**: Gunakan **Redis Pub/Sub** atau **NATS** sebagai message bus antar instance
- **Topic-based sharding**: Pisahkan hub per stock code, sehingga broadcast bisa paralel antar stock
- **Worker pool** untuk broadcast: Alih-alih 1 goroutine broadcast ke 500 client, gunakan 10 worker masing-masing broadcast ke 50 client

### Strategi Scaling Horizontal

```
                    ┌─────────────┐
                    │ Load Balancer│
                    └──────┬──────┘
               ┌───────────┼───────────┐
               ▼           ▼           ▼
          ┌─────────┐ ┌─────────┐ ┌─────────┐
          │ Node 1  │ │ Node 2  │ │ Node 3  │
          │(WS+API) │ │(WS+API) │ │(WS+API) │
          └────┬────┘ └────┬────┘ └────┬────┘
               │           │           │
               └─────┬─────┘───────────┘
                     ▼
              ┌─────────────┐
              │ Redis/NATS  │ ← Shared event bus
              │  Pub/Sub    │
              └──────┬──────┘
                     ▼
              ┌─────────────┐
              │ PostgreSQL  │ ← Persistent storage
              └─────────────┘
```

1. **WebSocket**: Sticky sessions via Load Balancer (IP hash / cookie)
2. **Matching Engine**: Centralized per stock (atau sharded per stock range)
3. **Events**: Broadcast melalui Redis Pub/Sub sehingga event dari Node 1 juga diterima client di Node 2
4. **State**: Shared state di Redis + PostgreSQL

---

## Asumsi yang Digunakan

Sesuai dengan spesifikasi test:

| Asumsi | Nilai | Bagaimana Ditangani |
|--------|-------|---------------------|
| Orders per menit | 1000 | Per-stock mutex memungkinkan parallelism. Async matching via goroutine |
| Active WS clients | 500 | Buffered channel per client (256). Non-blocking broadcast |
| Subscriptions per client | 1-5 stocks | Subscription map per client. Filtered broadcast |
| Slow client handling | Tidak boleh blocking | Select-default pattern. Message dropping untuk slow clients |

**Asumsi tambahan:**
- Harga dalam **float64** (untuk simplicity, real system pakai integer basis point)
- Order ID auto-generated menggunakan **atomic counter** (thread-safe)
- Authentication via JWT (opt-in), diaktifkan secara default di routing
- Storage **in-memory** by default (hilang saat restart); aktifkan `POSTGRES_ENABLED=true` untuk data persisten
- 5 stock simulasi: BBCA, BBRI, TLKM, ASII, BMRI

---

## Price Simulation

### Cara Kerja

1. **5 stock** diinisialisasi dengan harga awal yang berbeda
2. Setiap stock memiliki **goroutine sendiri** yang menghasilkan price tick
3. Harga berubah menggunakan **random walk model**:
   ```
   new_price = old_price × (1 + random(-volatility, +volatility))
   ```
4. **Interval** antar tick **random** antara 500ms–3000ms (simulasi natural market)
5. Setiap tick meng-update MarketData dan emit event `market.ticker` ke WebSocket Hub
6. **Volatility** per stock berbeda (0.3%–0.5%) untuk variasi yang realistis

### Konfigurasi Stock

| Stock | Harga Awal | Volatility |
|-------|-----------|------------|
| BBCA  | 9500      | 0.3%       |
| BBRI  | 5200      | 0.4%       |
| TLKM  | 3800      | 0.5%       |
| ASII  | 6100      | 0.3%       |
| BMRI  | 6800      | 0.4%       |

---

## Testing dengan cURL

### Create Orders (buat trade terjadi)

```bash
# Create SELL order
curl -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{"stock_code":"BBCA","side":"SELL","price":9500,"quantity":100}'

# Create BUY order (akan match dengan SELL di atas)
curl -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{"stock_code":"BBCA","side":"BUY","price":9500,"quantity":100}'

# Get all orders
curl http://localhost:8080/api/orders

# Get orders filtered
curl "http://localhost:8080/api/orders?stock=BBCA&status=FILLED"

# Get trade history
curl http://localhost:8080/api/trades

# Get ticker
curl http://localhost:8080/api/market/ticker?stock=BBCA

# Get all tickers
curl http://localhost:8080/api/market/ticker

# Get order book
curl http://localhost:8080/api/market/orderbook?stock=BBCA

# Get recent trades
curl "http://localhost:8080/api/market/trades?stock=BBCA&limit=5"

# Health check
curl http://localhost:8080/health
```

---

## Bonus Features (Nice to Have)

Semua fitur bonus di bawah ini bersifat **opt-in** via environment variable. Server tetap berjalan normal tanpa konfigurasi apapun.

### 1. JWT Authentication

Endpoints `/api/orders` dan `/api/trades` dilindungi JWT. Market data tetap public.

```bash
# Register user baru
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"username":"trader1","email":"trader1@example.com","password":"secret123"}'

# Login
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"trader1","password":"secret123"}'

# Gunakan token dari response untuk akses protected endpoint
curl http://localhost:8080/api/orders \
  -H "Authorization: Bearer <token>"

# WebSocket dengan auth (via query param)
wscat -c "ws://localhost:8080/ws?token=<token>"
```

Konfigurasi: `JWT_SECRET=my-secret-key`

### 2. Rate Limiting (Token Bucket)

Setiap IP dibatasi 100 request/menit dengan burst capacity 100. Jika limit terlampaui, server mengembalikan `429 Too Many Requests`.

Implementasi: per-IP token bucket dengan automatic refill dan stale bucket cleanup.

### 3. Redis (Pub/Sub + Cache)

```bash
REDIS_ENABLED=true REDIS_ADDR=localhost:6379 go run cmd/server/main.go
```

- **Pub/Sub**: Event dari satu server instance di-broadcast ke semua instance lain
- **Cache**: Ticker dan orderbook di-cache dengan TTL pendek (2-5 detik)

### 4. PostgreSQL Persistence

> **Pastikan database `mini_exchange` sudah dibuat terlebih dahulu** (lihat [Langkah Mode PostgreSQL](#langkah-mode-postgresql--data-persisten)).

```bash
POSTGRES_ENABLED=true \
POSTGRES_HOST=localhost \
POSTGRES_PORT=5432 \
POSTGRES_USER=postgres \
POSTGRES_PASSWORD=postgres \
POSTGRES_DB=mini_exchange \
go run cmd/server/main.go
```

Atau cukup set di `.env` lalu jalankan biasa:
```bash
go run cmd/server/main.go
```

GORM **AutoMigrate** membuat/memperbarui tabel `orders`, `trades`, dan `users` secara otomatis saat server start. Tidak perlu jalankan SQL migration manual.

### 5. NATS Message Broker

```bash
NATS_ENABLED=true NATS_URL=nats://localhost:4222 go run cmd/server/main.go
```

Matching engine → NATS publish → Worker subscribe → WebSocket hub. Mendukung auto-reconnect.

### 6. Binance Real Market Data

```bash
BINANCE_ENABLED=true go run cmd/server/main.go
```

Mengganti price simulator dengan data real dari Binance WebSocket (crypto: BTCUSDT, ETHUSDT, dll). Auto-fallback ke simulator jika koneksi gagal.

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `JWT_SECRET` | `ybtech-mini-exchange-...` | JWT signing key |
| `REDIS_ENABLED` | `false` | Enable Redis Pub/Sub + Cache |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | `` | Redis password |
| `POSTGRES_ENABLED` | `false` | Enable PostgreSQL persistence |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_USER` | `postgres` | PostgreSQL username |
| `POSTGRES_PASSWORD` | `postgres` | PostgreSQL password |
| `POSTGRES_DB` | `mini_exchange` | Nama database |
| `POSTGRES_SSLMODE` | `disable` | SSL mode (`disable`/`require`/`verify-full`) |
| `NATS_ENABLED` | `false` | Enable NATS message broker |
| `NATS_URL` | `nats://localhost:4222` | NATS server URL |
| `BINANCE_ENABLED` | `false` | Enable Binance real market data |

---

## Tech Stack

- **Language**: Go 1.21+
- **WebSocket**: [gorilla/websocket](https://github.com/gorilla/websocket)
- **HTTP**: Standard `net/http` library
- **Auth**: [golang-jwt/jwt](https://github.com/golang-jwt/jwt) (JWT v5)
- **Storage**: In-memory (default) + PostgreSQL via [GORM](https://gorm.io) (opt-in)
- **ORM**: [GORM v2](https://gorm.io) + `gorm.io/driver/postgres` — AutoMigrate, upsert, query builder
- **Cache/PubSub**: [go-redis](https://github.com/redis/go-redis) (opt-in)
- **Message Broker**: [nats.go](https://github.com/nats-io/nats.go) (opt-in)
- **Architecture**: Clean Architecture
- **Testing**: Standard `testing` package
