<div align="center">

# ⟁ Eidolon

### Local Code Intelligence. No cloud. No API. No limits.

**CodeLlama-7B · Apple MLX · Apache Kafka · Go · React · VSCode**

---

*A fully local, privacy-first code completion engine that runs entirely on your machine.*  
*Every keystroke stays on your device. Every token is yours.*

</div>

---

## What is Eidolon?

Eidolon is a production-grade, event-driven code intelligence system built from scratch. It fine-tunes a 7-billion-parameter language model on your private repositories, streams completions directly into your editor with sub-100ms first-token latency, and uses Apache Kafka to decouple every stage of the pipeline.

Zero bytes leave your machine. No subscriptions. No telemetry. No cloud dependency of any kind.

> This is not a wrapper around an API. This is an ML system, an event streaming platform, a Go backend, a React dashboard, and a VSCode extension — all built from scratch, all running locally on Apple M4.

---

## The Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                         YOUR EDITOR                              │
│               VSCode / Cursor Extension (TypeScript)             │
│          Inline ghost text  ·  Tab to accept  ·  Status bar      │
└──────────────────────────────┬───────────────────────────────────┘
                               │  keystroke event
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                        APACHE KAFKA                              │
│                      Single-broker KRaft                         │
│                                                                  │
│   eidolon.keystrokes   ──▶   eidolon.context.requests            │
│   eidolon.infer.requests ──▶ eidolon.completions                 │
│   eidolon.feedback  ·  eidolon.embeddings  ·  eidolon.metrics    │
└──────────┬───────────────────────────────────────┬───────────────┘
           │                                       │
           ▼                                       ▼
┌──────────────────────┐               ┌───────────────────────────┐
│   Context Builder    │               │    Inference Gateway      │
│     (Go service)     │               │      (Go service)         │
│                      │               │                           │
│  · Tree-sitter AST   │               │  · Rate limiting          │
│  · FIM context build │               │  · Token budget check     │
│  · 300ms debounce    │               │  · Unix socket IPC        │
│  · Publish to Kafka  │               │  · Stream tokens to Kafka │
└──────────────────────┘               └──────────────┬────────────┘
                                                      │
                                                      ▼
                                       ┌──────────────────────────┐
                                       │   MLX Inference Server   │
                                       │  (Python · Unix socket)  │
                                       │                          │
                                       │  · CodeLlama-7B 4-bit    │
                                       │  · Apple Neural Engine   │
                                       │  · LoRA adapter support  │
                                       │  · ~40 tok/s on M4       │
                                       └──────────────┬───────────┘
                                                      │
                              ┌───────────────────────┘
                              ▼
