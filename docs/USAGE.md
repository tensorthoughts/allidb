# AlliDB Usage Guide

## Table of Contents

1. [Getting Started](#getting-started)
2. [Vectorizing Data](#vectorizing-data)
3. [Storing Entities](#storing-entities)
4. [Vector Search](#vector-search)
5. [Graph Search](#graph-search)
6. [Combined Queries](#combined-queries)
7. [Batch Operations](#batch-operations)
8. [Complete Examples](#complete-examples)

---

## Getting Started

### Prerequisites

- AlliDB server running (see [README.md](./README.md) for setup)
- API key configured (if authentication enabled)
- Vector embeddings ready (or use AlliDB's AI integration)
- Optional sample corpus at `benchmark/data/sample_text.txt` for quick tests

### Base URL

- **REST API**: `http://localhost:7001` (default)
- **gRPC API**: `localhost:7000` (default)

### Authentication

All requests (except `/health`) require an API key:

```bash
export ALLIDB_API_KEY="your-api-key-here"
```

---

## Vectorizing Data

### Option 1: Using OpenAI Embeddings

```python
import openai
import json

# Initialize OpenAI client
client = openai.OpenAI(api_key="your-openai-api-key")

# Text to vectorize
text = "AlliDB is a vector + graph database for RAG workloads"

# Generate embedding
response = client.embeddings.create(
    model="text-embedding-3-large",
    input=text
)

# Extract vector
vector = response.data[0].embedding
print(f"Vector dimension: {len(vector)}")
print(f"Vector: {vector[:5]}...")  # First 5 dimensions
```

### Option 2: Using Ollama (Local)

```python
import requests
import json

# Ollama API endpoint
url = "http://localhost:11434/api/embeddings"

# Text to vectorize
text = "AlliDB is a vector + graph database for RAG workloads"

# Generate embedding
response = requests.post(url, json={
    "model": "nomic-embed-text",
    "prompt": text
})

# Extract vector
result = response.json()
vector = result["embedding"]
print(f"Vector dimension: {len(vector)}")
```

#### Running Ollama locally

1. Install Ollama from the official site and start the daemon:
   ```bash
   # macOS / Linux (see ollama.com for details)
   ollama serve
   ```

2. Pull an embedding model, for example:
   ```bash
   ollama pull nomic-embed-text
   ```

3. Verify the embeddings endpoint:
   ```bash
   curl http://localhost:11434/api/embeddings \
     -d '{"model":"nomic-embed-text","prompt":"test"}'
   ```

#### Configuring AlliDB to use Ollama

In `configs/allidb.yaml`, set the AI providers to `ollama` and make sure the embedding dimension matches the model you use:

```yaml
ai:
  embeddings:
    provider: "ollama"              # openai | ollama | local | custom
    model: "nomic-embed-text"
    dim: 768                        # match your Ollama model's embedding size
    timeout_ms: 3000

  llm:
    provider: "ollama"              # optional, if you want to use Ollama as LLM
    model: "llama3"
    temperature: 0.0
    max_tokens: 2048
    timeout_ms: 5000

  entity_extraction:
    enabled: true
    max_retries: 2
```

After updating the config, restart AlliDB:

```bash
./build/allidbd -config ./build/configs/allidb.yaml
```

### Option 3: Using AlliDB's AI Integration

If AlliDB is configured with AI providers, you can use the embedding service:

```bash
# Note: This requires AlliDB's AI integration to be configured
# See CONFIGURATION.md for AI setup
```

### Vector Dimensions

- **OpenAI text-embedding-3-large**: 3072 dimensions
- **OpenAI text-embedding-3-small**: 1536 dimensions
- **Ollama nomic-embed-text**: 768 dimensions

**Important**: All entities in AlliDB must use the same vector dimension (configured in `ai.embeddings.dim`).

---

## Storing Entities

### Basic Entity Storage

An entity in AlliDB contains:
- Entity ID (unique identifier)
- Tenant ID (for multi-tenancy)
- Entity Type (classification)
- Vector embeddings (one or more)
- Graph edges (relationships)
- Text chunks (content)
- Importance score
- Timestamps

### Example: Store a Document Entity

```bash
curl -X POST http://localhost:7001/entity \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "entity": {
      "entity_id": "doc-001",
      "tenant_id": "tenant-1",
      "entity_type": "document",
      "embeddings": [
        {
          "version": 1,
          "vector": [0.1, 0.2, 0.3, 0.4, 0.5],
          "timestamp_nanos": 1234567890000000000
        }
      ],
      "chunks": [
        {
          "chunk_id": "chunk-1",
          "text": "AlliDB is a vector + graph database designed for RAG workloads.",
          "metadata": {
            "source": "documentation",
            "page": "1"
          }
        }
      ],
      "importance": 0.8,
      "created_at_nanos": 1234567890000000000,
      "updated_at_nanos": 1234567890000000000
    },
    "consistency_level": "QUORUM",
    "trace_id": "trace-abc123"
  }'
```

### End-to-End Local Example (vector + graph)

1) Generate or supply an embedding (replace `EMB` with real values, e.g., from OpenAI/Ollama) using `benchmark/data/sample_text.txt` as the input text:
```bash
EMB='[0.12,0.05,0.33,0.44]'
```

2) Store an entity with a vector and an outgoing edge:
```bash
curl -X POST http://localhost:7001/entity \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "entity": {
      "entity_id": "doc-1",
      "tenant_id": "tenant-1",
      "entity_type": "document",
      "embeddings": [
        {"version":1,"vector":'"${EMB}"',"timestamp_nanos":1680000000000000000}
      ],
      "outgoing_edges": [
        {"from_entity":"doc-1","to_entity":"doc-2","relation":"RELATED","weight":0.8,"timestamp":1680000100}
      ]
    },
    "consistency_level": "QUORUM",
    "trace_id": "trace-sample-1"
  }'
```

3) Query by vector:
```bash
curl -X POST http://localhost:7001/query \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "query_vector": '"${EMB}"',
    "k": 5,
    "expand_factor": 2
  }'
```

4) Range scan (local):
```bash
curl -X POST http://localhost:7001/query/range \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "start_key": "doc-",
    "end_key": "doc.zzz",
    "limit": 50
  }'
```

**Response**:
```json
{
  "success": true,
  "entity_id": "doc-001"
}
```

### Example: Store Entity with Graph Edges

```bash
curl -X POST http://localhost:7001/entity \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "entity": {
      "entity_id": "person-alice",
      "tenant_id": "tenant-1",
      "entity_type": "person",
      "embeddings": [
        {
          "version": 1,
          "vector": [0.1, 0.2, 0.3],
          "timestamp_nanos": 1234567890000000000
        }
      ],
      "outgoing_edges": [
        {
          "from_entity": "person-alice",
          "to_entity": "person-bob",
          "relation": "KNOWS",
          "weight": 0.9,
          "timestamp_nanos": 1234567890000000000
        },
        {
          "from_entity": "person-alice",
          "to_entity": "company-acme",
          "relation": "WORKS_AT",
          "weight": 1.0,
          "timestamp_nanos": 1234567890000000000
        }
      ],
      "importance": 0.7
    },
    "consistency_level": "QUORUM"
  }'
```

### Example: Store Entity with Multiple Embeddings

```bash
curl -X POST http://localhost:7001/entity \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "entity": {
      "entity_id": "doc-002",
      "tenant_id": "tenant-1",
      "entity_type": "document",
      "embeddings": [
        {
          "version": 1,
          "vector": [0.1, 0.2, 0.3],
          "timestamp_nanos": 1234567890000000000
        },
        {
          "version": 2,
          "vector": [0.2, 0.3, 0.4],
          "timestamp_nanos": 1234567900000000000
        }
      ],
      "importance": 0.6
    }
  }'
```

**Note**: Multiple embeddings allow versioning. The latest version is used for search by default.

---

## Vector Search

### Basic Vector Search

Find entities similar to a query vector:

```bash
# First, create a query vector (using your embedding model)
# For this example, we'll use a sample vector

curl -X POST http://localhost:7001/query \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3, 0.4, 0.5],
    "k": 10,
    "consistency_level": "QUORUM"
  }'
```

**Response**:
```json
{
  "results": [
    {
      "entity_id": "doc-001",
      "score": 0.95,
      "entity": {
        "entity_id": "doc-001",
        "tenant_id": "tenant-1",
        "entity_type": "document",
        "embeddings": [...],
        "importance": 0.8
      }
    },
    {
      "entity_id": "doc-002",
      "score": 0.87,
      "entity": {...}
    }
  ],
  "total_results": 10
}
```

### Vector Search with Graph Expansion

Enable graph-based candidate expansion:

```bash
curl -X POST http://localhost:7001/query \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3, 0.4, 0.5],
    "k": 10,
    "expand_factor": 2,
    "consistency_level": "QUORUM"
  }'
```

**Parameters**:
- `k`: Number of results to return (default: 10)
- `expand_factor`: Graph expansion multiplier (default: 2)
  - `expand_factor=1`: No graph expansion (pure vector search)
  - `expand_factor=2`: Expand to 2x candidates via graph
  - `expand_factor=3`: Expand to 3x candidates via graph

### Vector Search with Consistency Levels

```bash
# Fastest (ONE replica)
curl -X POST http://localhost:7001/query \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3],
    "k": 10,
    "consistency_level": "ONE"
  }'

# Balanced (QUORUM - default)
curl -X POST http://localhost:7001/query \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3],
    "k": 10,
    "consistency_level": "QUORUM"
  }'

# Strongest (ALL replicas)
curl -X POST http://localhost:7001/query \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3],
    "k": 10,
    "consistency_level": "ALL"
  }'
```

---

## Graph Search

### Graph Traversal via Vector Search

AlliDB combines vector search with graph traversal. To perform graph-based search:

1. **Start with vector search** to find initial candidates
2. **Expand via graph edges** to find related entities
3. **Score combined results** using unified scoring

```bash
# Graph search: Find entities similar to query, then expand via relationships
curl -X POST http://localhost:7001/query \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3],
    "k": 10,
    "expand_factor": 3,
    "consistency_level": "QUORUM"
  }'
```

**How it works**:
1. Vector search finds top-K similar entities
2. For each candidate, traverse graph edges (1-3 hops)
3. Add neighbors to candidate set
4. Score all candidates (vector similarity + graph structure)
5. Return top-K results

### Building a Knowledge Graph

To enable effective graph search, build relationships between entities:

```bash
# Store entity A
curl -X POST http://localhost:7001/entity \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "entity": {
      "entity_id": "person-alice",
      "tenant_id": "tenant-1",
      "entity_type": "person",
      "embeddings": [{"version": 1, "vector": [0.1, 0.2, 0.3]}],
      "outgoing_edges": [
        {
          "from_entity": "person-alice",
          "to_entity": "person-bob",
          "relation": "KNOWS",
          "weight": 0.9
        }
      ]
    }
  }'

# Store entity B (with incoming edge from A)
curl -X POST http://localhost:7001/entity \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "entity": {
      "entity_id": "person-bob",
      "tenant_id": "tenant-1",
      "entity_type": "person",
      "embeddings": [{"version": 1, "vector": [0.2, 0.3, 0.4]}],
      "incoming_edges": [
        {
          "from_entity": "person-alice",
          "to_entity": "person-bob",
          "relation": "KNOWS",
          "weight": 0.9
        }
      ],
      "outgoing_edges": [
        {
          "from_entity": "person-bob",
          "to_entity": "company-acme",
          "relation": "WORKS_AT",
          "weight": 1.0
        }
      ]
    }
  }'
```

**Note**: Edges are bidirectional. You can define them in either direction, but it's recommended to define them in both for efficient traversal.

### Graph Edge Types

Common relationship types:
- `KNOWS`: Person-to-person relationships
- `WORKS_AT`: Person-to-company relationships
- `RELATED_TO`: General relationships
- `CONTAINS`: Hierarchical relationships
- `SIMILAR_TO`: Similarity relationships
- `CITES`: Citation relationships

---

## Combined Queries

### Vector + Graph Hybrid Search

The unified index automatically combines vector similarity and graph structure:

```bash
curl -X POST http://localhost:7001/query \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3],
    "k": 10,
    "expand_factor": 2,
    "consistency_level": "QUORUM"
  }'
```

**Scoring Formula**:
```
score = 0.5 * cosine_similarity(vector, query)
      + 0.3 * graph_traversal_score(edges)
      + 0.15 * entity_importance
      + 0.05 * recency_decay(timestamp)
```

**Weights are configurable** in `query.scoring` section of config.

---

## Batch Operations

### Batch Put Entities

Store multiple entities in a single request:

```bash
curl -X POST http://localhost:7001/entities/batch \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "entities": [
      {
        "entity_id": "doc-001",
        "tenant_id": "tenant-1",
        "entity_type": "document",
        "embeddings": [{"version": 1, "vector": [0.1, 0.2, 0.3]}],
        "importance": 0.8
      },
      {
        "entity_id": "doc-002",
        "tenant_id": "tenant-1",
        "entity_type": "document",
        "embeddings": [{"version": 1, "vector": [0.2, 0.3, 0.4]}],
        "importance": 0.7
      },
      {
        "entity_id": "doc-003",
        "tenant_id": "tenant-1",
        "entity_type": "document",
        "embeddings": [{"version": 1, "vector": [0.3, 0.4, 0.5]}],
        "importance": 0.6
      }
    ],
    "consistency_level": "QUORUM"
  }'
```

**Response**:
```json
{
  "success": true,
  "entity_ids": ["doc-001", "doc-002", "doc-003"],
  "success_count": 3,
  "failure_count": 0
}
```

**Performance**: Batch operations are 10-100x faster than individual puts.

---

## Complete Examples

### Example 1: RAG Document Store

Store documents with embeddings and search:

```bash
#!/bin/bash

# 1. Vectorize a document
# (In production, use OpenAI/Ollama API)
QUERY_TEXT="What is AlliDB?"
# ... vectorize using your embedding model ...
QUERY_VECTOR="[0.1, 0.2, 0.3, ...]"  # 3072 dimensions

# 2. Store document
curl -X POST http://localhost:7001/entity \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d "{
    \"entity\": {
      \"entity_id\": \"doc-rag-001\",
      \"tenant_id\": \"tenant-1\",
      \"entity_type\": \"document\",
      \"embeddings\": [{
        \"version\": 1,
        \"vector\": ${QUERY_VECTOR},
        \"timestamp_nanos\": $(date +%s%N)
      }],
      \"chunks\": [{
        \"chunk_id\": \"chunk-1\",
        \"text\": \"AlliDB is a vector + graph database for RAG workloads.\",
        \"metadata\": {
          \"source\": \"documentation\",
          \"page\": \"1\"
        }
      }],
      \"importance\": 0.9
    }
  }"

# 3. Search for similar documents
curl -X POST http://localhost:7001/query \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d "{
    \"query_vector\": ${QUERY_VECTOR},
    \"k\": 5,
    \"expand_factor\": 2
  }"
```

### Example 2: Knowledge Graph

Build a knowledge graph of people and companies:

```bash
#!/bin/bash

# Store person entities with relationships
curl -X POST http://localhost:7001/entity \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "entity": {
      "entity_id": "person-alice",
      "tenant_id": "tenant-1",
      "entity_type": "person",
      "embeddings": [{"version": 1, "vector": [0.1, 0.2, 0.3]}],
      "outgoing_edges": [
        {
          "from_entity": "person-alice",
          "to_entity": "person-bob",
          "relation": "KNOWS",
          "weight": 0.9
        },
        {
          "from_entity": "person-alice",
          "to_entity": "company-acme",
          "relation": "WORKS_AT",
          "weight": 1.0
        }
      ],
      "importance": 0.8
    }
  }'

curl -X POST http://localhost:7001/entity \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "entity": {
      "entity_id": "person-bob",
      "tenant_id": "tenant-1",
      "entity_type": "person",
      "embeddings": [{"version": 1, "vector": [0.2, 0.3, 0.4]}],
      "incoming_edges": [
        {
          "from_entity": "person-alice",
          "to_entity": "person-bob",
          "relation": "KNOWS",
          "weight": 0.9
        }
      ],
      "outgoing_edges": [
        {
          "from_entity": "person-bob",
          "to_entity": "company-acme",
          "relation": "WORKS_AT",
          "weight": 1.0
        }
      ],
      "importance": 0.7
    }
  }'

curl -X POST http://localhost:7001/entity \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "entity": {
      "entity_id": "company-acme",
      "tenant_id": "tenant-1",
      "entity_type": "company",
      "embeddings": [{"version": 1, "vector": [0.3, 0.4, 0.5]}],
      "incoming_edges": [
        {
          "from_entity": "person-alice",
          "to_entity": "company-acme",
          "relation": "WORKS_AT",
          "weight": 1.0
        },
        {
          "from_entity": "person-bob",
          "to_entity": "company-acme",
          "relation": "WORKS_AT",
          "weight": 1.0
        }
      ],
      "importance": 0.9
    }
  }'

# Search: Find people similar to Alice, then expand via graph
curl -X POST http://localhost:7001/query \
  -H "Content-Type: application/json" \
  -H "X-ALLIDB-API-KEY: ${ALLIDB_API_KEY}" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3],
    "k": 10,
    "expand_factor": 3
  }'
```

### Example 3: Python Client

```python
import requests
import json
import openai
from typing import List

class AlliDBClient:
    def __init__(self, base_url: str, api_key: str):
        self.base_url = base_url
        self.headers = {
            "Content-Type": "application/json",
            "X-ALLIDB-API-KEY": api_key
        }
    
    def put_entity(self, entity: dict, consistency_level: str = "QUORUM"):
        """Store an entity."""
        response = requests.put(
            f"{self.base_url}/entity",
            headers=self.headers,
            json={
                "entity": entity,
                "consistency_level": consistency_level
            }
        )
        return response.json()
    
    def query(self, query_vector: List[float], k: int = 10, 
              expand_factor: int = 2, consistency_level: str = "QUORUM"):
        """Perform vector + graph search."""
        response = requests.post(
            f"{self.base_url}/query",
            headers=self.headers,
            json={
                "query_vector": query_vector,
                "k": k,
                "expand_factor": expand_factor,
                "consistency_level": consistency_level
            }
        )
        return response.json()
    
    def batch_put(self, entities: List[dict], consistency_level: str = "QUORUM"):
        """Store multiple entities."""
        response = requests.post(
            f"{self.base_url}/entities/batch",
            headers=self.headers,
            json={
                "entities": entities,
                "consistency_level": consistency_level
            }
        )
        return response.json()

# Usage
client = AlliDBClient("http://localhost:7001", "your-api-key")

# Vectorize text
openai_client = openai.OpenAI(api_key="your-openai-key")
text = "AlliDB is a vector + graph database"
embedding = openai_client.embeddings.create(
    model="text-embedding-3-large",
    input=text
).data[0].embedding

# Store entity
entity = {
    "entity_id": "doc-001",
    "tenant_id": "tenant-1",
    "entity_type": "document",
    "embeddings": [{
        "version": 1,
        "vector": embedding,
        "timestamp_nanos": 1234567890000000000
    }],
    "chunks": [{
        "chunk_id": "chunk-1",
        "text": text,
        "metadata": {"source": "example"}
    }],
    "importance": 0.8
}
result = client.put_entity(entity)
print(f"Stored: {result['entity_id']}")

# Search
query_text = "What is AlliDB?"
query_embedding = openai_client.embeddings.create(
    model="text-embedding-3-large",
    input=query_text
).data[0].embedding

results = client.query(query_embedding, k=5, expand_factor=2)
for result in results["results"]:
    print(f"Entity: {result['entity_id']}, Score: {result['score']:.4f}")
```

### Example 4: Go Client

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type AlliDBClient struct {
    baseURL string
    apiKey  string
    client  *http.Client
}

func NewAlliDBClient(baseURL, apiKey string) *AlliDBClient {
    return &AlliDBClient{
        baseURL: baseURL,
        apiKey:  apiKey,
        client:  &http.Client{Timeout: 30 * time.Second},
    }
}

func (c *AlliDBClient) PutEntity(entity map[string]interface{}) error {
    reqBody := map[string]interface{}{
        "entity":            entity,
        "consistency_level": "QUORUM",
    }
    
    jsonData, _ := json.Marshal(reqBody)
    req, _ := http.NewRequest("POST", c.baseURL+"/entity", bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-ALLIDB-API-KEY", c.apiKey)
    
    resp, err := c.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    fmt.Println("Response:", string(body))
    return nil
}

func (c *AlliDBClient) Query(queryVector []float32, k int) error {
    reqBody := map[string]interface{}{
        "query_vector":      queryVector,
        "k":                 k,
        "expand_factor":     2,
        "consistency_level": "QUORUM",
    }
    
    jsonData, _ := json.Marshal(reqBody)
    req, _ := http.NewRequest("POST", c.baseURL+"/query", bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-ALLIDB-API-KEY", c.apiKey)
    
    resp, err := c.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    fmt.Println("Results:", string(body))
    return nil
}

func main() {
    client := NewAlliDBClient("http://localhost:7001", "your-api-key")
    
    entity := map[string]interface{}{
        "entity_id": "doc-001",
        "tenant_id": "tenant-1",
        "entity_type": "document",
        "embeddings": []map[string]interface{}{
            {
                "version":  1,
                "vector":   []float32{0.1, 0.2, 0.3},
                "timestamp_nanos": time.Now().UnixNano(),
            },
        },
        "importance": 0.8,
    }
    
    client.PutEntity(entity)
    
    queryVector := []float32{0.1, 0.2, 0.3}
    client.Query(queryVector, 10)
}
```

---

## Best Practices

### 1. Vector Dimensions

- **Use consistent dimensions**: All entities must use the same vector dimension; mixing 768‑dim and 3072‑dim vectors in one cluster will break ANN search.
- **Match embedding model**: Set `ai.embeddings.dim` to the exact size of your embedding model, e.g.:
  - OpenAI `text-embedding-3-large`: `dim: 3072`
  - OpenAI `text-embedding-3-small`: `dim: 1536`
  - Ollama `nomic-embed-text`: `dim: 768`
- **Per-tenant consistency**: If you ever change models, do it per-tenant and reindex; avoid mixing dimensions across tenants that share HNSW indexes.
- **Schema hint**: Store the model name in entity metadata (e.g. `metadata.model = "text-embedding-3-large"`) for audit/debugging.

### 2. Entity IDs

- **Use meaningful IDs**: e.g., `doc-001`, `person-alice`, `company-acme`; these show up in logs/metrics and debugging tools.
- **Ensure uniqueness per tenant**: `entity_id` must be unique *within a tenant*; two tenants can safely use the same `entity_id`.
- **Stable IDs for upserts**: Reuse the same `entity_id` when updating a document so timestamp-based deduplication in SSTables works as intended.
- **Namespace conventions**: For larger apps, use prefixes: `doc:<collection>:<id>`, `user:<region>:<id>`, etc., to make range scans easier to reason about.

### 3. Graph Edges

- **Directed vs. undirected**:
  - For friendship‑like relations, store both `A -> B` and `B -> A` with relation `KNOWS`.
  - For hierarchical or causal relations, use one direction only, e.g. `DOC_PARENT_OF_SECTION`.
- **Use meaningful relations**: Standardize relation strings, e.g., `REFERS_TO`, `CITES`, `SIMILAR_TO`, `ANSWERED_BY`, and document them in your app.
- **Set appropriate weights**:
  - Use `0.0–1.0` where `1.0` is a very strong edge (e.g. same paragraph, strong citation).
  - Down‑weight weaker edges (e.g. `0.1–0.3` for loose co‑occurrence).
- **Example edge payload**:
  ```json
  {
    "from_entity": "doc-1",
    "to_entity": "doc-2",
    "relation": "RELATED",
    "weight": 0.75,
    "timestamp": 1680000100
  }
  ```
- **Keep graphs sparse**: Prefer a few high‑quality edges over many noisy ones; dense graphs hurt performance and quality.

### 4. Batch Operations

- **Use batch puts**: 10-100x faster than individual puts
- **Batch size**: 100-1000 entities per batch (adjust based on entity size)
- **Handle failures**: Check `success_count` and `failure_count` in response
 - **Group by tenant**: Keep a batch single‑tenant to avoid surprising tenant‑guard failures.

### 5. Query Performance

- **Adjust expand_factor**: Higher values find more related entities but slower
- **Use appropriate k**: Request only the results you need
- **Consistency levels**: Use `ONE` for fastest, `QUORUM` for balanced, `ALL` for strongest
 - **Warm up caches**: Run a small “pre‑flight” workload (or replay recent queries) on new nodes to warm HNSW and entity caches.
 - **Use range scans for maintenance**: Use the range‑scan API to iterate over key ranges when building offline indices or running audits.

### 6. Importance Scores

- **Set meaningful scores**: Use 0.0-1.0 to indicate entity importance
- **Boost important entities**: Higher importance scores boost ranking
- **Update over time**: Adjust importance as entities become more/less relevant
 - **Practical scheme**:
   - Start all entities at `0.5`.
   - Boost entities that get clicked/viewed more often (e.g. +0.1 capped at 1.0).
   - Decay very old entities unless they are “evergreen” (e.g. reference docs).
 - **Combine with graph**: Let importance work together with graph edge weights so “central” or authoritative nodes rank higher in RAG results.

---

## Troubleshooting

### Common Issues

1. **"Invalid vector dimension"**
   - Ensure all vectors have the same dimension
   - Check `ai.embeddings.dim` in config matches your vectors

2. **"Authentication failed"**
   - Check API key is correct
   - Verify `X-ALLIDB-API-KEY` header is set
   - Ensure authentication is enabled in config

3. **"Tenant isolation violation"**
   - Ensure `tenant_id` matches across operations
   - Check tenant guard is configured correctly

4. **"Query timeout"**
   - Increase `query.timeout_ms` in config
   - Reduce `k` or `expand_factor`
   - Check cluster health

5. **"Low recall"**
   - Increase `ef_search` in HNSW config
   - Increase `expand_factor` in queries
   - Check vector quality

---

## Next Steps

- **API Reference**: See [API.md](./API.md) for complete API documentation
- **Configuration**: See [CONFIGURATION.md](./CONFIGURATION.md) for configuration options
- **Architecture**: See [ARCHITECTURE.md](./ARCHITECTURE.md) for system details
- **Performance**: See [PERFORMANCE.md](./PERFORMANCE.md) for optimization tips

