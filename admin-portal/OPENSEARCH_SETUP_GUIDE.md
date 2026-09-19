# OpenSearch Setup Guide for Admin Portal

This guide provides comprehensive instructions for setting up OpenSearch integration with the Social Protection Platform Admin Portal to enable full-text search capabilities across all entities.

---

## Overview

The Admin Portal includes built-in OpenSearch integration that provides powerful full-text search across benefit programs, users, MCC codes, and audit logs. When OpenSearch is configured, administrators can use the global search feature (Cmd/Ctrl+K) to quickly find information across the entire platform.

**Key Features:**
- Full-text search with fuzzy matching
- Search across multiple entity types simultaneously
- Highlighted search results showing matched terms
- Date range filtering for time-based queries
- Saved search filters for frequently-used queries

**Optional Integration:** OpenSearch is completely optional. If not configured, the Admin Portal will function normally but the global search feature will be disabled. All other features remain fully operational.

---

## Architecture

The OpenSearch integration follows a **lazy initialization** pattern that gracefully handles missing configuration:

1. **Connection Check:** On startup, the system checks for the `OPENSEARCH_NODE` environment variable
2. **Mock Mode:** If not configured, the system logs a message and continues in mock mode
3. **Graceful Degradation:** All OpenSearch-dependent features check for client availability before executing
4. **No Errors:** Missing OpenSearch configuration never causes errors or prevents application startup

This design ensures the Admin Portal can be deployed and used immediately without requiring OpenSearch infrastructure.

---

## Prerequisites

Before setting up OpenSearch, ensure you have:

- **OpenSearch Cluster:** Version 2.0 or later (self-hosted or managed service)
- **Network Access:** Admin Portal must be able to reach the OpenSearch cluster
- **Authentication:** Username and password for OpenSearch cluster (if authentication is enabled)
- **TLS/SSL:** Certificate configuration (for production deployments)
- **Resources:** Adequate disk space and memory for your expected data volume

**Minimum Requirements:**
- OpenSearch 2.0+
- 2 GB RAM (development)
- 8 GB RAM (production)
- 20 GB disk space

---

## Deployment Options

### Option 1: Docker Compose (Development)

For local development and testing, use Docker Compose to run OpenSearch.

**Step 1: Create docker-compose.yml**

```yaml
version: '3.8'

services:
  opensearch:
    image: opensearchproject/opensearch:latest
    container_name: opensearch-dev
    environment:
      - discovery.type=single-node
      - OPENSEARCH_INITIAL_ADMIN_PASSWORD=Admin123!
      - DISABLE_SECURITY_PLUGIN=false
      - "OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m"
    ports:
      - "9200:9200"
      - "9600:9600"
    volumes:
      - opensearch-data:/usr/share/opensearch/data
    networks:
      - admin-portal-network

  opensearch-dashboards:
    image: opensearchproject/opensearch-dashboards:latest
    container_name: opensearch-dashboards
    ports:
      - "5601:5601"
    environment:
      - OPENSEARCH_HOSTS=["https://opensearch:9200"]
      - OPENSEARCH_USERNAME=admin
      - OPENSEARCH_PASSWORD=Admin123!
    networks:
      - admin-portal-network
    depends_on:
      - opensearch

volumes:
  opensearch-data:

networks:
  admin-portal-network:
    driver: bridge
```

**Step 2: Start OpenSearch**

```bash
docker-compose up -d
```

**Step 3: Verify Installation**

```bash
# Check cluster health
curl -k -u admin:Admin123! https://localhost:9200/_cluster/health?pretty

# Expected output:
# {
#   "cluster_name" : "docker-cluster",
#   "status" : "green",
#   ...
# }
```

**Step 4: Configure Admin Portal**

Add to your `.env` file or environment variables:

```bash
OPENSEARCH_NODE=https://localhost:9200
OPENSEARCH_USERNAME=admin
OPENSEARCH_PASSWORD=Admin123!
```

### Option 2: AWS OpenSearch Service (Production)

For production deployments, use AWS OpenSearch Service (formerly Amazon Elasticsearch Service).

**Step 1: Create OpenSearch Domain**

1. Open AWS Console → OpenSearch Service
2. Click "Create domain"
3. Configure:
   - **Domain name:** `admin-portal-search`
   - **Version:** OpenSearch 2.11 (latest)
   - **Instance type:** t3.small.search (development) or m6g.large.search (production)
   - **Number of nodes:** 2 (for high availability)
   - **Storage:** 20 GB EBS (gp3)
   - **Network:** VPC access (recommended) or Public access
   - **Fine-grained access control:** Enabled
   - **Master user:** Create master user with strong password

4. Click "Create"

**Step 2: Configure Security**

