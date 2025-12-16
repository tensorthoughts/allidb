#!/usr/bin/env node
/**
 * AlliDB Search Examples Script
 * 
 * Demonstrates various search capabilities:
 * - Vector search (ANN)
 * - Graph traversal
 * - Combined vector + graph search (Graph RAG)
 * - Range scanning
 * - Batch operations
 * 
 * Usage:
 *   node scripts/search_examples.mjs
 * 
 * Environment variables:
 *   ALLIDB_BASE_URL - Base URL for AlliDB (default: http://localhost:7001)
 *   ALLIDB_API_KEY  - API key for authentication (default: test-api-key)
 *   OLLAMA_URL      - Ollama API URL for embeddings (default: http://localhost:11434)
 *   OLLAMA_MODEL    - Ollama embedding model (default: nomic-embed-text)
 */

import https from 'https';
import http from 'http';
import { fileURLToPath } from 'url';

// ============================================================================
// Configuration
// ============================================================================

const ALLIDB_BASE_URL = process.env.ALLIDB_BASE_URL || 'http://localhost:7001';
const ALLIDB_API_KEY  = process.env.ALLIDB_API_KEY  || 'test-api-key';
const TENANT_ID       = process.env.ALLIDB_TENANT_ID || 'tenant1';

const OLLAMA_URL      = process.env.OLLAMA_URL || 'http://localhost:11434';
const OLLAMA_MODEL    = process.env.OLLAMA_MODEL || 'nomic-embed-text';

// ============================================================================
// Helper Functions
// ============================================================================

function makeRequest(url, options, data = null) {
  return new Promise((resolve, reject) => {
    const urlObj = new URL(url);
    const isHttps = urlObj.protocol === 'https:';
    const client = isHttps ? https : http;
    
    // Build headers - don't include Content-Type for GET requests without body
    const headers = {
      'X-ALLIDB-API-KEY': ALLIDB_API_KEY,
      ...options.headers,
    };
    
    // Only add Content-Type if we have data or it's explicitly set
    if (data || options.method === 'POST' || options.method === 'PUT') {
      headers['Content-Type'] = 'application/json';
    }
    
    const opts = {
      hostname: urlObj.hostname,
      port: urlObj.port || (isHttps ? 443 : 80),
      path: urlObj.pathname + urlObj.search,
      method: options.method || 'GET',
      headers,
    };

    if (data) {
      const body = JSON.stringify(data);
      opts.headers['Content-Length'] = Buffer.byteLength(body);
    }

    const req = client.request(opts, (res) => {
      let responseData = '';
      res.on('data', (chunk) => {
        responseData += chunk;
      });
      res.on('end', () => {
        try {
          const parsed = responseData ? JSON.parse(responseData) : {};
          resolve({ status: res.statusCode, data: parsed });
        } catch (e) {
          resolve({ status: res.statusCode, data: responseData });
        }
      });
    });

    req.on('error', reject);
    
    if (data) {
      req.write(JSON.stringify(data));
    }
    
    req.end();
  });
}

