# Standard Lakehouse Architecture for Social Protection Platform

## Executive Summary

This document describes the comprehensive lakehouse architecture integrating **Delta Lake**, **Apache Spark**, **Apache Flink**, **Apache DataFusion**, **Ray**, and **Apache Sedona** to create an enterprise-grade data platform for the Social Protection Platform with advanced geospatial analytics capabilities.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Data Sources                                     │
├─────────────────────────────────────────────────────────────────────────┤
│  Kafka Topics  │  MySQL/TiDB  │  External APIs  │  File Uploads        │
└────────┬────────────────┬──────────────┬────────────────┬───────────────┘
         │                │              │                │
         ▼                ▼              ▼                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Ingestion Layer (Apache Flink)                        │
├─────────────────────────────────────────────────────────────────────────┤
│  • Real-time streaming from Kafka                                        │
│  • CDC from databases (Debezium)                                         │
│  • Exactly-once semantics                                                │
│  • Data validation and enrichment                                        │
└────────────────────────────┬───────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                  Storage Layer (Delta Lake on S3/HDFS)                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  Bronze Layer (Raw Data)                                         │   │
│  │  • Raw events from Kafka                                         │   │
│  │  • CDC snapshots                                                 │   │
│  │  • Parquet format with Snappy compression                        │   │
│  │  • Partitioned by date/event_type                                │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                             │                                             │
│                             ▼                                             │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  Silver Layer (Cleaned & Validated)                              │   │
│  │  • Deduplicated events                                           │   │
│  │  • Schema validated                                              │   │
│  │  • Enriched with reference data                                  │   │
│  │  • Z-ordered for query performance                               │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                             │                                             │
│                             ▼                                             │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  Gold Layer (Business Aggregates)                                │   │
│  │  • Beneficiary profiles                                          │   │
│  │  • Program metrics                                               │   │
│  │  • Geospatial datasets                                           │   │
│  │  • ML features                                                   │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                           │
└────────────────────────────┬──────────────────────────────────────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
┌─────────────────┐  ┌──────────────┐  ┌────────────────────┐
│  Apache Spark   │  │  DataFusion  │  │   Apache Sedona    │
│  (Batch ETL)    │  │  (SQL Query) │  │  (Geospatial)      │
├─────────────────┤  ├──────────────┤  ├────────────────────┤
│ • Bronze→Silver │  │ • Interactive│  │ • Spatial queries  │
│ • Silver→Gold   │  │   queries    │  │ • Coverage maps    │
│ • Aggregations  │  │ • OLAP       │  │ • Proximity        │
│ • ML prep       │  │ • Dashboards │  │ • Hotspot analysis │
└─────────────────┘  └──────────────┘  └────────────────────┘
         │                   │                   │
         └───────────────────┼───────────────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │      Ray        │
                    │  (Distributed   │
                    │   ML & AI)      │
                    ├─────────────────┤
                    │ • Fraud detect  │
                    │ • Risk scoring  │
                    │ • Predictions   │
                    │ • Feature eng   │
                    └─────────────────┘
```

## Component Details

### 1. Delta Lake (Storage Foundation)

**Purpose:** ACID-compliant data lake storage with versioning and time travel.

**Key Features:**
- **ACID Transactions:** Ensures data consistency across concurrent reads/writes
- **Time Travel:** Query historical versions for audit and rollback
- **Schema Evolution:** Add/modify columns without breaking downstream
- **Z-Ordering:** Optimize query performance with data clustering
- **Change Data Feed:** Track row-level changes for CDC
- **Deletion Vectors:** Efficient row-level deletes

**Implementation:**
- **Library:** delta-rs (Rust core with Go/Python bindings)
- **Format:** Parquet files + JSON transaction log
- **Storage:** S3-compatible object storage (MinIO for dev, S3 for prod)

**Data Organization:**
```
s3://lakehouse/
├── bronze/
│   ├── kafka_events/
│   │   ├── _delta_log/
│   │   │   ├── 00000000000000000000.json
│   │   │   ├── 00000000000000000010.checkpoint.parquet
│   │   └── partition_date=2024-01-01/
│   │       ├── part-00000.snappy.parquet
│   │       └── part-00001.snappy.parquet
│   └── cdc_snapshots/
├── silver/
│   ├── beneficiaries/
│   ├── disbursements/
│   └── enrollments/
└── gold/
    ├── beneficiary_profiles/
    ├── program_metrics/
    └── geospatial_datasets/
