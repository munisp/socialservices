# Middleware Deployment Guide

This document provides instructions for deploying and configuring all enterprise middleware components integrated into the Social Protection Platform.

## Overview

The platform integrates eight enterprise middleware components:

1. **Redis** - Caching and session management
2. **APISIX** - API gateway with rate limiting
3. **Kafka** - Event streaming and event sourcing
4. **Fluvio** - Real-time data streaming
5. **Keycloak** - Identity management and SSO
6. **Permify** - Fine-grained authorization
7. **Dapr** - Microservices runtime
8. **Temporal** - Workflow orchestration

## Quick Start

### 1. Start All Middleware Services

```bash
# Start all middleware containers
docker-compose -f docker-compose.middleware.yml up -d

# Check status
docker-compose -f docker-compose.middleware.yml ps
```

### 2. Configure Environment Variables

Create a `.env.middleware` file:

```bash
# Redis
REDIS_URL=redis://:redis123@localhost:6379
REDIS_PASSWORD=redis123

# APISIX
APISIX_ADMIN_KEY=your-admin-key-here

# Kafka
KAFKA_BROKERS=localhost:9092
KAFKA_CLIENT_ID=admin-portal

# Fluvio
FLUVIO_CLUSTER=localhost:9003

# Keycloak
KEYCLOAK_URL=http://localhost:8081
KEYCLOAK_REALM=social-protection
KEYCLOAK_CLIENT_ID=admin-portal
KEYCLOAK_CLIENT_SECRET=your-client-secret
KEYCLOAK_ADMIN_PASSWORD=admin123
KEYCLOAK_DB_PASSWORD=keycloak123

# Permify
PERMIFY_URL=http://localhost:3476
PERMIFY_DB_PASSWORD=permify123

# Dapr
DAPR_HOST=localhost
DAPR_HTTP_PORT=3500
DAPR_GRPC_PORT=50001
DAPR_SERVER_HOST=127.0.0.1
DAPR_SERVER_PORT=3001

# Temporal
TEMPORAL_ADDRESS=localhost:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_DB_PASSWORD=temporal123
```

### 3. Initialize Services

```bash
# Initialize Keycloak realm
npm run middleware:init-keycloak

# Initialize Permify schema
npm run middleware:init-permify

# Create Kafka topics
npm run middleware:init-kafka

# Test all connections
npm run middleware:health-check
```

## Component Details

### Redis (Port 6379)

**Purpose:** Caching frequently accessed data and session storage

**Configuration:**
- Password: Set via `REDIS_PASSWORD`
- Persistence: AOF enabled
- Max memory: 256MB (configurable)

**Usage Examples:**

```typescript
import { withCache, invalidateCache } from "./server/middleware/redis";

// Cache database query
const beneficiaries = await withCache(
  "beneficiaries:active",
  async () => db.getActiveBeneficiaries(),
  300 // 5 minutes TTL
);

// Invalidate cache
await invalidateCache("beneficiaries:*");
```

**Monitoring:**
- Redis CLI: `docker exec -it admin-portal-redis redis-cli`
- Monitor commands: `MONITOR`, `INFO`, `CLIENT LIST`

---

### APISIX (Ports 9080, 9180, 9443)

**Purpose:** API gateway with rate limiting, load balancing, and analytics

**Admin Dashboard:** http://localhost:9000

**Configuration:**
- Gateway HTTP: Port 9080
- Admin API: Port 9180
- Gateway HTTPS: Port 9443

**Features:**
- Rate limiting: 1000 req/min per IP
- Load balancing: Round-robin
- Authentication: Consumer-based
- Logging: HTTP logger plugin

**Usage Examples:**

```typescript
import { ApisixClient } from "./server/middleware/apisix";

const client = new ApisixClient();

// Create route
await client.createRoute({
  uri: "/api/beneficiaries/*",
  upstream_id: "beneficiary-service",
  plugins: {
    "limit-req": { rate: 100, burst: 50 }
  }
});

// Create consumer
await client.createConsumer("user-123", "api-key-secret");
```

**Monitoring:**
- Dashboard: http://localhost:9000
- Prometheus metrics: http://localhost:9080/apisix/prometheus/metrics

---

### Kafka (Port 9092)

**Purpose:** Event streaming, event sourcing, and async processing

**Kafka UI:** http://localhost:8080

**Topics:**
- `beneficiary.created`, `beneficiary.updated`, `beneficiary.deleted`
- `transaction.completed`, `transaction.failed`
- `disbursement.scheduled`, `disbursement.processed`, `disbursement.failed`
- `fraud.detected`
- `kyc.verified`, `kyc.rejected`
- `program.created`, `program.updated`
- `approval.requested`, `approval.completed`

**Usage Examples:**