┌──────────────────────────────────────────────────────────────────┐
│                       REACT DASHBOARD                            │
│               Vite  ·  TailwindCSS  ·  Recharts  ·  SSE         │
│                                                                  │
│   Live completion feed  ·  Model config  ·  Kafka inspector      │
│   Training metrics  ·  Repo index status  ·  Real-time charts    │
└──────────────────────────────────────────────────────────────────┘
```

---

## How It Works — End to End

**1. You type in your editor.**  
The Cursor/VSCode extension captures keystrokes with a 300ms debounce and sends a `KeystrokeEvent` to the Go backend via HTTP.

**2. The Go server publishes to Kafka.**  
The event lands on `eidolon.keystrokes`. This decouples the editor from everything downstream — if the model is busy, events queue up and replay automatically.

**3. The Context Builder consumes the event.**  
A Go goroutine parses the file using Tree-sitter, extracts the enclosing function, imports, and type signatures, then builds a Fill-in-the-Middle (FIM) prompt. It publishes an enriched `ContextPayload` to `eidolon.context.requests`.

**4. The Inference Gateway routes to the model.**  
Another Go goroutine consumes the context payload, validates the token budget, applies rate limiting, and calls the Python MLX inference server over a Unix domain socket.

**5. CodeLlama generates tokens.**  
The MLX inference server runs CodeLlama-7B at 4-bit quantisation on Apple's Neural Engine. Tokens stream back as newline-delimited JSON. Each token is published to `eidolon.completions` on Kafka.

**6. The completion appears in your editor.**  
The extension polls the SSE stream, assembles the token stream, and renders the completion as inline ghost text. Tab to accept. Escape to reject.

**7. Your feedback trains the next version.**  
Accept/reject signals flow into `eidolon.feedback`. When 5,000 new signals accumulate, the system automatically triggers a LoRA fine-tuning run on your codebase and hot-swaps the new adapter — no restart required.

---

## Stack

| Layer | Technology |
|---|---|
| Language model | CodeLlama-7B-Instruct (4-bit, MLX) |
| ML framework | Apple MLX — runs on M4 Neural Engine |
| Fine-tuning | LoRA via mlx-lm (rank=16, ~50MB adapters) |
| Event streaming | Apache Kafka 3.7 (KRaft, no ZooKeeper) |
| Backend | Go 1.26 — context builder, inference gateway, REST API |
| IPC | Unix domain socket (Go to Python) |
| Frontend | React 18 · Vite · TailwindCSS · Recharts |
| Editor | VSCode/Cursor extension (TypeScript) |
| Storage | SQLite (events) · pgvector (embeddings) |
| Context parsing | Tree-sitter (Go, Python, TypeScript) |

---

## Kafka Topic Design

```
eidolon.keystrokes        Raw editor events. High volume, 7-day retention.
eidolon.context.requests  AST-enriched FIM payloads. 1-day retention.
eidolon.infer.requests    Validated inference requests. 1-day retention.
eidolon.completions       Streaming token output. 7-day retention.
eidolon.feedback          Accept/reject signals. 30-day retention.
eidolon.embeddings        Code chunk vectors for pgvector. 7-day retention.
eidolon.metrics           Latency and throughput telemetry. 3-day retention.
```

---

## Performance

| Metric | Value |
|---|---|
| First token latency | < 100ms (P95) |
| Throughput | ~40 tokens/sec on M4 |
| Model memory | 4.3 GB (4-bit quantised) |
| Fine-tuning time | 2–4 hours on M4 (100k examples) |
| Adapter size | ~50 MB |

---

## Getting Started

### Prerequisites

- Apple M4 Mac (M1/M2/M3 also work, slower inference)
- macOS 14.0+
- 16 GB RAM minimum (24 GB recommended)
- Docker + Colima
- Go 1.22+, Python 3.11+, Node 20+

### 1. Clone and install dependencies

```bash
git clone https://github.com/Shylin26/Eidolon.git
cd Eidolon
go mod tidy
pip3 install mlx mlx-lm transformers fastapi uvicorn confluent-kafka
cd web && npm install
cd ../extension/eidolon && npm install
```

### 2. Start Kafka

```bash
colima start --cpu 4 --memory 8
docker compose -f docker/kafka-kraft.yml up -d
```

### 3. Download the model

```bash
hf download mlx-community/CodeLlama-7b-Instruct-hf-4bit-mlx \
  --local-dir ~/eidolon/models/eidolon-base

# Verify M4 GPU
python3 -c "import mlx.core as mx; print(mx.default_device())"
# Expected: Device(gpu, 0)
```

### 4. Start everything

```bash
# Terminal 1 — ML inference server
python3 ml/inference_server.py

# Terminal 2 — Go backend
go run cmd/server/main.go

# Terminal 3 — React dashboard
cd web && npm run dev
```

### 5. Install the editor extension

```bash
cd extension/eidolon
npx vsce package --no-dependencies --allow-star-activation
# Cursor/VSCode: Cmd+Shift+P → Install from VSIX
```

---

## Project Structure

```
eidolon/
├── cmd/
│   ├── server/          # Main binary — all services wired together
│   ├── indexer/         # Repo indexer CLI
│   ├── trainer/         # LoRA training trigger CLI
│   └── dataset-builder/ # FIM dataset extractor
├── internal/
│   ├── kafka/           # Producer and consumer wrappers
│   ├── context/         # Tree-sitter context builder
│   ├── inference/       # Unix socket client
│   ├── api/
│   │   ├── grpc/        # Inference gateway
│   │   └── rest/        # HTTP + SSE server
│   └── config/          # Viper config
├── ml/                  # Python MLX inference server
├── web/                 # React dashboard
├── extension/           # VSCode/Cursor extension
└── docker/              # Kafka KRaft compose
```

---

## Roadmap

- [ ] Neovim plugin (LSP server mode)
- [ ] Automated nightly LoRA retraining on accumulated feedback
- [ ] Team mode — shared LAN inference server with per-user adapters
- [ ] pgvector RAG — inject semantically similar code chunks into prompt
- [ ] Chat mode — ask questions about your codebase in the sidebar
- [ ] Mixtral-8x7B support for M4 Max / M4 Ultra

---

## Why Eidolon?

Most AI coding tools are thin wrappers around cloud APIs. Your code gets sent to a server. Someone else's model runs on someone else's hardware. You pay per token. You hope your IP stays private.

Eidolon is the opposite. The model runs on your Neural Engine. The weights live on your disk. The events flow through your Kafka broker. The fine-tuning happens on your GPU cores. Nothing leaves. Ever.

This is what it looks like when you build the full stack yourself.

---

<div align="center">

Built from scratch · Apple M4 · No cloud · No compromises

**[View on GitHub](https://github.com/Shylin26/Eidolon)**

</div>
