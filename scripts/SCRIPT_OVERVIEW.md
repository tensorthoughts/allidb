### AlliDB Scripts Overview

This document explains what the Node.js scripts in the `scripts/` folder do and how the example flows in the `.mjs` files work.

---

### `search_examples.mjs`

**Purpose**
- End-to-end demo client for AlliDB.
- Shows how to:
  - Store entities with vector embeddings and text chunks.
  - Run vector search (ANN via HNSW).
  - Use graph relationships in queries (graph traversal + Graph RAG).
  - Do batch writes and range scans over entity IDs.

**Configuration**
- Reads settings from environment variables (with sensible defaults):
  - `ALLIDB_BASE_URL` – HTTP gateway base URL (default: `http://localhost:7001`).
  - `ALLIDB_API_KEY` – API key (default: `test-api-key`).
  - `ALLIDB_TENANT_ID` – tenant ID (default: `tenant1`).
  - `OLLAMA_URL` – Ollama URL for embeddings (default: `http://localhost:11434`).
  - `OLLAMA_MODEL` – embedding model (default: `nomic-embed-text`).

**How requests are made**
- Central helper: `makeRequest(url, options, data?)`
  - Chooses `http` vs `https` based on URL.
  - Automatically sets `X-ALLIDB-API-KEY` for AlliDB calls.
  - Adds JSON `Content-Type` only when needed.
  - Serializes request body as JSON and parses JSON responses.

**Embeddings**
- `generateEmbedding(text)`:
  - Tries to call Ollama’s `/api/embeddings` with `{ model, prompt }`.
  - On success: returns the real embedding vector.
  - On failure (Ollama not running, etc.): logs a warning and returns a **mock 768‑dimensional float32 array**, so the demo still works even without a live embedding service.

**Core HTTP helpers**
- `healthCheck()` – calls `GET /health`:
  - Prints status and body.
  - Handles `401` explicitly and warns about API key misconfiguration but continues running.
- `putEntity(entity)` – calls `POST /entity` with:
  - `{ entity, consistency_level: "QUORUM" }`.
- `batchPutEntities(entities)` – calls `POST /entities/batch`.
- `queryVector(queryVector, k, expandFactor)` – calls `POST /query` with:
  - `{ query_vector, k, expand_factor, consistency_level: "QUORUM" }`.
- There are also helpers (later in the file) for:
  - Range scans.
  - Multi-hop graph traversal.

**Example flows**
- The script defines a series of `exampleX_*` functions and then runs them in order in `main()`:

1. **`example1_SimpleVectorSearch`**
   - Builds 4 “ML/NLP/CV” documents.
   - Generates embeddings using `generateEmbedding`.
   - Writes them via `putEntity`.
   - Builds a query embedding for `"neural networks and deep learning"`.
   - Calls `queryVector` and prints the ranked results (entity IDs, scores, and snippets).

2. **`example2_GraphTraversal`**
   - Creates a small person/relationship graph (e.g., Alice, Bob, Carol, Dave) as entities with graph edges.
   - Writes entities + edges.
   - Issues a query starting from “Alice” and uses graph expansion to traverse neighbors.
   - Shows how graph structure influences search results.

3. **`example3_GraphRAG`**
   - Creates documents + relationships between them (doc‑to‑doc edges).
   - Runs a vector search for a question (e.g., “How do neural networks work?”).
   - Expands the candidate set via the graph (neighbors of top vector hits).
   - Demonstrates how combining vector similarity + graph relationships surfaces related docs.

4. **`example4_BatchOperations`**
   - Builds a batch of synthetic documents in a loop.
   - Calls `batchPutEntities` once to write them efficiently.
   - Prints success / failure counts and timing.

5. **`example5_MultiHopGraphTraversal`**
   - Builds a chain graph (A → B → C → D).
   - Issues a query from the start node with an `expandFactor` large enough to walk multiple hops.
   - Demonstrates that the graph expansion step can find multi-hop neighbors (C, D) starting from A.

**Execution**
- The `main()` function roughly does:
  - print configuration (base URL, API key, tenant, Ollama settings),
  - run `healthCheck()`,
  - then run each of the examples above in sequence,
  - and print a summary at the end.

---

### `load_sample_text.mjs`

**Purpose**
- Utility to load and chunk text (e.g., from `benchmark/data/sample_text.txt`) and insert it into AlliDB.
- Typically used to simulate a larger text corpus and generate embeddings + chunks automatically.

**High-level flow**
- Reads the sample text file.
- Splits it into chunks (by size or paragraph/line boundaries).
- For each chunk:
  - Generates an embedding (via Ollama or mock, similar to `search_examples.mjs`).
  - Builds a `pb.Entity`-compatible JSON payload with:
    - `entity_id`,
    - `tenant_id`,
    - `entity_type` (e.g., `document` or `chunk`),
    - `embeddings`,
    - `chunks` with `metadata` (always string values).
  - Sends the entities to AlliDB via `POST /entity` or `POST /entities/batch`.

This script is ideal when you want to quickly load a realistic dataset into AlliDB and then use `search_examples.mjs` to query it.

---

### `run_local_debug.sh`

**Purpose**
- Convenience shell script to help you run or debug AlliDB locally together with the Node scripts.

**Typical responsibilities**
- Exports environment variables needed by the Node scripts:
  - `ALLIDB_BASE_URL`
  - `ALLIDB_API_KEY`
  - `ALLIDB_TENANT_ID`
  - `OLLAMA_URL`
  - `OLLAMA_MODEL`
- Optionally starts `allidbd` (or reminds you which command to run).
- Shows example commands like:
  - `node scripts/search_examples.mjs`
  - `node scripts/load_sample_text.mjs`

Use this when you want a quick “one-command” setup for local debugging of both the server and client scripts.


