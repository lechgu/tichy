# Tichy

A self-contained, privacy-focused RAG (Retrieval-Augmented Generation) system in Go.
All data stays local — nothing is sent to external LLM providers.

---

## Table of Contents

- [Requirements](#requirements)
- [Quick Start](#quick-start)
- [Architecture Overview](#architecture-overview)
- [Vector Store Backends](#vector-store-backends)
- [Multi-Collection Support](#multi-collection-support)
- [Generic Interfaces](#generic-interfaces)
- [FOXDEN Authorization](#foxden-authorization)
- [Usage](#usage)
- [Configuration](#configuration)
- [Services](#services)
- [Acknowledgments](#acknowledgments)
- [License](#license)

---

## Requirements

- **Go 1.24.4+**
- **Docker and Docker Compose**
- **NVIDIA GPU with CUDA support** (required for llama.cpp inference with the default `docker-compose.yml`)
  - For CPU-only inference, use `ghcr.io/ggerganov/llama.cpp:server` and remove the `runtime: nvidia` and NVIDIA environment variables from the `llm` and `embeddings` services.
- **GGUF Models**:
  - Main LLM model (e.g., Gemma 3 12B)
  - Embedding model (e.g., nomic-embed-text v1.5)

---

## Quick Start

### 1. Prepare Models

Place your GGUF models in a directory of your choice (e.g., `~/models/llama/`):

```bash
mkdir -p ~/models/llama
# Copy your models to:
# ~/models/llama/google_gemma-3-12b-it-Q8_0.gguf
# ~/models/llama/nomic-embed-text-v1.5.Q8_0.gguf
```

Update the volume paths in `docker-compose.yml` if using a different location.

### 2. Start Services

Start PostgreSQL, LLM server, and embeddings server:

```bash
docker compose up -d
```

Verify services are running:

```bash
docker compose ps
```

### 3. Configure Environment

Copy and configure the environment file:

```bash
cp examples/insurellm/.env .env
# Edit .env to adjust URLs, ports, vector backend, and chunk sizes
```

### 4. Build and Run

Build the application:

```bash
make build
```

Or use Docker to run commands without building locally:

```bash
docker compose run --rm tichy db up
docker compose run --rm tichy ingest --source /mnt/cwd/examples/insurellm/knowledge-base/ --mode text
```

Initialize the database:

```bash
./tichy db up
```

Ingest documents:

```bash
./tichy ingest --source ./examples/insurellm/knowledge-base/ --mode text
```

### 5. Start Chatting

Start an interactive chat session:

```bash
./tichy chat
```

Or with markdown rendering:

```bash
./tichy chat --markdown
```

---

## Architecture Overview

Tichy is built around a small set of clean interfaces that decouple the retrieval
layer from the rest of the system. The high-level data flow is:

```
User query
    |
    v
[ HTTP Server ] --> [ Responder ]
                         |
              +----------+----------+
              |                     |
        [ Retriever ]         [ LLM Client ]
              |                     |
     [ Vector Store ]        [ llama.cpp ]
    (pgvector or Qdrant)
```

1. The HTTP server parses the request, validates the JWT, and hands the query to the `Responder`.
2. The `Responder` calls the `Retriever` to fetch relevant chunks from the vector store.
3. Retrieved chunks are assembled into a RAG context and injected into the system prompt.
4. The enriched prompt is sent to the local LLM server and the response is returned to the caller.

---

## Vector Store Backends

Tichy supports multiple vector store backends, selectable at runtime via the
`VECTORDB_BACKEND` environment variable. The retrieval and ingestion logic is fully
abstracted behind interfaces (see [Generic Interfaces](#generic-interfaces)), so the
rest of the application is unaware of which backend is in use.

### Supported backends

| Backend | `VECTORDB_BACKEND` value |
|---------|--------------------------|
| PostgreSQL + pgvector | `pgvector` |
| Qdrant | `qdrant` |

For a detailed comparison of backends — including pros, cons, and guidance on when
to use each — see [VectorDBs.md](VectorDBs.md).

### Selecting a backend

Set the backend in your `.env` file:

```bash
# PostgreSQL + pgvector
VECTORDB_BACKEND=pgvector
DATABASE_URL=postgres://user:pass@localhost:5432/tichy

# Qdrant
VECTORDB_BACKEND=qdrant
QDRANT_HOST=localhost
QDRANT_PORT=6333
QDRANT_COLLECTION=my_collection
```

No application code changes are required when switching backends.

---

## Multi-Collection Support

Tichy supports querying **multiple Qdrant collections concurrently** in a single
request. This allows different knowledge domains (e.g., per-instrument or per-group
data) to be stored in separate collections and queried together, with all results
merged into a single LLM response.

### How it works

When the `?collection=` query parameter is provided, the server fans out one
goroutine per collection, queries Qdrant in parallel, merges the retrieved chunks,
and passes them all to the LLM as a single enriched context.

```
POST /v1/chat/completions?collection=beamline_a&collection=beamline_b
```

This triggers:

```
AuthzServer
    |-- goroutine --> QdrantRetriever.QueryCollection("beamline_a", ...)
    |-- goroutine --> QdrantRetriever.QueryCollection("beamline_b", ...)
    |
    v  (results merged)
Responder.callLLM(merged chunks)
    |
    v
Single ChatCompletionResponse
```

### Collection parameter formats

Both formats are accepted and can be combined:

```bash
# Comma-separated
POST /v1/chat/completions?collection=beamline_a,beamline_b,beamline_c

# Repeated parameters
POST /v1/chat/completions?collection=beamline_a&collection=beamline_b

# Single collection
POST /v1/chat/completions?collection=beamline_a

# No parameter → falls back to QDRANT_COLLECTION default
POST /v1/chat/completions
```

### Error handling

Partial failures are tolerated: if one collection fails, its error is logged and
the remaining collections' results are still passed to the LLM. The request only
returns HTTP 500 if **every** collection fails.

### curl examples

```bash
# Single collection
curl -X POST "http://tichy:8080/v1/chat/completions?collection=beamline_a" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"What is the beam energy?"}]}'

# Multiple collections (comma-separated)
curl -X POST "http://tichy:8080/v1/chat/completions?collection=beamline_a,beamline_b" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"What is the beam energy?"}]}'
```

For a full description of the data flow, see [multi-collection-rag.md](docs/multi-collection-rag.md).

---

## Generic Interfaces

All major extension points are defined as Go interfaces in
`internal/interfaces/interfaces.go`. This single package is the only place that
defines these contracts; everything else imports from it.

```
internal/interfaces/      ← defines Embedder, Ingestor, Retriever
    |
    +-- internal/pgvectorstore/   implements Ingestor, Retriever (uses Embedder)
    +-- internal/qdrantstore/     implements Ingestor, Retriever (uses Embedder)
    +-- internal/embedders/       satisfies Embedder
    +-- internal/responders/      consumes Retriever
    +-- internal/injectors/       wires everything together
```

### The three core interfaces

```go
// Embedder converts chunks of text into dense embedding vectors.
type Embedder interface {
    Embed(ctx context.Context, chunks []models.Chunk) ([][]float32, error)
}

// Ingestor writes chunks and their embeddings into a vector store.
type Ingestor interface {
    Ingest(ctx context.Context, chunks []models.Chunk, embeddings [][]float32) error
}

// Retriever searches a vector store and returns the topK most relevant chunks.
type Retriever interface {
    Query(ctx context.Context, query string, topK int) ([]models.Chunk, error)
    QueryCollection(ctx context.Context, collection, query string, topK int) ([]models.Chunk, error)
}
```

The `vectorstore` package re-exports `Ingestor` and `Retriever` as type aliases for
backward compatibility, so existing code that references `vectorstore.Retriever`
continues to compile unchanged.

### Adding a new backend

To add a new vector store backend:

1. Create a new package under `internal/` (e.g., `internal/weaviatestore/`).
2. Implement `interfaces.Ingestor` and `interfaces.Retriever` in that package.
3. Register the new backend in `internal/injectors/injectors.go` by adding a case
   to `provideIngestor` and `provideRetriever`.
4. Add the corresponding configuration variables.

No changes to `responders`, `servers`, or any other package are needed.

---

## FOXDEN Authorization

Tichy integrates with the FOXDEN frontend for access control. When the `authz` web
server is configured, every request must carry a signed JWT. The token encodes:

- The user's identity (`user` claim)
- The list of vector DB collections the user is allowed to query (`vector_dbs` claim)

The `AuthMiddleware` validates the token and injects the user information into the
request context. Downstream components (the Qdrant retriever and the responder) read
this context to restrict retrieval to the user's permitted collections.

```bash
# Enable the authz server
WEB_SERVER=authz
WEB_TOKEN_SECRET=your-secret-here
```

Access rules mapping groups to collections are configured in the FOXDEN
configuration module. For example, a user in the `computing` group will only be able
to query the `computing` Qdrant collection, while a user in both `computing` and
`beamline_x` groups can query both.

---

## Usage

### Ingest Documents

```bash
./tichy ingest --source ./path/to/documents/ --mode text
```

### Interactive Chat

```bash
./tichy chat
> When was InsureLLM founded?
```

### Chat with Markdown Rendering

```bash
./tichy chat --markdown
```

### Generate Test Cases

```bash
./tichy tests generate --source ./knowledge-base/ --mode text --num 20 --output tests.json
```

### Evaluate RAG Performance

```bash
./tichy tests evaluate --input tests.json
```

### Database Migrations

```bash
./tichy db up       # apply all pending migrations
./tichy db down     # roll back one migration
./tichy db reset    # roll back all migrations
./tichy db status   # show current migration status
```

---

## Configuration

All configuration is via environment variables, typically placed in a `.env` file.

### Core settings

| Variable | Default | Description |
|---|---|---|
| `PORT` | `80` | HTTP server port |
| `LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `LLM_SERVER_URL` | — | Base URL of the llama.cpp LLM server |
| `EMBEDDING_SERVER_URL` | — | Base URL of the llama.cpp embeddings server |
| `EMBEDDING_DIMENSION` | `768` | Embedding vector dimension |
| `CHUNK_SIZE` | `1000` | Target size of each document chunk (characters) |
| `CHUNK_OVERLAP` | `200` | Overlap between adjacent chunks (characters) |
| `TOP_K` | `5` | Number of chunks to retrieve per query |
| `SYSTEM_PROMPT_TEMPLATE` | built-in | Path to a custom system prompt template file |
| `FILE_EXTENSIONS` | `.txt,.md,.log` | Comma-separated list of file extensions to ingest |

### Vector store selection

| Variable | Default | Description |
|---|---|---|
| `VECTORDB_BACKEND` | — | `pgvector` or `qdrant` |

### PostgreSQL / pgvector settings

| Variable | Description |
|---|---|
| `DATABASE_URL` | PostgreSQL connection string |

### Qdrant settings

| Variable | Description |
|---|---|
| `QDRANT_HOST` | Qdrant server hostname |
| `QDRANT_PORT` | Qdrant server port |
| `QDRANT_COLLECTION` | Default collection name |
| `QDRANT_COLLECTION_SIZE` | Vector dimension for new collections |
| `QDRANT_API_KEY` | API key (if Qdrant authentication is enabled) |
| `QDRANT_USE_TLS` | Enable TLS for Qdrant connection |
| `QDRANT_RECREATE_COLLECTION` | Drop and recreate the collection on ingest |

### Web server settings

| Variable | Default | Description |
|---|---|---|
| `WEB_SERVER` | `default` | `default` (no auth) or `authz` (JWT-based auth) |
| `WEB_TOKEN_SECRET` | — | HMAC secret used to validate JWT tokens (required when `WEB_SERVER=authz`) |

---

## Services

The default `docker-compose.yml` starts the following services:

| Service | Description | Port |
|---|---|---|
| `postgres` | PostgreSQL with the pgvector extension | 5432 |
| `qdrant` | Qdrant vector database | 6333 |
| `llm` | llama.cpp inference server | 8080 |
| `embeddings` | llama.cpp embeddings server | 8081 |

---

## Acknowledgments

The example insurance knowledge base in `examples/insurellm/` is derived from the
dataset provided by the [LLM Engineering course](https://github.com/ed-donner/llm_engineering).

---

## License

BSD 3-Clause — see [LICENSE](LICENSE) for details.
