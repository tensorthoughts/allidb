# Docker Setup for AlliDB

This directory contains Docker configuration files for running AlliDB locally.

## Quick Start

### Single Node

Build and run a single AlliDB node:

```bash
# Build the Docker image
docker build -t allidb:latest .

# Run a single node
docker run -d \
  --name allidb-node1 \
  -p 50051:50051 \
  -p 8080:8080 \
  -v allidb-data:/data \
  -e ALLIDB_NODE_ID=node1 \
  -e ALLIDB_GRPC_ADDR=0.0.0.0:50051 \
  -e ALLIDB_HTTP_ADDR=0.0.0.0:8080 \
  allidb:latest
```

### Multi-Node Cluster

Use Docker Compose to run a 3-node cluster:

```bash
# Navigate to docker directory
cd docker

# Start all nodes
docker-compose up -d

# View logs
docker-compose logs -f

# Stop all nodes
docker-compose down

# Stop and remove volumes (WARNING: deletes all data)
docker-compose down -v
```

## Configuration

### Environment Variables

- `ALLIDB_NODE_ID`: Unique node identifier (default: `node1`)
- `ALLIDB_GRPC_ADDR`: gRPC server address (default: `0.0.0.0:50051`)
- `ALLIDB_HTTP_ADDR`: HTTP server address (default: `0.0.0.0:8080`)
- `ALLIDB_DATA_DIR`: Data directory (default: `/data`)
- `ALLIDB_WAL_DIR`: WAL directory (default: `/data/wal`)
- `ALLIDB_SSTABLE_DIR`: SSTable directory (default: `/data/sstables`)
- `ALLIDB_LOG_LEVEL`: Log level - DEBUG, INFO, WARN, ERROR (default: `INFO`)
- `ALLIDB_CLUSTER_NODES`: Comma-separated list of cluster nodes (e.g., `node1:50051,node2:50051,node3:50051`)

### Ports

- **50051**: gRPC API
- **8080**: HTTP/REST API
- **7946**: Gossip protocol (internal)

### Volumes

Data is persisted in Docker volumes:
- `allidb-node1-data`: Node 1 data
- `allidb-node2-data`: Node 2 data
- `allidb-node3-data`: Node 3 data

## API Access

### HTTP/REST API

Default API key for testing: `test-api-key`

```bash
# Health check
curl http://localhost:8080/health

# Put entity
curl -X POST http://localhost:8080/entity \
  -H "X-ALLIDB-API-KEY: test-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "entity": {
      "entity_id": "entity-1",
      "entity_type": "Document",
      "embeddings": [{
        "version": 1,
        "vector": [0.1, 0.2, 0.3],
        "timestamp_nanos": 1234567890
      }]
    },
    "consistency_level": "CONSISTENCY_QUORUM"
  }'

# Query
curl -X POST http://localhost:8080/query \
  -H "X-ALLIDB-API-KEY: test-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3],
    "k": 10,
    "expand_factor": 2
  }'
```

### gRPC API

```bash
# Using grpcurl (install: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest)
grpcurl -plaintext -H "X-ALLIDB-API-KEY: test-api-key" \
  localhost:50051 allidb.AlliDB/HealthCheck
```

## Multi-Node Setup

The `docker-compose.yml` file sets up a 3-node cluster:

- **Node 1**: gRPC on 50051, HTTP on 8080
- **Node 2**: gRPC on 50052, HTTP on 8081
- **Node 3**: gRPC on 50053, HTTP on 8082

Each node has its own data volume and can communicate via the `allidb-network` bridge network.

## Development

### Building from Source

```bash
# Build with grpc tag
docker build -t allidb:dev --build-arg BUILD_TAGS=grpc .
```

### Running Tests

```bash
# Run tests in container
docker run --rm allidb:latest go test ./...
```

### Debugging

```bash
# Access container shell
docker exec -it allidb-node1 sh

# View logs
docker logs -f allidb-node1

# Check health
docker exec allidb-node1 wget -q -O- http://localhost:8080/health
```

## Production Considerations

For production deployments:

1. **TLS/mTLS**: Configure TLS certificates (see `core/security/tls/`)
2. **API Keys**: Use proper API key management (not hardcoded test keys)
3. **Network**: Use overlay networks for multi-host deployments
4. **Storage**: Use persistent volumes or external storage
5. **Monitoring**: Integrate with observability stack (metrics, tracing, logging)
6. **Resource Limits**: Set appropriate CPU/memory limits
7. **Health Checks**: Configure proper health check endpoints
8. **Backup**: Implement backup strategy for data volumes

## Troubleshooting

### Container won't start

Check logs:
```bash
docker logs allidb-node1
```

### Port conflicts

Modify port mappings in `docker-compose.yml`:
```yaml
ports:
  - "50052:50051"  # Host:Container
```

### Data persistence

Volumes are created automatically. To backup:
```bash
docker run --rm -v allidb-node1-data:/data -v $(pwd):/backup \
  alpine tar czf /backup/allidb-backup.tar.gz /data
```

### Network issues

Check network connectivity:
```bash
docker network inspect allidb-network
```

