# Social Protection Platform - Operations Runbook

This document consolidates all operational procedures for the Social Protection Platform.

## 1. Platform Overview

The Social Protection Platform is a comprehensive system for managing social welfare programs, beneficiary enrollment, disbursements, and compliance. It consists of the following core components:

- **Admin Portal**: React/TypeScript frontend for administrative operations
- **API Gateway**: APISIX for routing, rate limiting, and authentication
- **Orchestrator**: Go-based workflow orchestration with Temporal
- **Authentication**: Keycloak for identity management
- **Authorization**: Permify for fine-grained access control
- **Message Queue**: Kafka for event streaming
- **Cache**: Redis with Sentinel for high availability
- **Financial Ledger**: TigerBeetle for transaction processing
- **Data Lakehouse**: Flink, Spark, and Delta Lake for analytics
- **Stream Processing**: Fluvio for real-time data processing
- **Security**: OpenAppSec WAF for application protection

## 2. Deployment

### 2.1 Prerequisites

- Kubernetes cluster (v1.29+)
- kubectl configured with cluster access
- Helm 3.x installed
- Storage class configured for persistent volumes

### 2.2 Namespace Setup

```bash
kubectl apply -f k8s/base/namespace.yaml
```

### 2.3 Deploy Infrastructure Services

Deploy services in order:

```bash
# 1. Deploy Kafka
kubectl apply -f k8s/kafka/kafka-ha.yaml

# 2. Deploy Redis
kubectl apply -f k8s/redis/redis-ha.yaml

# 3. Deploy APISIX with etcd
kubectl apply -f k8s/apisix/apisix-ha.yaml

# 4. Deploy Temporal
kubectl apply -f k8s/temporal/temporal-ha.yaml

# 5. Deploy Keycloak
kubectl apply -f k8s/keycloak/keycloak-ha.yaml

# 6. Deploy Permify
kubectl apply -f k8s/permify/permify-ha.yaml

# 7. Deploy Dapr
kubectl apply -f k8s/dapr/dapr-ha.yaml

# 8. Deploy Fluvio
kubectl apply -f k8s/fluvio/fluvio-ha.yaml

# 9. Deploy TigerBeetle
kubectl apply -f k8s/tigerbeetle/tigerbeetle-ha.yaml

# 10. Deploy Lakehouse
kubectl apply -f k8s/lakehouse/lakehouse-ha.yaml

# 11. Deploy OpenAppSec
kubectl apply -f k8s/openappsec/openappsec-ha.yaml
```

### 2.4 Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n social-protection

# Check services
kubectl get svc -n social-protection

# Check PVCs
kubectl get pvc -n social-protection
```

## 3. Service Configuration

### 3.1 Kafka Configuration

| Parameter | Value | Description |
|-----------|-------|-------------|
| Replicas | 3 | Number of broker nodes |
| Replication Factor | 3 | Data replication across brokers |
| Min ISR | 2 | Minimum in-sync replicas |
| Retention | 7 days | Message retention period |

Key topics:
- `enrollments` - Enrollment events
- `disbursements` - Disbursement events
- `notifications` - Notification events
- `audit-logs` - Audit trail events

### 3.2 Redis Configuration

| Parameter | Value | Description |
|-----------|-------|-------------|
| Replicas | 3 | Master + 2 replicas |
| Sentinels | 3 | Sentinel instances for HA |
| Max Memory | 4GB | Per-node memory limit |
| Eviction Policy | allkeys-lru | LRU eviction when full |

### 3.3 Temporal Configuration

| Component | Replicas | Description |
|-----------|----------|-------------|
| Frontend | 3 | API gateway |
| History | 3 | Workflow history |
| Matching | 3 | Task matching |
| Worker | 3 | Workflow workers |

### 3.4 Keycloak Configuration

| Parameter | Value | Description |
|-----------|-------|-------------|
| Replicas | 3 | Clustered instances |
| Realm | social-protection | Main realm |
| Clients | admin-portal, api | OAuth clients |

## 4. Monitoring

### 4.1 Health Checks

All services expose health endpoints:

| Service | Health Endpoint |
|---------|-----------------|
| APISIX | `GET /health` |
| Keycloak | `GET /health/ready` |
| Temporal | `GET /health` |
| Kafka | TCP port 9092 |
| Redis | `PING` command |

### 4.2 Metrics

Prometheus metrics are exposed on port 9090 for each service. Key metrics to monitor:

- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request latency
- `kafka_consumer_lag` - Consumer lag
- `redis_connected_clients` - Redis connections
- `temporal_workflow_completed_total` - Completed workflows

### 4.3 Logging

Logs are collected via Fluentd and stored in the Lakehouse. Log levels:

| Level | Usage |
|-------|-------|
| ERROR | System errors requiring attention |
| WARN | Potential issues |
| INFO | Normal operations |
| DEBUG | Detailed debugging (disabled in prod) |

## 5. Common Operations

### 5.1 Scaling Services

```bash
# Scale API pods
kubectl scale deployment admin-portal -n social-protection --replicas=5

