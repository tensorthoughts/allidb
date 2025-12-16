# ADR-005: LLM Integration for Entity Extraction

## Status
Accepted

## Context
AlliDB is designed for RAG (Retrieval-Augmented Generation) workloads. A key requirement is extracting entities and relationships from unstructured text to build a knowledge graph. This requires:
- Entity recognition
- Relationship extraction
- Structured output
- Schema validation
- Error handling

Manual extraction is not scalable. Traditional NLP tools (NER, relation extraction models) are limited and require training. LLMs provide a flexible, zero-shot solution.

## Decision
We will integrate **LLMs for entity and relation extraction** with:

### Architecture
1. **Pluggable Backend**:
   - Support multiple LLM providers (OpenAI, Ollama, local models)
   - Abstract interface for different backends
   - Easy to add new providers

2. **Strict JSON Output**:
   - Enforce JSON schema in prompts
   - Parse and validate responses
   - Handle markdown code blocks

3. **Retry Logic**:
   - Retry on malformed responses
   - Configurable max retries and delays
   - Exponential backoff support

4. **Schema Validation**:
   - Optional JSON schema validation
   - Ensures extracted data matches expected format
   - Rejects invalid extractions

### Output Format
```json
{
  "entities": [
    {
      "id": "entity-1",
      "type": "Person",
      "properties": {...}
    }
  ],
  "relations": [
    {
      "from": "entity-1",
      "to": "entity-2",
      "type": "KNOWS",
      "weight": 0.8
    }
  ]
}
```

## Consequences

### Positive
- **Flexibility**: Works with any LLM provider
- **Zero-shot**: No training required
- **High quality**: LLMs understand context and semantics
- **Extensible**: Easy to add new extraction patterns
- **Schema enforcement**: Validates output structure

### Negative
- **Latency**: LLM calls are slower than local models
- **Cost**: API calls can be expensive at scale
- **Non-deterministic**: Same input may produce different output
- **Dependency**: Requires external LLM service (unless local)
- **Error handling**: Malformed responses need retry logic

### Trade-offs
- Chose LLMs over trained models for flexibility
- JSON over free-form text for structured output
- Retry logic over fail-fast for reliability
- Pluggable backends over single provider for flexibility

## Implementation Notes
- Extractor: `core/llm/extractor.go`
- Backends: `core/llm/backends.go`
- AI Factory: `core/ai/factory.go` (creates embedders, LLMs, extractors)
- Interfaces: `core/ai/interfaces.go` (Embedder, LLM, EntityExtractor)
- Configurable prompts and schemas
- Supports both API and local LLM backends
- Secret provider: `core/security/secrets/provider.go` (environment-based)
- Configuration-driven: All AI providers configured via YAML
- Pluggable architecture: Easy to add new providers via factory pattern
- Supports OpenAI, Ollama, local, and custom providers
- Embeddings and LLM can use different providers

## Alternatives Considered
1. **Trained NER models**: Rejected - requires training data and domain expertise
2. **Rule-based extraction**: Rejected - too rigid, doesn't scale
3. **Hybrid approach**: Future consideration - use LLM for complex cases, rules for simple