1. Navigate to domain → Security configuration
2. Add access policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "*"
      },
      "Action": "es:*",
      "Resource": "arn:aws:es:region:account-id:domain/admin-portal-search/*",
      "Condition": {
        "IpAddress": {
          "aws:SourceIp": ["YOUR_ADMIN_PORTAL_IP/32"]
        }
      }
    }
  ]
}
```

**Step 3: Get Endpoint**

1. Navigate to domain → General information
2. Copy the **Domain endpoint** (e.g., `https://search-admin-portal-search-xxx.region.es.amazonaws.com`)

**Step 4: Configure Admin Portal**

Add to your environment variables (use Secrets Manager or Parameter Store):

```bash
OPENSEARCH_NODE=https://search-admin-portal-search-xxx.region.es.amazonaws.com
OPENSEARCH_USERNAME=master-username
OPENSEARCH_PASSWORD=master-password
```

### Option 3: Self-Hosted Production

For self-hosted production deployments on Linux servers.

**Step 1: Install OpenSearch**

```bash
# Add OpenSearch repository
curl -o- https://artifacts.opensearch.org/publickeys/opensearch.pgp | sudo gpg --dearmor --batch --yes -o /usr/share/keyrings/opensearch-keyring

echo "deb [signed-by=/usr/share/keyrings/opensearch-keyring] https://artifacts.opensearch.org/releases/bundle/opensearch/2.x/apt stable main" | sudo tee /etc/apt/sources.list.d/opensearch-2.x.list

# Update and install
sudo apt-get update
sudo apt-get install opensearch

# Configure
sudo nano /etc/opensearch/opensearch.yml
```

**Step 2: Configure opensearch.yml**

```yaml
cluster.name: admin-portal-cluster
node.name: node-1
network.host: 0.0.0.0
http.port: 9200
discovery.type: single-node

# Security
plugins.security.ssl.http.enabled: true
plugins.security.ssl.http.pemcert_filepath: certs/node.pem
plugins.security.ssl.http.pemkey_filepath: certs/node-key.pem
plugins.security.ssl.http.pemtrustedcas_filepath: certs/root-ca.pem

# Authentication
plugins.security.authcz.admin_dn:
  - CN=admin,OU=IT,O=Company,L=City,ST=State,C=US
```

**Step 3: Start OpenSearch**

```bash
sudo systemctl start opensearch
sudo systemctl enable opensearch

# Check status
sudo systemctl status opensearch
```

**Step 4: Configure Admin Portal**

```bash
OPENSEARCH_NODE=https://your-server:9200
OPENSEARCH_USERNAME=admin
OPENSEARCH_PASSWORD=your-secure-password
```

---

## Environment Variables

The Admin Portal requires the following environment variables for OpenSearch integration:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OPENSEARCH_NODE` | No | `null` | OpenSearch cluster URL (e.g., `https://localhost:9200`). If not set, search features are disabled. |
| `OPENSEARCH_USERNAME` | No | `admin` | Username for OpenSearch authentication. Only required if cluster has authentication enabled. |
| `OPENSEARCH_PASSWORD` | No | `Admin123!` | Password for OpenSearch authentication. Only required if cluster has authentication enabled. |

**Configuration Methods:**

1. **Environment Variables (Development):**
   ```bash
   export OPENSEARCH_NODE=https://localhost:9200
   export OPENSEARCH_USERNAME=admin
   export OPENSEARCH_PASSWORD=Admin123!
   ```

2. **Management UI (Production):**
   - Navigate to Settings → Secrets in the Admin Portal Management UI
   - Add each environment variable with its value
   - Click "Save"

3. **Docker Compose:**
   ```yaml
   services:
     admin-portal:
       environment:
         - OPENSEARCH_NODE=https://opensearch:9200
         - OPENSEARCH_USERNAME=admin
         - OPENSEARCH_PASSWORD=Admin123!
   ```

4. **Kubernetes ConfigMap/Secret:**
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: opensearch-credentials
   type: Opaque
   stringData:
     OPENSEARCH_NODE: https://opensearch-cluster:9200
     OPENSEARCH_USERNAME: admin
     OPENSEARCH_PASSWORD: your-password
   ```

---

## Index Initialization

When the Admin Portal starts with OpenSearch configured, it automatically creates the following indices:

| Index Name | Purpose | Searchable Fields |
|------------|---------|-------------------|
| `admin_portal_programs` | Benefit programs | name, description, benefitType, status |
| `admin_portal_users` | Platform users | name, email, role, loginMethod |
| `admin_portal_mcc_codes` | Merchant category codes | mccCode, description, category |
| `admin_portal_audit_logs` | Administrative actions | action, targetType, justification, performedByName |

**Automatic Index Creation:** The Admin Portal automatically creates these indices with proper mappings on first startup. No manual index creation is required.

**Index Mappings:** Each index includes optimized field mappings for full-text search, keyword filtering, and date range queries. The mappings are defined in `server/opensearch.ts`.

---

## Data Synchronization

The Admin Portal automatically synchronizes data to OpenSearch when configured:

**Automatic Sync Events:**
- **Program Creation:** New benefit programs are indexed immediately
- **Program Updates:** Changes to programs update the corresponding document
- **Program Deletion:** Removed programs are deleted from the index
- **User Registration:** New users are indexed on first login
- **MCC Import:** Bulk MCC imports trigger batch indexing
- **Audit Logging:** Administrative actions are indexed in real-time

**Manual Reindexing:** If you need to rebuild indices from scratch (e.g., after data migration):

```bash
# Connect to your database and run this SQL to get all data
# Then use the bulk indexing API