# Scale Temporal workers
kubectl scale deployment temporal-worker -n social-protection --replicas=5
```

### 5.2 Database Backup

```bash
# Backup Postgres (Keycloak, Temporal, Permify)
kubectl exec -n social-protection postgres-0 -- pg_dump -U postgres keycloak > keycloak_backup.sql
kubectl exec -n social-protection postgres-0 -- pg_dump -U postgres temporal > temporal_backup.sql
kubectl exec -n social-protection postgres-0 -- pg_dump -U postgres permify > permify_backup.sql
```

### 5.3 Kafka Topic Management

```bash
# List topics
kubectl exec -n social-protection kafka-0 -- kafka-topics.sh --list --bootstrap-server localhost:9092

# Create topic
kubectl exec -n social-protection kafka-0 -- kafka-topics.sh --create --topic new-topic --partitions 3 --replication-factor 3 --bootstrap-server localhost:9092

# Describe topic
kubectl exec -n social-protection kafka-0 -- kafka-topics.sh --describe --topic enrollments --bootstrap-server localhost:9092
```

### 5.4 Redis Operations

```bash
# Connect to Redis
kubectl exec -it -n social-protection redis-0 -- redis-cli

# Check replication status
kubectl exec -n social-protection redis-0 -- redis-cli INFO replication

# Flush cache (use with caution)
kubectl exec -n social-protection redis-0 -- redis-cli FLUSHALL
```

### 5.5 Temporal Operations

```bash
# List workflows
kubectl exec -n social-protection temporal-frontend-0 -- tctl workflow list

# Describe workflow
kubectl exec -n social-protection temporal-frontend-0 -- tctl workflow describe -w <workflow-id>

# Terminate workflow
kubectl exec -n social-protection temporal-frontend-0 -- tctl workflow terminate -w <workflow-id>
```

## 6. Troubleshooting

### 6.1 Service Not Starting

1. Check pod status: `kubectl describe pod <pod-name> -n social-protection`
2. Check logs: `kubectl logs <pod-name> -n social-protection`
3. Check events: `kubectl get events -n social-protection --sort-by='.lastTimestamp'`

### 6.2 High Latency

1. Check resource usage: `kubectl top pods -n social-protection`
2. Check Kafka consumer lag
3. Check Redis memory usage
4. Review slow query logs

### 6.3 Authentication Failures

1. Verify Keycloak is running: `kubectl get pods -n social-protection -l app=keycloak`
2. Check Keycloak logs for errors
3. Verify client configuration in Keycloak admin console
4. Check token expiration settings

### 6.4 Workflow Failures

1. Check Temporal UI for failed workflows
2. Review workflow history for error details
3. Check worker logs for exceptions
4. Verify activity timeouts are appropriate

### 6.5 Data Inconsistency

1. Check TigerBeetle transaction logs
2. Verify Kafka message delivery
3. Check for duplicate processing
4. Review idempotency keys

## 7. Disaster Recovery

### 7.1 Backup Strategy

| Component | Backup Frequency | Retention |
|-----------|------------------|-----------|
| Postgres | Daily | 30 days |
| Kafka | Continuous (replication) | 7 days |
| Redis | Hourly RDB | 24 hours |
| TigerBeetle | Continuous (replication) | Indefinite |
| Lakehouse | Daily | 90 days |

### 7.2 Recovery Procedures

#### Database Recovery

```bash
# Restore Postgres from backup
kubectl exec -i -n social-protection postgres-0 -- psql -U postgres keycloak < keycloak_backup.sql
```

#### Kafka Recovery

Kafka data is replicated across 3 brokers. If a broker fails:
1. The cluster continues operating with remaining brokers
2. Replace failed broker and let it sync from replicas

#### Redis Recovery

Redis Sentinel automatically handles failover:
1. Sentinel detects master failure
2. Promotes replica to master
3. Reconfigures other replicas

### 7.3 RTO/RPO Targets

| Component | RTO | RPO |
|-----------|-----|-----|
| API Gateway | 5 min | 0 |
| Database | 30 min | 1 hour |
| Message Queue | 10 min | 0 |
| Cache | 5 min | 1 hour |

## 8. Security Operations

### 8.1 Certificate Rotation

```bash
# Rotate TLS certificates
kubectl create secret tls social-protection-tls \
  --cert=new-cert.pem \
  --key=new-key.pem \
  -n social-protection \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart affected pods