```typescript
import { publishEvent, TOPICS } from "./server/middleware/kafka";

// Publish event
await publishEvent(TOPICS.BENEFICIARY_CREATED, beneficiary, {
  userId: currentUser.id,
  metadata: { source: "admin-portal" }
});
```

**Monitoring:**
- Kafka UI: http://localhost:8080
- Consumer lag: Check in Kafka UI
- Topic metrics: Partition count, message rate

---

### Fluvio (Port 9003)

**Purpose:** Real-time data streaming for analytics and live dashboards

**Streams:**
- `transactions-stream` - Real-time transaction feed
- `fraud-alerts-stream` - Fraud detection alerts
- `system-metrics-stream` - Performance metrics
- `user-activity-stream` - User actions
- `analytics-events-stream` - Analytics events

**Usage Examples:**

```typescript
import { streamTransaction, streamFraudAlert } from "./server/middleware/fluvio";

// Stream transaction
await streamTransaction(transaction);

// Stream fraud alert
await streamFraudAlert(fraudAlert);
```

**Monitoring:**
- Fluvio CLI: `fluvio topic list`
- Stream metrics: `fluvio topic describe <topic>`

---

### Keycloak (Port 8081)

**Purpose:** Enterprise identity management and SSO

**Admin Console:** http://localhost:8081/admin
- Username: `admin`
- Password: Set via `KEYCLOAK_ADMIN_PASSWORD`

**Realm:** `social-protection`

**Features:**
- OAuth2/OIDC authentication
- User federation
- Role mapping
- Multi-factor authentication
- Session management

**Usage Examples:**

```typescript
import { getKeycloakLoginUrl, verifyToken } from "./server/middleware/keycloak";

// Get login URL
const loginUrl = getKeycloakLoginUrl("http://localhost:3000/callback");

// Verify token
const user = await verifyToken(accessToken);
console.log(user.roles); // ['admin', 'beneficiary-manager']
```

**Configuration:**
1. Create realm: `social-protection`
2. Create client: `admin-portal`
3. Configure redirect URIs
4. Set up roles: `admin`, `manager`, `viewer`
5. Enable user registration (optional)

---

### Permify (Ports 3476, 3478)

**Purpose:** Fine-grained, relationship-based authorization

**HTTP API:** http://localhost:3476
**gRPC API:** localhost:3478

**Authorization Model:**
- Entities: `user`, `beneficiary`, `program`, `transaction`, `document`, `organization`
- Relations: `owner`, `editor`, `viewer`, `admin`, `manager`, `member`
- Actions: `view`, `edit`, `delete`, `approve`, `manage`, `configure`

**Usage Examples:**

```typescript
import { checkPermission, grantBeneficiaryAccess } from "./server/middleware/permify";

// Grant access
await grantBeneficiaryAccess(beneficiaryId, userId, "editor");

// Check permission
const canEdit = await checkPermission({
  entity: "beneficiary",
  entityId: beneficiaryId,
  action: "edit",
  userId: currentUser.id
});
```

**Schema Management:**
- Schema is auto-initialized on startup
- View schema: `permify schema read`
- Update schema: Modify `PERMIFY_SCHEMA` in `server/middleware/permify.ts`

---

### Dapr (Ports 3500, 50001)

**Purpose:** Portable microservices runtime

**HTTP Port:** 3500
**gRPC Port:** 50001

**Features:**
- Service-to-service invocation
- Pub/sub messaging
- State management
- Secret management
- Bindings

**Usage Examples:**

```typescript
import { invokeService, publishEvent, saveState } from "./server/middleware/dapr";

// Service invocation
const result = await invokeService("payment-service", "processPayment", {
  amount: 1000,
  beneficiaryId: "123"
});

// Pub/sub
await publishEvent("pubsub", "beneficiary-events", beneficiaryData);

// State management
await saveState("statestore", "beneficiary:123", beneficiaryData);
```

**Configuration:**
- Components: Define in `.dapr/components/`
- State store: Redis
- Pub/sub: Kafka
- Secrets: Local file or Kubernetes

---

### Temporal (Port 7233)

**Purpose:** Durable workflow orchestration

**Temporal UI:** http://localhost:8082
**gRPC Endpoint:** localhost:7233

**Workflows:**
- `disbursementApproval` - Multi-level approval workflow
- `kycVerification` - KYC document verification
- `fraudInvestigation` - Fraud alert investigation
- `cardIssuance` - Benefit card issuance
- `monthlyDisbursement` - Scheduled monthly disbursements

**Usage Examples:**

```typescript
import { 
  startDisbursementApprovalWorkflow,
  approveDisbursement,
  getDisbursementApprovalStatus
} from "./server/middleware/temporal";

// Start workflow
const workflowId = await startDisbursementApprovalWorkflow(
  disbursementId,
  amount,
  beneficiaryId
);

// Query status
const status = await getDisbursementApprovalStatus(disbursementId);

// Signal approval
await approveDisbursement(disbursementId, approverId);
```