```

### 2. Apache Flink (Real-time Streaming)

**Purpose:** Process real-time event streams with exactly-once guarantees.

**Use Cases:**
- Kafka → Delta Lake ingestion
- Real-time data validation
- Stream enrichment
- Windowed aggregations
- CDC processing

**Architecture:**
```
Kafka Topics → Flink Source → Flink Operators → Delta Lake Sink
                                    │
                                    ├─ Map (transform)
                                    ├─ Filter (validate)
                                    ├─ Window (aggregate)
                                    └─ Join (enrich)
```

**Jobs:**
1. **Event Ingestion Job**
   - Source: Kafka (disbursement_events, enrollment_events, etc.)
   - Transform: Parse JSON, validate schema, add metadata
   - Sink: Delta Lake Bronze layer

2. **CDC Ingestion Job**
   - Source: Debezium CDC topics
   - Transform: Convert CDC format to Delta Lake format
   - Sink: Delta Lake Bronze layer

3. **Real-time Aggregation Job**
   - Source: Delta Lake Bronze
   - Transform: Windowed aggregations (5-minute windows)
   - Sink: Delta Lake Silver layer

**Configuration:**
- **Parallelism:** 4-8 slots per job
- **Checkpointing:** Every 60 seconds
- **State Backend:** RocksDB
- **Exactly-once:** Enabled with 2-phase commit

### 3. Apache Spark (Batch Processing)

**Purpose:** Large-scale batch ETL and data transformation.

**Use Cases:**
- Bronze → Silver transformation
- Silver → Gold aggregation
- Complex joins and deduplication
- ML feature engineering
- Historical data processing

**Jobs:**
1. **Bronze to Silver ETL**
   ```scala
   // Read from Bronze Delta table
   val bronze = spark.read.format("delta").load("s3://lakehouse/bronze/kafka_events")
   
   // Deduplicate, validate, enrich
   val silver = bronze
     .dropDuplicates("event_id")
     .filter("event_type IS NOT NULL")
     .join(referenceData, "program_id")
     .withColumn("processed_at", current_timestamp())
   
   // Write to Silver Delta table with Z-ordering
   silver.write
     .format("delta")
     .mode("append")
     .option("mergeSchema", "true")
     .save("s3://lakehouse/silver/events")
   
   // Optimize with Z-ordering
   spark.sql("OPTIMIZE delta.`s3://lakehouse/silver/events` ZORDER BY (program_id, beneficiary_id)")
   ```

2. **Gold Aggregation**
   ```scala
   // Create beneficiary profiles
   val profiles = spark.sql("""
     SELECT 
       beneficiary_id,
       MAX(name) as name,
       MAX(location) as location,
       COUNT(DISTINCT program_id) as programs_enrolled,
       SUM(amount_received) as total_received,
       MAX(last_disbursement_date) as last_disbursement
     FROM delta.`s3://lakehouse/silver/disbursements`
     GROUP BY beneficiary_id
   """)
   
   profiles.write.format("delta").mode("overwrite").save("s3://lakehouse/gold/beneficiary_profiles")
   ```

**Cluster Configuration:**
- **Driver:** 4 cores, 8 GB RAM
- **Executors:** 8 executors × 4 cores × 16 GB RAM
- **Dynamic Allocation:** Enabled (min 2, max 20 executors)

### 4. Apache DataFusion (SQL Query Engine)

**Purpose:** Fast, embeddable SQL query engine for interactive analytics.

**Advantages:**
- **Rust-based:** High performance with low memory footprint
- **Embeddable:** Can run in Go applications via FFI
- **Vectorized:** SIMD-optimized query execution
- **Parquet-native:** Direct Parquet file reading

**Use Cases:**
- Interactive dashboards
- Ad-hoc queries
- Real-time analytics API
- Report generation

**Integration:**
```rust
// Rust service exposing DataFusion via gRPC
use datafusion::prelude::*;

