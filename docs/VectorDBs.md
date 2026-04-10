# Vector Database Backends

Tichy abstracts all vector store operations behind the `interfaces.Ingestor` and
`interfaces.Retriever` interfaces, so the backend can be swapped at runtime by
changing a single environment variable. This document describes each supported
backend, its trade-offs, and guidance on when to choose it.

---

## Selecting a Backend

Set `VECTORDB_BACKEND` in your `.env` file:

```bash
VECTORDB_BACKEND=pgvector   # PostgreSQL + pgvector
VECTORDB_BACKEND=qdrant     # Qdrant
```

No application code changes are needed when switching backends. The DI container
in `internal/injectors/injectors.go` wires the correct `Ingestor` and `Retriever`
implementation at startup.

---

## PostgreSQL + pgvector

[pgvector](https://github.com/pgvector/pgvector) is an open-source PostgreSQL
extension that adds a native `vector` column type and three similarity-search
operators (`<->` L2, `<#>` inner product, `<=>` cosine). Tichy stores document
chunks together with their embeddings in a single `chunks` table and queries it
with a `ORDER BY embedding <=> $1 LIMIT $2` nearest-neighbour search.

### Configuration

```bash
VECTORDB_BACKEND=pgvector
DATABASE_URL=postgres://user:pass@localhost:5432/tichy
EMBEDDING_DIMENSION=768
```

### Pros

- **Zero additional infrastructure.** If you are already running PostgreSQL (very
  common), adding the `pgvector` extension requires no new services.
- **Transactional consistency.** Chunks and their metadata live in the same
  ACID-compliant database as the rest of your relational data, so you can join them
  with other tables, roll back failed ingests, and use standard SQL tooling.
- **Familiar operations.** Backups, monitoring, access control, and migrations all
  use existing PostgreSQL skills and tooling. Tichy manages schema evolution via
  [goose](https://github.com/pressly/goose) migrations.
- **Low operational overhead for small datasets.** A single service handles both
  structured and vector data. No separate vector store process to manage.
- **Rich filtering.** Because chunks are stored in a regular SQL table, you can
  combine vector similarity with arbitrary `WHERE` clauses on metadata columns.

### Cons

- **Limited scalability for pure ANN workloads.** PostgreSQL performs an exact or
  HNSW/IVFFlat approximate search depending on the index you create. At very large
  scale (tens of millions of vectors) a dedicated ANN engine will outperform it on
  both throughput and latency.
- **No built-in multi-collection isolation.** pgvector does not have a "collection"
  concept. Tichy's `QueryCollection` method for pgvector silently ignores the
  collection parameter — all chunks share one table. Multi-tenant isolation requires
  separate schemas or tables with additional query predicates.
- **Memory pressure.** PostgreSQL loads index pages into its shared buffer pool. A
  large vector index can crowd out other data if the server is not sized accordingly.
- **Sequential scan fallback.** Without an explicit HNSW or IVFFlat index, similarity
  queries do a full sequential scan, which does not scale beyond a few hundred
  thousand rows.

### When to use

- You already have PostgreSQL in your stack and want to add RAG with minimal new
  infrastructure.
- Your dataset is small to medium (up to ~1 million vectors) and you do not need
  per-collection isolation.
- You need to combine vector search with relational filters or joins.
- You want full ACID guarantees on ingestion.
- You are running Tichy for development, testing, or a single-tenant deployment.

---

## Qdrant

[Qdrant](https://qdrant.tech) is a purpose-built vector similarity search engine
written in Rust. It exposes a gRPC and REST API, stores vectors in HNSW indexes,
and supports named **collections** as first-class objects. Tichy communicates with
Qdrant via the official `qdrant/go-client` gRPC client.

### Configuration

```bash
VECTORDB_BACKEND=qdrant
QDRANT_HOST=localhost
QDRANT_PORT=6333
QDRANT_COLLECTION=default_collection
QDRANT_COLLECTION_SIZE=768
QDRANT_API_KEY=                      # optional, for authenticated Qdrant instances
QDRANT_USE_TLS=false
QDRANT_RECREATE_COLLECTION=false     # set true to drop and recreate on ingest
```

### Pros

- **First-class multi-collection support.** Every collection is an independent HNSW
  index with its own vector dimension and distance metric. Tichy uses this directly
  to implement per-group or per-instrument knowledge isolation, and to fan out
  concurrent queries across collections (`QueryCollection`).
- **High performance at scale.** Qdrant's HNSW implementation is highly optimised
  and benchmarks favourably against other ANN engines at tens of millions of vectors.
- **Payload filtering.** Qdrant supports rich payload filters that are evaluated
  inside the HNSW graph (not as a post-filter), keeping query latency low even with
  selective filters.
- **Horizontal scalability.** Qdrant supports distributed mode with sharding and
  replication for production deployments requiring high availability or very large
  datasets.
- **Memory-mapped storage.** Large collections can be memory-mapped to disk,
  allowing Qdrant to handle datasets larger than available RAM.
- **Snapshots and backups.** Native snapshot and restore API, no need for
  PostgreSQL-style dump/restore workflows.

### Cons

- **Additional service to operate.** Qdrant is a separate process (or container)
  that must be deployed, monitored, and backed up independently.
- **No relational queries.** Qdrant stores unstructured payloads (JSON). If you
  need to join vector results with relational data, you must do it at the
  application layer.
- **No ACID transactions.** Qdrant prioritises throughput and availability; it does
  not offer multi-statement transactions. A failed bulk upsert may leave a
  collection in a partially-updated state.
- **Collection recreation required to change vector size.** If you switch embedding
  models (and thus change the vector dimension), you must drop and recreate the
  collection. The `QDRANT_RECREATE_COLLECTION=true` flag automates this.
- **Operational unfamiliarity.** Teams without prior Qdrant experience will need to
  learn its administration model, gRPC API, and monitoring approach.

### When to use

- You need **multi-collection isolation** — for example, per-group, per-instrument,
  or per-project knowledge bases that users can query independently or in
  combination (this is the primary use case driving Tichy's multi-collection
  feature).
- Your dataset is large (millions of vectors) and query latency at scale matters.
- You want to fan out concurrent queries across collections and aggregate results
  (see [multi-collection-rag.md](docs/multi-collection-rag.md)).
- You are running a multi-tenant system where different users must be restricted to
  different collections (enforced via JWT `vector_dbs` claims and FOXDEN access
  rules).
- You need payload filtering that is evaluated efficiently inside the ANN graph.
- You are building toward a production deployment and want native HA and
  replication support.

---

## Other Backends (Planned)

The interface-based architecture makes adding new backends straightforward. The
following are candidates for future implementation:

### Weaviate

[Weaviate](https://weaviate.io) is an open-source vector database with built-in
support for hybrid search (dense + BM25 sparse), a GraphQL API, and native
multi-tenancy. It is a strong candidate when you need hybrid keyword/vector search
or when you are already using a Weaviate deployment.

To add Weaviate support, implement `interfaces.Ingestor` and `interfaces.Retriever`
in a new `internal/weaviatestore/` package and register a `weaviate` case in
`injectors.go`.

### Milvus

[Milvus](https://milvus.io) is a cloud-native, highly scalable vector database
designed for billion-scale workloads. It supports multiple index types (HNSW, IVF,
DISKANN), GPU acceleration, and a rich SDK ecosystem. It is a strong candidate for
scientific computing environments (such as FOXDEN) where dataset sizes can be
very large and query throughput requirements are demanding.

### Comparison Summary

| Feature | pgvector | Qdrant | Weaviate | Milvus |
|---|---|---|---|---|
| Extra service required | No | Yes | Yes | Yes |
| Multi-collection support | No (single table) | Yes (native) | Yes (classes) | Yes (collections) |
| ACID transactions | Yes | No | No | No |
| Horizontal scaling | Limited | Yes | Yes | Yes |
| Hybrid search | Via SQL | Payload filter | Yes (native) | Partial |
| GPU acceleration | No | No | No | Yes |
| Best dataset size | < 1M vectors | 1M–100M+ vectors | 1M–100M+ vectors | 100M+ vectors |
| Operational complexity | Low | Medium | Medium | High |

---

## Implementing a New Backend

To add support for a new vector database:

1. Create `internal/<name>store/ingestor.go` implementing `interfaces.Ingestor`.
2. Create `internal/<name>store/retriever.go` implementing `interfaces.Retriever`
   (both `Query` and `QueryCollection`).
3. Add a new `case "<name>"` block to `provideIngestor` and `provideRetriever` in
   `internal/injectors/injectors.go`.
4. Add any required configuration fields to `internal/config/config.go`.
5. Document the new backend in this file.

The rest of the application — `responders`, `servers`, `commands`, and the
`interfaces` package itself — requires no changes.