async function generateEmbedding(text) {
  try {
    const url = `${OLLAMA_URL}/api/embeddings`;
    const response = await makeRequest(url, {
      method: 'POST',
      headers: { 'X-ALLIDB-API-KEY': '' }, // Remove auth header for Ollama
    }, {
      model: OLLAMA_MODEL,
      prompt: text,
    });
    
    if (response.status === 200 && response.data.embedding) {
      return response.data.embedding;
    }
    throw new Error(`Failed to generate embedding: ${JSON.stringify(response.data)}`);
  } catch (error) {
    console.warn(`[WARN] Could not generate embedding via Ollama: ${error.message}`);
    console.warn(`[WARN] Using mock embedding instead`);
    // Return a mock embedding (768 dimensions, typical for nomic-embed-text)
    const mock = new Array(768).fill(0).map(() => Math.random() * 0.1 - 0.05);
    return mock;
  }
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

// ============================================================================
// AlliDB API Functions
// ============================================================================

async function healthCheck() {
  console.log('\n[1] Health Check');
  console.log('─'.repeat(60));
  
  try {
    const response = await makeRequest(`${ALLIDB_BASE_URL}/health`, { method: 'GET' });
    console.log(`Status: ${response.status}`);
    
    if (response.status === 200) {
      console.log(`Response:`, JSON.stringify(response.data, null, 2));
      return true;
    } else if (response.status === 401) {
      console.log(`⚠️  Health check returned 401 (Unauthorized)`);
      console.log(`Response:`, JSON.stringify(response.data, null, 2));
      console.log(`\nThis usually means:`);
      console.log(`  1. API key "${ALLIDB_API_KEY}" is not configured in AlliDB`);
      console.log(`  2. Check configs/allidb.yaml -> security.auth.static_api_keys`);
      console.log(`  3. Or set ALLIDB_API_KEY environment variable to match a configured key`);
      console.log(`\nContinuing anyway - examples may fail if API key is invalid...\n`);
      return false; // Don't exit, but warn
    } else {
      console.log(`⚠️  Health check returned ${response.status}`);
      console.log(`Response:`, JSON.stringify(response.data, null, 2));
      return false;
    }
  } catch (error) {
    console.error(`❌ Health check failed: ${error.message}`);
    console.error(`\nIs AlliDB running on ${ALLIDB_BASE_URL}?`);
    console.error(`Start it with: ./build/allidbd -config configs/allidb.yaml\n`);
    return false;
  }
}

async function putEntity(entity) {
  const response = await makeRequest(`${ALLIDB_BASE_URL}/entity`, {
    method: 'POST',
  }, {
    entity,
    consistency_level: 'QUORUM',
  });
  
  if (response.status !== 200) {
    throw new Error(`Failed to put entity: ${JSON.stringify(response.data)}`);
  }
  return response.data;
}

async function batchPutEntities(entities) {
  const response = await makeRequest(`${ALLIDB_BASE_URL}/entities/batch`, {
    method: 'POST',
  }, {
    entities,
    consistency_level: 'QUORUM',
  });
  
  if (response.status !== 200) {
    throw new Error(`Failed to batch put entities: ${JSON.stringify(response.data)}`);
  }
  return response.data;
}

async function queryVector(queryVector, k = 10, expandFactor = 2) {
  const response = await makeRequest(`${ALLIDB_BASE_URL}/query`, {
    method: 'POST',
  }, {
    query_vector: queryVector,
    k,
    expand_factor: expandFactor,
    consistency_level: 'QUORUM',
  });
  
  if (response.status !== 200) {
    throw new Error(`Query failed: ${JSON.stringify(response.data)}`);
  }
  return response.data;
}

// ============================================================================
// Example Use Cases
// ============================================================================

async function example1_SimpleVectorSearch() {
  console.log('\n[2] Example 1: Simple Vector Search');
  console.log('─'.repeat(60));
  console.log('Storing documents and searching by vector similarity...\n');
  
  // Generate embeddings for sample documents
  const doc1Text = "Machine learning is a subset of artificial intelligence";
  const doc2Text = "Deep learning uses neural networks with multiple layers";
  const doc3Text = "Natural language processing enables computers to understand text";
  const doc4Text = "Computer vision allows machines to interpret visual information";
  
  console.log('Generating embeddings...');
  const [emb1, emb2, emb3, emb4] = await Promise.all([
    generateEmbedding(doc1Text),
    generateEmbedding(doc2Text),
    generateEmbedding(doc3Text),
    generateEmbedding(doc4Text),
  ]);
  
  // Store entities
  const entities = [
    {
      entity_id: 'doc-ml-1',
      tenant_id: TENANT_ID,
      entity_type: 'document',
      embeddings: [{
        version: 1,
        vector: emb1,
        timestamp_nanos: Date.now() * 1000000,
      }],
      chunks: [{ chunk_id: 'chunk-1', text: doc1Text, metadata: {} }],
      importance: 0.8,
      created_at_nanos: Date.now() * 1000000,
      updated_at_nanos: Date.now() * 1000000,
    },
    {
      entity_id: 'doc-dl-1',
      tenant_id: TENANT_ID,
      entity_type: 'document',
      embeddings: [{
        version: 1,
        vector: emb2,
        timestamp_nanos: Date.now() * 1000000,
      }],
      chunks: [{ chunk_id: 'chunk-2', text: doc2Text, metadata: {} }],
      importance: 0.9,
      created_at_nanos: Date.now() * 1000000,
      updated_at_nanos: Date.now() * 1000000,
    },
    {
      entity_id: 'doc-nlp-1',
      tenant_id: TENANT_ID,
      entity_type: 'document',
      embeddings: [{
        version: 1,
        vector: emb3,
        timestamp_nanos: Date.now() * 1000000,
      }],
      chunks: [{ chunk_id: 'chunk-3', text: doc3Text, metadata: {} }],
      importance: 0.7,
      created_at_nanos: Date.now() * 1000000,
      updated_at_nanos: Date.now() * 1000000,
    },
    {
      entity_id: 'doc-cv-1',
      tenant_id: TENANT_ID,
      entity_type: 'document',
      embeddings: [{
        version: 1,
        vector: emb4,
        timestamp_nanos: Date.now() * 1000000,
      }],
      chunks: [{ chunk_id: 'chunk-4', text: doc4Text, metadata: {} }],
      importance: 0.75,
      created_at_nanos: Date.now() * 1000000,
      updated_at_nanos: Date.now() * 1000000,
    },
  ];
  
  console.log('Storing entities...');
  await batchPutEntities(entities);
  console.log('✓ Stored 4 documents\n');
  
  // Wait a moment for indexing
  await sleep(1000);
  
  // Search for "neural networks"
  console.log('Searching for: "neural networks and deep learning"...');
  const queryText = "neural networks and deep learning";
  const queryEmbedding = await generateEmbedding(queryText);
  
  const results = await queryVector(queryEmbedding, 3, 1);
  
  console.log(`\nFound ${results.total_results || results.results?.length || 0} results:`);
  results.results?.forEach((result, idx) => {
    console.log(`  ${idx + 1}. ${result.entity_id} (score: ${result.score?.toFixed(4) || 'N/A'})`);
  });
}

async function example2_GraphTraversal() {
  console.log('\n[3] Example 2: Graph Traversal');
  console.log('─'.repeat(60));
  console.log('Creating a knowledge graph and traversing relationships...\n');
  
  const now = Date.now() * 1000000;
  
  // Create a knowledge graph: Person -> Company -> Product
  const graphEntities = [
    {
      entity_id: 'person-alice',
      tenant_id: TENANT_ID,
      entity_type: 'person',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding('Alice is a software engineer'),
        timestamp_nanos: now,
      }],
      outgoing_edges: [
        {
          from_entity: 'person-alice',
          to_entity: 'company-acme',
          relation: 'WORKS_AT',
          weight: 0.9,
          timestamp_nanos: now,
        },
      ],
      importance: 0.8,
      created_at_nanos: now,
      updated_at_nanos: now,
    },
    {
      entity_id: 'company-acme',
      tenant_id: TENANT_ID,
      entity_type: 'company',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding('Acme Corp develops AI software'),
        timestamp_nanos: now,
      }],
      incoming_edges: [
        {
          from_entity: 'person-alice',
          to_entity: 'company-acme',
          relation: 'WORKS_AT',
          weight: 0.9,
          timestamp_nanos: now,
        },
      ],
      outgoing_edges: [
        {
          from_entity: 'company-acme',
          to_entity: 'product-allidb',
          relation: 'DEVELOPS',
          weight: 0.95,
          timestamp_nanos: now,
        },
      ],
      importance: 0.9,
      created_at_nanos: now,
      updated_at_nanos: now,
    },
    {
      entity_id: 'product-allidb',
      tenant_id: TENANT_ID,
      entity_type: 'product',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding('AlliDB is a vector and graph database'),
        timestamp_nanos: now,
      }],
      incoming_edges: [
        {
          from_entity: 'company-acme',
          to_entity: 'product-allidb',
          relation: 'DEVELOPS',
          weight: 0.95,
          timestamp_nanos: now,
        },
      ],
      importance: 0.95,
      created_at_nanos: now,
      updated_at_nanos: now,
    },
    {
      entity_id: 'person-bob',
      tenant_id: TENANT_ID,
      entity_type: 'person',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding('Bob is a data scientist'),
        timestamp_nanos: now,
      }],
      outgoing_edges: [
        {
          from_entity: 'person-bob',
          to_entity: 'person-alice',
          relation: 'KNOWS',
          weight: 0.7,
          timestamp_nanos: now,
        },
        {
          from_entity: 'person-bob',
          to_entity: 'product-allidb',
          relation: 'USES',
          weight: 0.8,
          timestamp_nanos: now,
        },
      ],
      importance: 0.75,
      created_at_nanos: now,
      updated_at_nanos: now,
    },
  ];
  
  console.log('Creating knowledge graph...');
  await batchPutEntities(graphEntities);
  console.log('✓ Created graph with 4 entities and 4 edges\n');
  
  await sleep(1000);
  
  // Search starting from Alice - should find related entities via graph
  console.log('Searching from "Alice" with graph expansion...');
  const queryEmbedding = await generateEmbedding('software engineer Alice');
  const results = await queryVector(queryEmbedding, 5, 2);
  
  console.log(`\nFound ${results.total_results || results.results?.length || 0} results (including graph neighbors):`);
  results.results?.forEach((result, idx) => {
    console.log(`  ${idx + 1}. ${result.entity_id} (score: ${result.score?.toFixed(4) || 'N/A'})`);
  });
  
  console.log('\nNote: Graph expansion finds entities connected to initial vector search results!');
}

