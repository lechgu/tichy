# Multi-Collection RAG: Concurrent Qdrant Query Support

## Overview

This document describes the design and data flow for querying multiple Qdrant
vector-database collections concurrently within the Tichy RAG pipeline. The feature
allows a single HTTP request to fan out across several collections, aggregate the
retrieved chunks, and return one unified LLM response to the caller.

---

## Use Case

A FOXDEN user belongs to one or more groups. Each group maps to one or more Qdrant
collections that hold domain-specific knowledge. Previously, only the **first**
collection in the user's list was queried. With this refactor:

* The **client** (`aichat.go`) fans out one HTTP request per allowed collection and
  merges the HTML responses.
* The **server** (`tichy`) accepts a `?collection=` query parameter and fans out
  concurrent Qdrant searches within a single HTTP call, then feeds all chunks into
  one LLM prompt.

Both strategies are composable: the client can target a subset of collections, and
the server handles the concurrent retrieval internally.

---

## API

### Server Endpoint

```
POST /v1/chat/completions?collection=<name>
```

| Parameter    | Location     | Description |
|--------------|--------------|-------------|
| `collection` | query string | One or more collection names. Accepts comma-separated values **or** repeated params. |

**Single collection**
```
POST /v1/chat/completions?collection=beamline_a
```

**Multiple collections – repeated params**
```
POST /v1/chat/completions?collection=beamline_a&collection=beamline_b
```

**Multiple collections – comma-separated**
```
POST /v1/chat/completions?collection=beamline_a,beamline_b,beamline_c
```

When no `collection` parameter is supplied the server falls back to the default
collection configured via the `QDRANT_COLLECTION` environment variable.

---

## Data Flow

### Server-Side Fan-Out

```
HTTP Client
    |
    |  POST /v1/chat/completions?collection=A&collection=B&collection=C
    v
+-------------------------------------------------------+
|  AuthzServer.handleChatCompletions                    |
|  1. Validate JWT -> User{VectorDBs: [...]}            |
|  2. Parse ?collection= query params -> [A, B, C]      |
|  3. Call responder.RespondMulti(collections=[A,B,C])  |
+------------------------+------------------------------+
                         |
                         v
+-------------------------------------------------------+
|  Responder.fetchChunks  (concurrent fan-out)          |
|                                                       |
|  goroutine A                                          |
|  QdrantRetriever.QueryCollection("A", query, topK) ---|--+
|                                                       |  |
|  goroutine B                                          |  |
|  QdrantRetriever.QueryCollection("B", query, topK) ---|--|--+
|                                                       |  |  |
|  goroutine C                                          |  |  |
|  QdrantRetriever.QueryCollection("C", query, topK) ---|--|--|--+
|                                                       |  |  |  |
|          results channel (buffered len=3)             |  |  |  |
|          <----- chunks A ----+---- chunks B ---+--- chunks C  |
|                              |                 |              |
|          merge: []Chunk from A + B + C         |              |
+------------------------------+-----------------+--------------+
                         |
                         v
+-------------------------------------------------------+
|  Responder.callLLM                                    |
|  system prompt = template + merged chunk context      |
|  -> LLM server /v1/chat/completions                   |
+------------------------+------------------------------+
                         |
                         v
              Single ChatCompletionResponse
              (one aggregated answer across all collections)
```

### Client-Side Fan-Out (aichat.go)

```
FOXDEN Frontend
    |
    |  aichat(user, prompt)
    v
+-------------------------------------------------------+
|  TichyClient.Chat                                     |
|  1. Resolve allowed collections from user groups      |
|     getVectorDbs(fuser) -> ["A", "B", "C"]           |
|  2. Fan out one aitichy() goroutine per collection    |
+-------+---------------+---------------+---------------+
        |               |               |
        v               v               v
   goroutine A     goroutine B     goroutine C
   POST ...?col=A  POST ...?col=B  POST ...?col=C
   (JWT token)     (JWT token)     (JWT token)
        |               |               |
        +-------+-------+-------+-------+
                        |
                        v  results channel
+-------------------------------------------------------+
|  Aggregate HTML responses                             |
|  Wrap each in <section data-collection="...">         |
|  Concatenate -> single string returned to caller      |
+-------------------------------------------------------+
```

---

## Changed Files

| File | Change |
|------|--------|
| `internal/vectorstore/interfaces.go` | Added `QueryCollection(ctx, collection, query, topK)` to `Retriever` interface |
| `internal/qdrantstore/retriever.go` | Renamed field to `defaultCollection`; added `QueryCollection` and private `queryCollection`; removed TODO |
| `internal/pgvectorstore/retreiver.go` | Added no-op `QueryCollection` to satisfy the updated interface |
| `internal/responders/responder.go` | Added `RespondMulti` with concurrent `fetchChunks`; `Respond` delegates to `RespondMulti` |
| `internal/servers/authz_server.go` | Added `parseCollections` helper; `handleChatCompletions` calls `RespondMulti` |
| `aichat.go` (client) | `TichyClient.Chat` fans out per-collection requests concurrently; `aitichy` appends `?collection=` to URL |

---

## Error Handling

Partial failures are tolerated at both layers:

* **Server** – if one collection's Qdrant search fails, its error is logged and
  chunks from the remaining collections are still passed to the LLM. The request
  only fails with HTTP 500 if **all** collections fail.
* **Client** – if one collection's HTTP call fails, its error is logged as a
  warning and the aggregated response is built from the remaining successful replies.
  Only when every call fails is an error returned to the caller.

---

## Configuration

No new environment variables are required. The existing `QDRANT_COLLECTION` value
continues to serve as the **default** collection when no `?collection=` parameter
is provided. Access-rule-based collection lists (via `AIChat.AccessRules`) continue
to control which collections each user group may reach.

---

## Example: curl

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

# Multiple collections (repeated params)
curl -X POST "http://tichy:8080/v1/chat/completions?collection=beamline_a&collection=beamline_b" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"What is the beam energy?"}]}'
```