**Monitoring:**
- Temporal UI: http://localhost:8082
- Workflow history: View in UI
- Task queues: Monitor backlog

---

## Health Checks

Run health checks for all middleware:

```bash
npm run middleware:health-check
```

Or check individually:

```typescript
import {
  redisHealthCheck,
  apisixHealthCheck,
  keycloakHealthCheck,
  permifyHealthCheck,
  daprHealthCheck,
  temporalHealthCheck,
  fluvioHealthCheck
} from "./server/middleware/*";

const health = {
  redis: await redisHealthCheck(),
  apisix: await apisixHealthCheck(),
  keycloak: await keycloakHealthCheck(),
  permify: await permifyHealthCheck(),
  dapr: await daprHealthCheck(),
  temporal: await temporalHealthCheck(),
  fluvio: await fluvioHealthCheck()
};
```

## Production Deployment

### Security Checklist

- [ ] Change all default passwords
- [ ] Enable TLS/SSL for all services
- [ ] Configure firewall rules
- [ ] Set up authentication for admin interfaces
- [ ] Enable audit logging
- [ ] Configure backup strategies
- [ ] Set resource limits (CPU, memory)
- [ ] Enable monitoring and alerting

### Scaling Considerations

**Redis:**
- Use Redis Cluster for high availability
- Configure sentinel for automatic failover
- Set appropriate maxmemory-policy

**Kafka:**
- Increase partition count for high throughput topics
- Configure replication factor ≥ 3
- Monitor consumer lag

**APISIX:**
- Deploy multiple APISIX nodes behind load balancer
- Use etcd cluster for configuration
- Enable caching plugins

**Keycloak:**
- Use external PostgreSQL database
- Configure clustering for HA
- Enable database connection pooling

**Permify:**
- Use PostgreSQL for production
- Configure read replicas for scaling
- Enable caching

**Temporal:**
- Use external PostgreSQL/MySQL
- Configure multiple worker pools
- Set appropriate workflow timeouts

## Troubleshooting

### Redis Connection Issues
```bash
# Test connection
redis-cli -h localhost -p 6379 -a redis123 PING

# Check logs
docker logs admin-portal-redis
```

### Kafka Producer/Consumer Issues
```bash
# List topics
docker exec admin-portal-kafka kafka-topics.sh --bootstrap-server localhost:9092 --list

# Check consumer groups
docker exec admin-portal-kafka kafka-consumer-groups.sh --bootstrap-server localhost:9092 --list
```

### Keycloak Login Issues
```bash
# Check realm configuration
curl http://localhost:8081/realms/social-protection

# View logs
docker logs admin-portal-keycloak
```

### Temporal Workflow Stuck
```bash
# Check workflow status in UI
open http://localhost:8082

# View worker logs
docker logs admin-portal-temporal
```

## Monitoring & Observability

### Metrics Collection

All middleware components expose metrics:

- **Redis:** `INFO` command
- **APISIX:** Prometheus endpoint
- **Kafka:** JMX metrics
- **Keycloak:** Metrics endpoint
- **Temporal:** Prometheus metrics

### Centralized Logging

Configure log aggregation:

```yaml
# docker-compose.middleware.yml
services:
  loki:
    image: grafana/loki:latest
    ports:
      - "3100:3100"
  
  promtail:
    image: grafana/promtail:latest
    volumes:
      - /var/log:/var/log
```

### Alerting Rules

Set up alerts for:
- Redis memory usage > 80%
- Kafka consumer lag > 1000
- APISIX error rate > 5%
- Keycloak login failures
- Temporal workflow failures
- Permify authorization errors

## Backup & Recovery

### Redis Backup
```bash
# Create snapshot
docker exec admin-portal-redis redis-cli BGSAVE

# Restore from snapshot
docker cp dump.rdb admin-portal-redis:/data/
```

### Kafka Backup
```bash
# Use MirrorMaker for replication
# Or backup topic data with Kafka Connect
```

### Database Backups
```bash
# PostgreSQL backup
docker exec admin-portal-keycloak-db pg_dump -U keycloak keycloak > keycloak-backup.sql
docker exec admin-portal-permify-db pg_dump -U permify permify > permify-backup.sql
docker exec admin-portal-temporal-db pg_dump -U temporal temporal > temporal-backup.sql
```

## Support & Resources

- **Redis:** https://redis.io/docs
- **APISIX:** https://apisix.apache.org/docs
- **Kafka:** https://kafka.apache.org/documentation
- **Fluvio:** https://fluvio.io/docs
- **Keycloak:** https://www.keycloak.org/documentation
- **Permify:** https://docs.permify.co
- **Dapr:** https://docs.dapr.io
- **Temporal:** https://docs.temporal.io