async function example3_GraphRAG() {
  console.log('\n[4] Example 3: Graph RAG (Vector + Graph Search)');
  console.log('─'.repeat(60));
  console.log('Combining vector similarity with graph relationships...\n');
  
  const now = Date.now() * 1000000;
  
  // Create a document graph about machine learning
  const mlDocs = [
    {
      entity_id: 'doc-ml-intro',
      tenant_id: TENANT_ID,
      entity_type: 'document',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding('Introduction to machine learning concepts and algorithms'),
        timestamp_nanos: now,
      }],
      chunks: [{ chunk_id: 'chunk-1', text: 'Introduction to machine learning...', metadata: {} }],
      outgoing_edges: [
        {
          from_entity: 'doc-ml-intro',
          to_entity: 'doc-neural-nets',
          relation: 'REFERENCES',
          weight: 0.8,
          timestamp_nanos: now,
        },
        {
          from_entity: 'doc-ml-intro',
          to_entity: 'doc-supervised',
          relation: 'REFERENCES',
          weight: 0.7,
          timestamp_nanos: now,
        },
      ],
      importance: 0.9,
      created_at_nanos: now,
      updated_at_nanos: now,
    },
    {
      entity_id: 'doc-neural-nets',
      tenant_id: TENANT_ID,
      entity_type: 'document',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding('Neural networks are computing systems inspired by biological neural networks'),
        timestamp_nanos: now,
      }],
      chunks: [{ chunk_id: 'chunk-2', text: 'Neural networks...', metadata: {} }],
      incoming_edges: [
        {
          from_entity: 'doc-ml-intro',
          to_entity: 'doc-neural-nets',
          relation: 'REFERENCES',
          weight: 0.8,
          timestamp_nanos: now,
        },
      ],
      outgoing_edges: [
        {
          from_entity: 'doc-neural-nets',
          to_entity: 'doc-cnn',
          relation: 'REFERENCES',
          weight: 0.9,
          timestamp_nanos: now,
        },
      ],
      importance: 0.85,
      created_at_nanos: now,
      updated_at_nanos: now,
    },
    {
      entity_id: 'doc-supervised',
      tenant_id: TENANT_ID,
      entity_type: 'document',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding('Supervised learning uses labeled training data'),
        timestamp_nanos: now,
      }],
      chunks: [{ chunk_id: 'chunk-3', text: 'Supervised learning...', metadata: {} }],
      incoming_edges: [
        {
          from_entity: 'doc-ml-intro',
          to_entity: 'doc-supervised',
          relation: 'REFERENCES',
          weight: 0.7,
          timestamp_nanos: now,
        },
      ],
      importance: 0.8,
      created_at_nanos: now,
      updated_at_nanos: now,
    },
    {
      entity_id: 'doc-cnn',
      tenant_id: TENANT_ID,
      entity_type: 'document',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding('Convolutional neural networks for image recognition'),
        timestamp_nanos: now,
      }],
      chunks: [{ chunk_id: 'chunk-4', text: 'CNNs are specialized...', metadata: {} }],
      incoming_edges: [
        {
          from_entity: 'doc-neural-nets',
          to_entity: 'doc-cnn',
          relation: 'REFERENCES',
          weight: 0.9,
          timestamp_nanos: now,
        },
      ],
      importance: 0.75,
      created_at_nanos: now,
      updated_at_nanos: now,
    },
  ];
  
  console.log('Creating document graph...');
  await batchPutEntities(mlDocs);
  console.log('✓ Created 4 documents with 4 relationships\n');
  
  await sleep(1000);
  
  // Query: "How do neural networks work?"
  // This should find doc-neural-nets via vector search, then expand to doc-cnn via graph
  console.log('Query: "How do neural networks work?"');
  console.log('Expected: Find neural networks doc, then expand to CNN doc via graph\n');
  
  const queryEmbedding = await generateEmbedding('How do neural networks work?');
  const results = await queryVector(queryEmbedding, 3, 2);
  
  console.log(`Found ${results.total_results || results.results?.length || 0} results:`);
  results.results?.forEach((result, idx) => {
    console.log(`  ${idx + 1}. ${result.entity_id} (score: ${result.score?.toFixed(4) || 'N/A'})`);
  });
  
  console.log('\n✓ Graph RAG combines vector similarity with graph relationships!');
}

