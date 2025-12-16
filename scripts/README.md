# AlliDB Scripts

This directory contains utility scripts for working with AlliDB.

## Available Scripts

### `load_sample_text.mjs`

Loads sample text from `benchmark/data/sample_text.txt`, chunks it, generates embeddings using Ollama, and stores entities in AlliDB.

**Usage:**
```bash
node scripts/load_sample_text.mjs
```

**Environment Variables:**
- `ALLIDB_BASE_URL` - AlliDB HTTP endpoint (default: `http://localhost:7001`)
- `ALLIDB_API_KEY` - API key for authentication (default: `test-api-key`)
- `ALLIDB_TENANT_ID` - Tenant ID (default: `tenant1`)
- `OLLAMA_URL` - Ollama API URL (default: `http://localhost:11434`)
- `OLLAMA_MODEL` - Ollama embedding model (default: `nomic-embed-text`)

**Example:**
```bash
ALLIDB_BASE_URL=http://localhost:7001 \
ALLIDB_API_KEY=your-api-key \
node scripts/load_sample_text.mjs
```

### `search_examples.mjs`

Comprehensive examples demonstrating AlliDB's search capabilities:
- **Example 1**: Simple vector search (ANN)
- **Example 2**: Graph traversal
- **Example 3**: Graph RAG (combined vector + graph search)
- **Example 4**: Batch operations
- **Example 5**: Multi-hop graph traversal

**Usage:**
```bash
node scripts/search_examples.mjs
```

**Environment Variables:**
- `ALLIDB_BASE_URL` - AlliDB HTTP endpoint (default: `http://localhost:7001`)
- `ALLIDB_API_KEY` - API key for authentication (default: `test-api-key`)
- `ALLIDB_TENANT_ID` - Tenant ID (default: `tenant1`)
- `OLLAMA_URL` - Ollama API URL (default: `http://localhost:11434`)
- `OLLAMA_MODEL` - Ollama embedding model (default: `nomic-embed-text`)

**Example:**
```bash
ALLIDB_BASE_URL=http://localhost:7001 \
ALLIDB_API_KEY=your-api-key \
OLLAMA_URL=http://localhost:11434 \
node scripts/search_examples.mjs
```

**What it demonstrates:**

1. **Vector Search**: Stores documents with embeddings and searches by vector similarity
2. **Graph Traversal**: Creates a knowledge graph (Person -> Company -> Product) and shows how graph relationships expand search results
3. **Graph RAG**: Combines vector similarity with graph relationships to find related documents
4. **Batch Operations**: Efficiently stores multiple entities in a single request
5. **Multi-Hop Traversal**: Traverses graph relationships across multiple hops (A -> B -> C -> D)

### `run_local_debug.sh`

Shell script for running AlliDB locally in debug mode.

**Usage:**
```bash
./scripts/run_local_debug.sh
```

## Prerequisites

1. **AlliDB Server**: Must be running and accessible
   ```bash
   # Start AlliDB (see main README for setup)
   ./build/allidbd -config configs/allidb.yaml
   ```

2. **Ollama** (optional, for embedding generation):
   ```bash
   # Install and start Ollama
   ollama serve
   
   # Pull an embedding model
   ollama pull nomic-embed-text
   ```

3. **Node.js**: Scripts require Node.js 18+ with ES modules support

## Quick Start

1. Start AlliDB server:
   ```bash
   make all
   ./build/allidbd -config configs/allidb.yaml
   ```

2. (Optional) Start Ollama for embeddings:
   ```bash
   ollama serve
   ollama pull nomic-embed-text
   ```

3. Run search examples:
   ```bash
   node scripts/search_examples.mjs
   ```

4. Load sample data:
   ```bash
   node scripts/load_sample_text.mjs
   ```

## Troubleshooting

### "Health check failed"
- Ensure AlliDB is running on the configured port (default: 7001)
- Check `ALLIDB_BASE_URL` environment variable

### "Failed to generate embedding"
- Script will use mock embeddings if Ollama is unavailable
- To use real embeddings, ensure Ollama is running and accessible

### "Tenant not found in context"
- Verify API key is configured in `configs/allidb.yaml`
- Check `ALLIDB_API_KEY` matches a configured key
- Ensure `ALLIDB_TENANT_ID` matches the tenant ID for your API key

## Script Output

The `search_examples.mjs` script provides detailed output for each example:

```
============================================================
AlliDB Search Examples
============================================================
Base URL: http://localhost:7001
API Key: test-api-k...
Tenant ID: tenant1
Ollama URL: http://localhost:11434
Ollama Model: nomic-embed-text

[1] Health Check
────────────────────────────────────────────────────────────
Status: 200
Response: { "healthy": true, "status": "ok", ... }

[2] Example 1: Simple Vector Search
────────────────────────────────────────────────────────────
...
```

Each example shows:
- What it demonstrates
- Entities being created
- Search queries and results
- Explanation of the results

