# Tichy

A self-contained, privacy-focused RAG (Retrieval-Augmented Generation) system in Go. All data stays local - nothing is sent to external LLM providers.

## Requirements

- **Go 1.24.4+**
- **Docker and Docker Compose**
- **NVIDIA GPU with CUDA support** (required for llama.cpp inference with default docker-compose.yml)
  - For CPU-only inference, use `ghcr.io/ggerganov/llama.cpp:server` image and remove the `runtime: nvidia` and NVIDIA environment variables from the llm and embeddings services
- **GGUF Models**:
  - Main LLM model (e.g., Gemma 3 12B)
  - Embedding model (e.g., nomic-embed-text v1.5)

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
# Edit .env if needed to adjust URLs, ports, or chunk sizes
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

## Usage Examples

### Ingest Documents
```bash
./tichy ingest --source ./path/to/documents/ --mode text
```

### Interactive Chat
```bash
./tichy chat
> What is the maximum coverage for earthquake insurance?
```

### Generate Tests
```bash
./tichy tests generate --num 20 --output tests.json
```

### Evaluate RAG Performance
```bash
./tichy tests evaluate --input tests.json
```

## Services

- **PostgreSQL + pgvector**: Vector database (port 5432)
- **LLM Server**: llama.cpp inference server (port 8080)
- **Embeddings Server**: llama.cpp embeddings server (port 8081)

## Configuration

Key environment variables in `.env`:
- `DATABASE_URL`: PostgreSQL connection string
- `LLM_SERVER_URL`: LLM inference endpoint
- `EMBEDDING_SERVER_URL`: Embeddings endpoint
- `SYSTEM_PROMPT_TEMPLATE`: Path to system prompt template
- `CHUNK_SIZE`: Document chunk size (default: 500)
- `CHUNK_OVERLAP`: Chunk overlap (default: 100)
- `TOP_K`: Number of results to retrieve (default: 10)

## Acknowledgments

The example insurance knowledge base in `examples/insurellm/` is derived from the dataset provided by [LLM Engineering course](https://github.com/ed-donner/llm_engineering).

## License

BSD 3-Clause - see [LICENSE](LICENSE) for details.