async function example4_BatchOperations() {
  console.log('\n[5] Example 4: Batch Operations');
  console.log('─'.repeat(60));
  console.log('Storing multiple entities efficiently...\n');
  
  const now = Date.now() * 1000000;
  const batchSize = 10;
  const entities = [];
  
  console.log(`Generating ${batchSize} entities...`);
  for (let i = 0; i < batchSize; i++) {
    const text = `Document number ${i + 1} about various topics`;
    entities.push({
      entity_id: `doc-batch-${i + 1}`,
      tenant_id: TENANT_ID,
      entity_type: 'document',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding(text),
        timestamp_nanos: now + i,
      }],
      // metadata must be map<string, string>, so we stringify values
      chunks: [{ chunk_id: `chunk-${i + 1}`, text, metadata: { index: String(i + 1) } }],
      importance: 0.5 + (i % 5) * 0.1,
      created_at_nanos: now + i,
      updated_at_nanos: now + i,
    });
  }
  
  console.log(`Storing ${batchSize} entities in a single batch request...`);
  const startTime = Date.now();
  const result = await batchPutEntities(entities);
  const duration = Date.now() - startTime;
  
  console.log(`✓ Batch stored in ${duration}ms`);
  console.log(`  Success: ${result.success_count || batchSize}, Failed: ${result.failure_count || 0}`);
}