kubectl rollout restart deployment -n social-protection
```

### 8.2 Secret Management

Secrets are stored in Kubernetes secrets. To update:

```bash
# Update secret
kubectl create secret generic app-secrets \
  --from-literal=DB_PASSWORD=new-password \
  -n social-protection \
  --dry-run=client -o yaml | kubectl apply -f -
```

### 8.3 Access Control

- All API access requires valid JWT token from Keycloak
- Permissions are enforced by Permify
- Audit logs capture all access attempts

### 8.4 Security Scanning

- OpenAppSec WAF protects against OWASP Top 10
- Regular vulnerability scans of container images
- Penetration testing quarterly

## 9. Performance Tuning

### 9.1 JVM Services (Kafka, Keycloak)

```yaml
env:
  - name: JAVA_OPTS
    value: "-Xms2g -Xmx4g -XX:+UseG1GC"
```

### 9.2 Database Tuning

Key Postgres parameters:
- `shared_buffers`: 25% of RAM
- `effective_cache_size`: 75% of RAM
- `work_mem`: 64MB
- `maintenance_work_mem`: 512MB

### 9.3 Connection Pooling

- Use PgBouncer for Postgres connection pooling
- Configure Redis connection pool size based on load

## 10. Maintenance Windows

### 10.1 Scheduled Maintenance

- **Weekly**: Sunday 02:00-04:00 UTC - Minor updates
- **Monthly**: First Sunday 02:00-06:00 UTC - Major updates
- **Quarterly**: Security patches and upgrades

### 10.2 Rolling Updates

```bash
# Update deployment with zero downtime
kubectl set image deployment/admin-portal admin-portal=new-image:tag -n social-protection
kubectl rollout status deployment/admin-portal -n social-protection
```

### 10.3 Database Migrations

```bash
# Run migrations
kubectl exec -n social-protection admin-portal-0 -- npm run migrate

# Rollback if needed
kubectl exec -n social-protection admin-portal-0 -- npm run migrate:rollback
```

## 11. Contact Information

| Role | Contact |
|------|---------|
| Platform Team | platform-team@social-protection.gov |
| Security Team | security@social-protection.gov |
| On-Call | +1-XXX-XXX-XXXX |

## 12. Appendix

### 12.1 Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `NODE_ENV` | Environment | production |
| `DATABASE_URL` | Postgres connection | - |
| `REDIS_URL` | Redis connection | - |
| `KAFKA_BROKERS` | Kafka bootstrap servers | - |
| `KEYCLOAK_URL` | Keycloak base URL | - |
| `TEMPORAL_HOST` | Temporal frontend address | - |

### 12.2 Port Reference

| Service | Port | Protocol |
|---------|------|----------|
| Admin Portal | 3000 | HTTP |
| APISIX | 9080 | HTTP |
| APISIX Admin | 9180 | HTTP |
| Keycloak | 8080 | HTTP |
| Temporal Frontend | 7233 | gRPC |
| Temporal UI | 8080 | HTTP |
| Kafka | 9092 | TCP |
| Redis | 6379 | TCP |
| Postgres | 5432 | TCP |
| TigerBeetle | 3000 | TCP |

### 12.3 Useful Commands

```bash
# Get all resources in namespace
kubectl get all -n social-protection

# Watch pod status
kubectl get pods -n social-protection -w

# Port forward for local access
kubectl port-forward svc/admin-portal 3000:3000 -n social-protection

# Get pod resource usage
kubectl top pods -n social-protection

# Execute command in pod
kubectl exec -it <pod-name> -n social-protection -- /bin/sh
```