#[tokio::main]
async fn main() -> Result<()> {
    let ctx = SessionContext::new();
    
    // Register Delta Lake tables
    ctx.register_parquet(
        "beneficiaries",
        "s3://lakehouse/gold/beneficiary_profiles/*.parquet",
        ParquetReadOptions::default()
    ).await?;
    
    // Execute query
    let df = ctx.sql("
        SELECT program_id, COUNT(*) as beneficiaries, SUM(total_received) as total_amount
        FROM beneficiaries
        WHERE last_disbursement >= '2024-01-01'
        GROUP BY program_id
    ").await?;
    
    df.show().await?;
    Ok(())
}
```

**Deployment:**
- **Service:** gRPC server in Rust
- **Client:** Go client library
- **Caching:** Redis for query result caching

### 5. Ray (Distributed ML & AI)

**Purpose:** Distributed machine learning and AI workloads.

**Use Cases:**
- Fraud detection models
- Risk scoring
- Beneficiary eligibility prediction
- Demand forecasting
- Anomaly detection

**Architecture:**
```
Ray Cluster
├── Head Node (scheduler, driver)
└── Worker Nodes (8 nodes × 16 cores)
    ├── Ray Actors (stateful tasks)
    ├── Ray Tasks (stateless tasks)
    └── Ray Datasets (distributed data)
```

**Example: Fraud Detection**
```python
import ray
from ray import train
from ray.data import read_parquet
import xgboost as xgb

# Initialize Ray
ray.init(address="ray://ray-cluster:10001")

# Load data from Delta Lake
ds = read_parquet("s3://lakehouse/gold/disbursements/*.parquet")

# Prepare features
def prepare_features(batch):
    # Feature engineering
    batch["amount_zscore"] = (batch["amount"] - batch["amount"].mean()) / batch["amount"].std()
    batch["time_since_last"] = (batch["timestamp"] - batch["last_disbursement"]).dt.days
    return batch

ds = ds.map_batches(prepare_features)

# Train model
def train_fraud_model(config):
    train_set = train.get_dataset_shard("train")
    
    dtrain = xgb.DMatrix(
        train_set.to_pandas()[["amount_zscore", "time_since_last"]],
        label=train_set.to_pandas()["is_fraud"]
    )
    
    model = xgb.train(
        {"objective": "binary:logistic", "max_depth": 6},
        dtrain,
        num_boost_round=100
    )
    
    return model

trainer = train.Trainer(
    backend="xgboost",
    num_workers=4,
    datasets={"train": ds}
)

result = trainer.fit()
```

**ML Pipeline:**
1. **Feature Store:** Store features in Delta Lake Gold layer
2. **Training:** Distributed training with Ray Train
3. **Serving:** Deploy model with Ray Serve
4. **Monitoring:** Track model performance with MLflow

### 6. Apache Sedona (Geospatial Analytics)

**Purpose:** Scalable geospatial data processing and analytics.

**Use Cases:**
- Beneficiary location analysis
- Program coverage mapping
- Proximity analysis (nearest service center)
- Hotspot detection (high-need areas)
- Spatial joins (beneficiaries within program zones)

**Integration with Spark:**
```scala
import org.apache.sedona.spark.SedonaContext

// Create Sedona-enabled Spark session
val sedona = SedonaContext.create(sparkSession)

// Register spatial UDFs
sedona.sql("SELECT sedona_version()")

// Load beneficiary locations
val beneficiaries = sedona.read.format("delta")
  .load("s3://lakehouse/gold/beneficiary_profiles")
  .selectExpr("beneficiary_id", "ST_Point(longitude, latitude) as location")

// Load program coverage zones
val programs = sedona.read.format("delta")
  .load("s3://lakehouse/gold/program_zones")
  .selectExpr("program_id", "ST_GeomFromWKT(coverage_polygon) as zone")

// Spatial join: Find beneficiaries in each program zone
val coverage = beneficiaries.join(programs, 
  expr("ST_Within(location, zone)")
)

coverage.groupBy("program_id").count().show()
```

**Geospatial Queries:**

1. **Proximity Analysis**
   ```sql
   -- Find nearest service center for each beneficiary
   SELECT 
     b.beneficiary_id,
     s.center_id,
     ST_Distance(b.location, s.location) as distance_km
   FROM beneficiaries b
   CROSS JOIN service_centers s
   WHERE ST_Distance(b.location, s.location) < 50  -- within 50km
   ORDER BY b.beneficiary_id, distance_km
   ```

2. **Hotspot Detection**
   ```sql
   -- Identify high-density areas using spatial clustering
   SELECT 
     ST_AsText(ST_Centroid(ST_Collect(location))) as hotspot_center,
     COUNT(*) as beneficiary_count
   FROM beneficiaries
   GROUP BY ST_GeoHash(location, 5)  -- 5km grid
   HAVING COUNT(*) > 100
   ```

3. **Coverage Gaps**
   ```sql
   -- Find areas with low program coverage
   SELECT 
     region_id,
     COUNT(DISTINCT b.beneficiary_id) as beneficiaries,
     COUNT(DISTINCT p.program_id) as programs,
     CASE 
       WHEN COUNT(DISTINCT p.program_id) = 0 THEN 'No Coverage'
       WHEN COUNT(DISTINCT p.program_id) < 3 THEN 'Low Coverage'
       ELSE 'Adequate Coverage'
     END as coverage_level
   FROM regions r
   LEFT JOIN beneficiaries b ON ST_Within(b.location, r.boundary)
   LEFT JOIN programs p ON ST_Intersects(p.zone, r.boundary)
   GROUP BY region_id
   ```

## Data Flow

### Real-time Flow (Flink)
```
Kafka Event → Flink Validation → Delta Lake Bronze → Flink Transform → Delta Lake Silver
                                                                              │
                                                                              ▼
                                                                    Real-time Dashboard
```

### Batch Flow (Spark)
```
Delta Lake Bronze → Spark Dedup → Delta Lake Silver → Spark Aggregate → Delta Lake Gold
                                                                              │
                                                                              ▼
                                                                    ML Feature Store
```

### Query Flow (DataFusion)
```
User Query → DataFusion SQL → Delta Lake Gold (Parquet) → Result Cache (Redis) → API Response
```

### ML Flow (Ray)
```
Delta Lake Gold → Ray Dataset → Ray Train → Model Registry (MLflow) → Ray Serve → Predictions
```

### Geospatial Flow (Sedona)
```
Location Data → Sedona Spatial Index → Spatial Queries → Visualization (GeoJSON) → Maps
```

## Deployment Architecture

### Development Environment (Docker Compose)
```yaml
version: '3.8'
services:
  minio:
    image: minio/minio
    ports: ["9000:9000", "9001:9001"]
    environment:
      MINIO_ROOT_USER: admin
      MINIO_ROOT_PASSWORD: password
    command: server /data --console-address ":9001"
  
  flink-jobmanager:
    image: flink:1.18
    ports: ["8081:8081"]
    command: jobmanager
  
  flink-taskmanager:
    image: flink:1.18
    depends_on: [flink-jobmanager]
    command: taskmanager
    scale: 2
  
  spark-master:
    image: bitnami/spark:3.5
    ports: ["8080:8080", "7077:7077"]
    environment:
      SPARK_MODE: master
  
  spark-worker:
    image: bitnami/spark:3.5
    depends_on: [spark-master]
    environment:
      SPARK_MODE: worker
      SPARK_MASTER_URL: spark://spark-master:7077
    scale: 3
  
  datafusion-service:
    build: ./datafusion-service
    ports: ["50051:50051"]
  
  ray-head:
    image: rayproject/ray:2.9.0
    ports: ["8265:8265", "10001:10001"]
    command: ray start --head --dashboard-host=0.0.0.0
  
  ray-worker:
    image: rayproject/ray:2.9.0
    depends_on: [ray-head]
    command: ray start --address=ray-head:6379
    scale: 4
```

### Production Environment (Kubernetes)
- **Flink:** Flink Kubernetes Operator
- **Spark:** Spark on K8s Operator
- **Ray:** KubeRay Operator
- **Storage:** S3 (AWS) or GCS (Google Cloud)
- **Monitoring:** Prometheus + Grafana
- **Logging:** ELK Stack

## Performance Characteristics

| Component | Throughput | Latency | Scalability |
|-----------|------------|---------|-------------|
| Flink Ingestion | 100K events/sec | <1s | Horizontal (add task managers) |
| Delta Lake Writes | 1M rows/sec | N/A | Vertical (faster storage) |
| Spark Batch | 10M rows/min | Minutes | Horizontal (add executors) |
| DataFusion Query | 1M rows/sec | <100ms | Vertical (more cores) |
| Ray ML Training | 1B samples/hour | Hours | Horizontal (add workers) |
| Sedona Spatial | 10M points/min | Seconds | Horizontal (Spark cluster) |

## Cost Estimation (AWS)

### Compute
- **Flink:** 2 × c5.2xlarge (8 vCPU, 16 GB) = $0.68/hr × 2 = $1,003/month
- **Spark:** 8 × r5.2xlarge (8 vCPU, 64 GB) = $1.008/hr × 8 = $5,898/month
- **Ray:** 4 × c5.4xlarge (16 vCPU, 32 GB) = $1.36/hr × 4 = $3,974/month
- **DataFusion:** 1 × c5.xlarge (4 vCPU, 8 GB) = $0.34/hr = $250/month

**Total Compute:** ~$11,125/month

### Storage (S3)
- **Data:** 10 TB × $0.023/GB = $235/month
- **Requests:** 1M PUT + 10M GET = $10/month

**Total Storage:** ~$245/month

### **Grand Total:** ~$11,370/month (~$136,440/year)

## Migration Plan

### Phase 1: Foundation (Week 1-2)
- [ ] Deploy MinIO for local S3-compatible storage
- [ ] Set up Delta Lake with delta-rs
- [ ] Migrate existing Parquet files to Delta format
- [ ] Test basic read/write operations

### Phase 2: Streaming (Week 3-4)
- [ ] Deploy Flink cluster
- [ ] Create Kafka → Delta Lake ingestion jobs
- [ ] Implement exactly-once semantics
- [ ] Test with production Kafka topics

### Phase 3: Batch Processing (Week 5-6)
- [ ] Deploy Spark cluster
- [ ] Create Bronze → Silver ETL jobs
- [ ] Create Silver → Gold aggregation jobs
- [ ] Schedule daily batch jobs

### Phase 4: Query Engine (Week 7-8)
- [ ] Deploy DataFusion service
- [ ] Create query API endpoints
- [ ] Integrate with Admin Portal
- [ ] Add query caching

### Phase 5: ML & Geospatial (Week 9-10)
- [ ] Deploy Ray cluster
- [ ] Migrate fraud detection models to Ray
- [ ] Deploy Apache Sedona
- [ ] Create geospatial queries

### Phase 6: Production (Week 11-12)
- [ ] Deploy to Kubernetes
- [ ] Set up monitoring and alerting
- [ ] Performance testing
- [ ] Documentation and training

## Conclusion

This comprehensive lakehouse architecture provides:

✅ **Enterprise-grade storage** with Delta Lake ACID guarantees  
✅ **Real-time streaming** with Apache Flink  
✅ **Scalable batch processing** with Apache Spark  
✅ **Fast SQL queries** with Apache DataFusion  
✅ **Distributed ML** with Ray  
✅ **Advanced geospatial analytics** with Apache Sedona  

**Next Steps:** Begin Phase 1 implementation with Delta Lake foundation.