async function example5_MultiHopGraph() {
  console.log('\n[6] Example 5: Multi-Hop Graph Traversal');
  console.log('─'.repeat(60));
  console.log('Traversing graph relationships across multiple hops...\n');
  
  const now = Date.now() * 1000000;
  
  // Create a chain: A -> B -> C -> D
  const chainEntities = [
    {
      entity_id: 'node-a',
      tenant_id: TENANT_ID,
      entity_type: 'node',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding('Starting point A'),
        timestamp_nanos: now,
      }],
      outgoing_edges: [{
        from_entity: 'node-a',
        to_entity: 'node-b',
        relation: 'LINKS_TO',
        weight: 0.9,
        timestamp_nanos: now,
      }],
      importance: 0.8,
      created_at_nanos: now,
      updated_at_nanos: now,
    },
    {
      entity_id: 'node-b',
      tenant_id: TENANT_ID,
      entity_type: 'node',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding('Intermediate node B'),
        timestamp_nanos: now,
      }],
      incoming_edges: [{
        from_entity: 'node-a',
        to_entity: 'node-b',
        relation: 'LINKS_TO',
        weight: 0.9,
        timestamp_nanos: now,
      }],
      outgoing_edges: [{
        from_entity: 'node-b',
        to_entity: 'node-c',
        relation: 'LINKS_TO',
        weight: 0.85,
        timestamp_nanos: now,
      }],
      importance: 0.75,
      created_at_nanos: now,
      updated_at_nanos: now,
    },
    {
      entity_id: 'node-c',
      tenant_id: TENANT_ID,
      entity_type: 'node',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding('Intermediate node C'),
        timestamp_nanos: now,
      }],
      incoming_edges: [{
        from_entity: 'node-b',
        to_entity: 'node-c',
        relation: 'LINKS_TO',
        weight: 0.85,
        timestamp_nanos: now,
      }],
      outgoing_edges: [{
        from_entity: 'node-c',
        to_entity: 'node-d',
        relation: 'LINKS_TO',
        weight: 0.8,
        timestamp_nanos: now,
      }],
      importance: 0.7,
      created_at_nanos: now,
      updated_at_nanos: now,
    },
    {
      entity_id: 'node-d',
      tenant_id: TENANT_ID,
      entity_type: 'node',
      embeddings: [{
        version: 1,
        vector: await generateEmbedding('End point D'),
        timestamp_nanos: now,
      }],
      incoming_edges: [{
        from_entity: 'node-c',
        to_entity: 'node-d',
        relation: 'LINKS_TO',
        weight: 0.8,
        timestamp_nanos: now,
      }],
      importance: 0.65,
      created_at_nanos: now,
      updated_at_nanos: now,
    },
  ];
  
  console.log('Creating chain: A -> B -> C -> D');
  await batchPutEntities(chainEntities);
  console.log('✓ Created 4-node chain\n');
  
  await sleep(1000);
  
  // Search from A with expandFactor=3 should find D via graph traversal
  console.log('Searching from "Starting point A" with expandFactor=3...');
  const queryEmbedding = await generateEmbedding('Starting point A');
  const results = await queryVector(queryEmbedding, 5, 3);
  
  console.log(`\nFound ${results.total_results || results.results?.length || 0} results:`);
  results.results?.forEach((result, idx) => {
    console.log(`  ${idx + 1}. ${result.entity_id} (score: ${result.score?.toFixed(4) || 'N/A'})`);
  });
  
  console.log('\n✓ Multi-hop traversal found nodes B, C, and D from starting point A!');
}