# Example: Reindex all programs
curl -k -u admin:Admin123! -X POST "https://localhost:9200/_bulk" \
  -H "Content-Type: application/x-ndjson" \
  --data-binary @programs-bulk.ndjson
```

**Bulk Indexing Script:** For large datasets, use the provided bulk indexing script:

```bash
cd /home/ubuntu/admin-portal
node scripts/reindex-opensearch.mjs
```

---

## Search Features

Once OpenSearch is configured, administrators can use these search features:

### Global Search (Cmd/Ctrl+K)

The global search dialog allows searching across all entity types simultaneously.

**Features:**
- **Fuzzy Matching:** Automatically handles typos and variations
- **Multi-Field Search:** Searches names, descriptions, emails, and other text fields
- **Highlighted Results:** Matched terms are highlighted in results
- **Type Filtering:** Filter results by entity type (programs, users, MCC codes, audit logs)
- **Date Range:** Filter by creation or modification date
- **Keyboard Navigation:** Arrow keys to navigate, Enter to select

**Usage:**
1. Press `Cmd+K` (Mac) or `Ctrl+K` (Windows/Linux)
2. Type your search query
3. Optionally select filters (type, date range)
4. Click or press Enter to view detailed results

### Saved Search Filters

Administrators can save frequently-used search queries for quick access.

**Creating Saved Filters:**
1. Perform a search with desired parameters
2. Click "Save Filter" button
3. Enter a name for the filter
4. Filter is saved and appears in the sidebar

**Using Saved Filters:**
1. Click on a saved filter in the sidebar
2. Search is automatically executed with saved parameters
3. Results are displayed instantly

---

## Performance Tuning

For production deployments, optimize OpenSearch performance:

### Index Settings

```json
{
  "settings": {
    "number_of_shards": 2,
    "number_of_replicas": 1,
    "refresh_interval": "5s",
    "max_result_window": 10000
  }
}
```

### Query Optimization

The Admin Portal uses optimized queries with:
- **Multi-match queries** for cross-field searching
- **Fuzziness: AUTO** for typo tolerance
- **Result size limit: 50** to prevent slow queries
- **Field boosting** (e.g., `name^2`) to prioritize important fields

### Monitoring

Monitor OpenSearch performance:

```bash
# Check cluster health
curl -k -u admin:Admin123! https://localhost:9200/_cluster/health?pretty

# Check index stats
curl -k -u admin:Admin123! https://localhost:9200/_cat/indices?v

# Check node stats
curl -k -u admin:Admin123! https://localhost:9200/_nodes/stats?pretty
```

---

## Troubleshooting

### Issue: Connection Refused

**Symptoms:** Admin Portal logs show "Failed to connect to OpenSearch"

**Solutions:**
1. Verify OpenSearch is running: `curl -k https://localhost:9200`
2. Check firewall rules allow traffic on port 9200
3. Verify `OPENSEARCH_NODE` URL is correct
4. Check network connectivity between Admin Portal and OpenSearch

### Issue: Authentication Failed

**Symptoms:** "401 Unauthorized" errors in logs

**Solutions:**
1. Verify `OPENSEARCH_USERNAME` and `OPENSEARCH_PASSWORD` are correct
2. Check user has necessary permissions in OpenSearch
3. For AWS OpenSearch, verify IAM roles and access policies

### Issue: Search Returns No Results

**Symptoms:** Global search returns empty results despite data existing

**Solutions:**
1. Check indices exist: `curl -k -u admin:Admin123! https://localhost:9200/_cat/indices`
2. Verify data is indexed: `curl -k -u admin:Admin123! https://localhost:9200/admin_portal_programs/_search`
3. Check index mappings are correct
4. Trigger manual reindexing if needed

### Issue: Slow Search Performance

**Symptoms:** Search queries take several seconds to complete

**Solutions:**
1. Increase OpenSearch cluster resources (CPU, RAM)
2. Add more nodes to distribute load
3. Optimize index settings (reduce replicas for development)
4. Reduce result size limit
5. Add caching layer for frequently-used queries

---

## Security Best Practices

### Production Deployment

