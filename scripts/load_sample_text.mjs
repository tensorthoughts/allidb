#!/usr/bin/env node
import fs from 'fs/promises';
import path from 'path';
import { fileURLToPath } from 'url';

// -------- Config --------
const ALLIDB_BASE_URL = process.env.ALLIDB_BASE_URL || 'http://localhost:7001';
const ALLIDB_API_KEY  = process.env.ALLIDB_API_KEY  || 'test-api-key';
// Default tenant must match configs/allidb.yaml security.auth.static_api_keys[*].tenant_id
const TENANT_ID       = process.env.ALLIDB_TENANT_ID || 'tenant1';

const OLLAMA_URL      = process.env.OLLAMA_URL || 'http://localhost:11434';
const OLLAMA_MODEL    = process.env.OLLAMA_MODEL || 'nomic-embed-text';

// Path to sample text
const __filename = fileURLToPath(import.meta.url);
const __dirname  = path.dirname(__filename);
const SAMPLE_PATH = path.join(__dirname, '..', 'benchmark', 'data', 'sample_text.txt');

// ------------------------

async function main() {
  console.log('[loader] Reading sample text from:', SAMPLE_PATH);
  const raw = await fs.readFile(SAMPLE_PATH, 'utf8');

  // Chunk on "---" separators; trim empty chunks
  const rawChunks = raw
    .split('\n---\n')
    .map(c => c.trim())
    .filter(c => c.length > 0);

  // Further split very long chunks into smaller pieces (optional)
  const chunks = [];
  const MAX_CHARS = 1500;
  for (const chunk of rawChunks) {
    if (chunk.length <= MAX_CHARS) {
      chunks.push(chunk);
    } else {
      // naive split by paragraphs
      const paras = chunk.split(/\n\s*\n/);
      let buf = '';
      for (const p of paras) {
        if ((buf + '\n\n' + p).length > MAX_CHARS && buf.length > 0) {
          chunks.push(buf.trim());
          buf = p;
        } else {
          buf = buf ? buf + '\n\n' + p : p;
        }
      }
      if (buf.trim().length > 0) chunks.push(buf.trim());
    }
  }

  console.log(`[loader] Total chunks to insert: ${chunks.length}`);

  let index = 0;
  for (const text of chunks) {
    index += 1;
    const entityId = `sample-doc-${index.toString().padStart(3, '0')}`;

    console.log(`\n[loader] Processing ${entityId}...`);

    const vector = await embedWithOllama(text);
    console.log(`[loader] Got embedding dim=${vector.length}`);

    await putEntity(entityId, text, vector);
    console.log(`[loader] Inserted ${entityId} into AlliDB`);
  }

  console.log('\n[loader] Done. All chunks inserted.');
}

async function embedWithOllama(text) {
  const res = await fetch(`${OLLAMA_URL}/api/embeddings`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      model: OLLAMA_MODEL,
      prompt: text,
    }),
  });

  if (!res.ok) {
    const body = await res.text();
    throw new Error(`Ollama embeddings failed: ${res.status} ${body}`);
  }

  const json = await res.json();
  if (!json.embedding || !Array.isArray(json.embedding)) {
    throw new Error('Ollama response missing "embedding" array');
  }
  return json.embedding;
}

async function putEntity(entityId, text, vector) {
  // Use a numeric timestamp (int64 in Go) – JS number is sufficient for tests.
  const nowNanos = Date.now() * 1_000_000;

  const entityPayload = {
    entity: {
      entity_id: entityId,
      tenant_id: TENANT_ID,
      entity_type: 'document',
      embeddings: [
        {
          version: 1,
          vector: vector,
          timestamp_nanos: nowNanos,
        },
      ],
      chunks: [
        {
          chunk_id: `${entityId}-chunk-1`,
          text: text,
          metadata: {
            source: 'sample_text',
          },
        },
      ],
      importance: 0.5,
      created_at_nanos: nowNanos,
      updated_at_nanos: nowNanos,
    },
    consistency_level: 'QUORUM',
    trace_id: `trace-${entityId}`,
  };

  const res = await fetch(`${ALLIDB_BASE_URL}/entity`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-ALLIDB-API-KEY': ALLIDB_API_KEY,
    },
    body: JSON.stringify(entityPayload),
  });

  if (!res.ok) {
    const body = await res.text();
    throw new Error(`AlliDB POST /entity failed: ${res.status} ${body}`);
  }

  const json = await res.json().catch(() => ({}));
  if (json.success === false) {
    throw new Error(`AlliDB responded with error: ${JSON.stringify(json)}`);
  }
}

// Run
main().catch(err => {
  console.error('[loader] ERROR:', err);
  process.exit(1);
});