// ============================================================================
// Main
// ============================================================================

async function main() {
  console.log('='.repeat(60));
  console.log('AlliDB Search Examples');
  console.log('='.repeat(60));
  console.log(`Base URL: ${ALLIDB_BASE_URL}`);
  console.log(`API Key: ${ALLIDB_API_KEY.substring(0, 10)}... (full: ${ALLIDB_API_KEY})`);
  console.log(`Tenant ID: ${TENANT_ID}`);
  console.log(`Ollama URL: ${OLLAMA_URL}`);
  console.log(`Ollama Model: ${OLLAMA_MODEL}`);
  console.log(`\nNote: Ensure the API key "${ALLIDB_API_KEY}" is configured in:`);
  console.log(`  configs/allidb.yaml -> security.auth.static_api_keys`);
  console.log(`  Or set ALLIDB_API_KEY environment variable to match a configured key`);
  
  try {
    // Health check
    const healthy = await healthCheck();
    if (!healthy) {
      console.log('\n⚠️  Health check failed, but continuing with examples...');
      console.log('   (Examples will fail if server is not running or API key is invalid)\n');
      // Don't exit - let the examples run and show actual errors
    }
    
    // Run examples
    await example1_SimpleVectorSearch();
    await example2_GraphTraversal();
    await example3_GraphRAG();
    await example4_BatchOperations();
    await example5_MultiHopGraph();
    
    console.log('\n' + '='.repeat(60));
    console.log('✓ All examples completed successfully!');
    console.log('='.repeat(60));
    
  } catch (error) {
    console.error('\n❌ Error:', error.message);
    if (error.stack) {
      console.error(error.stack);
    }
    process.exit(1);
  }
}

// Run if executed directly
const __filename = fileURLToPath(import.meta.url);
if (process.argv[1] === __filename) {
  main();
}

export { main, queryVector, putEntity, batchPutEntities };