1. **Use HTTPS:** Always use TLS/SSL for OpenSearch connections in production
2. **Strong Passwords:** Use complex passwords for OpenSearch authentication
3. **Network Isolation:** Place OpenSearch in a private subnet, not publicly accessible
4. **Access Control:** Use fine-grained access control to limit permissions
5. **Encryption at Rest:** Enable encryption for stored data
6. **Regular Updates:** Keep OpenSearch updated with latest security patches

### AWS OpenSearch Service

1. **VPC Access:** Use VPC access instead of public access
2. **IAM Roles:** Use IAM roles for authentication when possible
3. **Security Groups:** Restrict access to specific IP ranges
4. **Encryption:** Enable encryption at rest and in transit
5. **Audit Logging:** Enable audit logs for compliance

---

## Maintenance

### Regular Tasks

**Daily:**
- Monitor cluster health
- Check disk space usage
- Review error logs

**Weekly:**
- Review index sizes and optimize if needed
- Check for failed indexing operations
- Update saved search filters based on usage patterns

**Monthly:**
- Review and update access policies
- Optimize index settings based on usage patterns
- Plan capacity upgrades if needed

### Backup and Recovery

**Snapshot Configuration:**

```bash
# Register snapshot repository
curl -k -u admin:Admin123! -X PUT "https://localhost:9200/_snapshot/backup_repo" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "fs",
    "settings": {
      "location": "/mnt/backups/opensearch"
    }
  }'

# Create snapshot
curl -k -u admin:Admin123! -X PUT "https://localhost:9200/_snapshot/backup_repo/snapshot_1"

# Restore snapshot
curl -k -u admin:Admin123! -X POST "https://localhost:9200/_snapshot/backup_repo/snapshot_1/_restore"
```

---

## Migration Guide

### Migrating from No Search to OpenSearch

If you're adding OpenSearch to an existing Admin Portal deployment:

**Step 1: Deploy OpenSearch**
- Follow deployment instructions for your chosen option
- Ensure OpenSearch is accessible from Admin Portal

**Step 2: Configure Environment Variables**
- Add `OPENSEARCH_NODE`, `OPENSEARCH_USERNAME`, `OPENSEARCH_PASSWORD`
- Restart Admin Portal

**Step 3: Initial Data Sync**
- Admin Portal will automatically create indices on startup
- Existing data will be indexed on next modification
- For immediate full sync, use bulk reindexing script

**Step 4: Verify**
- Test global search functionality
- Check all entity types are searchable
- Verify search results are accurate

### Migrating Between OpenSearch Clusters

To migrate from one OpenSearch cluster to another:

**Step 1: Create Snapshot**
```bash
curl -k -u admin:Admin123! -X PUT "https://old-cluster:9200/_snapshot/backup_repo/migration_snapshot"
```

**Step 2: Transfer Snapshot**
- Copy snapshot data to new cluster's snapshot repository

**Step 3: Restore on New Cluster**
```bash
curl -k -u admin:Admin123! -X POST "https://new-cluster:9200/_snapshot/backup_repo/migration_snapshot/_restore"
```

**Step 4: Update Admin Portal Configuration**
- Update `OPENSEARCH_NODE` to point to new cluster
- Restart Admin Portal

---

## Cost Optimization

### AWS OpenSearch Service

**Development:**
- Use t3.small.search instances
- Single node (no replicas)
- 20 GB gp3 storage
- **Estimated cost:** $30-50/month

**Production:**
- Use m6g.large.search instances (2 nodes)
- 1 replica for high availability
- 100 GB gp3 storage
- **Estimated cost:** $200-300/month

**Cost Reduction Tips:**
1. Use Reserved Instances for predictable workloads (up to 50% savings)
2. Right-size instances based on actual usage
3. Use lifecycle policies to delete old indices
4. Consider UltraWarm storage for infrequently-accessed data

### Self-Hosted

**Infrastructure Costs:**
- 2x servers (8 GB RAM, 4 vCPU each)
- 100 GB SSD storage per node
- Load balancer
- **Estimated cost:** $100-150/month (cloud VMs)

---

## Additional Resources

**Official Documentation:**
- OpenSearch Documentation: https://opensearch.org/docs/latest/
- AWS OpenSearch Service: https://docs.aws.amazon.com/opensearch-service/

**Admin Portal Files:**
- OpenSearch client: `server/opensearch.ts`
- Index mappings: `server/opensearch.ts` (lines 50-101)
- Search implementation: `server/routers.ts` (search procedures)
- Global search UI: `client/src/components/GlobalSearch.tsx`

**Support:**
- For OpenSearch issues: https://github.com/opensearch-project/OpenSearch/issues
- For Admin Portal integration: Consult platform documentation

---

**Last Updated:** November 10, 2025  
**Version:** 1.0.0
