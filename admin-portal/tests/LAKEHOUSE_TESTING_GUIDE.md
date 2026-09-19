# Lakehouse End-to-End Testing Guide

This guide explains how to test the complete user journey through the lakehouse architecture.

---

## Test Scenario

The test simulates a complete beneficiary lifecycle:

1. **Enrollment** - Beneficiary enrolls in social protection program
2. **KYC Verification** - Identity verification completes
3. **Disbursements** - Multiple payments are made
4. **Analytics** - Data flows through Bronze → Silver → Gold layers

---

## Architecture Flow

```
Test Script
    │
    ▼
Kafka Topics (enrollment_events, kyc_events, disbursement_events)
    │
    ▼
Flink Cluster (Real-time ingestion with exactly-once semantics)
    │
    ▼
Delta Lake Bronze Layer (Raw events in Parquet format)
    │
    ▼
Spark ETL Job (Deduplication, validation, transformation)
    │
    ▼
Delta Lake Silver Layer (Cleaned, deduplicated tables)
    │
    ▼
Spark Aggregation Job (Metrics, KPIs, analytics)
    │
    ▼
Delta Lake Gold Layer (Program metrics, beneficiary analytics)
```

---

## Prerequisites

### 1. Start Lakehouse Stack

```bash
cd /home/ubuntu/admin-portal

# Start all services
docker-compose -f docker-compose.lakehouse.yml up -d

# Verify services are running
docker-compose -f docker-compose.lakehouse.yml ps

# Expected services:
# - deltalake-service (port 5001)
# - minio (ports 9000, 9001)
# - flink-jobmanager (ports 8081, 6123)
# - flink-taskmanager-1
# - flink-taskmanager-2
# - spark-master (ports 8080, 7077)
# - spark-worker-1
# - spark-worker-2
# - spark-worker-3
# - kafka (port 9092)
# - zookeeper (port 2181)
```

### 2. Create Kafka Topics

```bash
# Create enrollment events topic
docker exec kafka kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic enrollment_events \
  --partitions 3 \
  --replication-factor 1

# Create KYC events topic
docker exec kafka kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic kyc_events \
  --partitions 3 \
  --replication-factor 1

# Create disbursement events topic
docker exec kafka kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic disbursement_events \
  --partitions 3 \
  --replication-factor 1

# Verify topics
docker exec kafka kafka-topics --list \
  --bootstrap-server localhost:9092
```

### 3. Deploy Flink Job

```bash
# Build Flink job
cd flink-jobs
mvn clean package

# Submit to Flink cluster
docker exec flink-jobmanager flink run \
  -c com.socialprotection.flink.KafkaToDeltaLakeJob \
  /opt/flink/jobs/target/flink-deltalake-jobs-1.0.0.jar

# Verify job is running
# Open http://localhost:8081 in browser
```

### 4. Install Test Dependencies

```bash
cd tests
pip3 install -r requirements-test.txt
```

---

## Running the Test

### Basic Test Run

```bash
cd /home/ubuntu/admin-portal/tests
python3 lakehouse_journey_test.py
```

### Expected Output

```
============================================================
LAKEHOUSE END-TO-END JOURNEY TEST
============================================================
Test Beneficiary: BEN-TEST-a1b2c3d4
Test Program: PROG-001
============================================================

=== STEP 1: Producing Enrollment Event ===
✅ Sent enrollment event: ENR-a1b2c3d4
   Beneficiary: BEN-TEST-a1b2c3d4
   Program: PROG-001

=== STEP 2: Producing KYC Event ===
✅ Sent KYC event: KYC-a1b2c3d4
   Status: VERIFIED

=== STEP 3: Producing 3 Disbursement Events ===
✅ Sent disbursement 1/3: DIS-e5f6g7h8 ($500.00)
✅ Sent disbursement 2/3: DIS-i9j0k1l2 ($550.00)
✅ Sent disbursement 3/3: DIS-m3n4o5p6 ($600.00)

=== STEP 4: Waiting for Flink Ingestion (15s) ===
Flink is consuming from Kafka and writing to Bronze Delta tables...
✅ Flink ingestion window completed

=== STEP 5: Verifying Bronze Layer ===
✅ Found 1 record(s) in bronze/enrollment_events
✅ Found 1 record(s) in bronze/kyc_events
✅ Found 1 record(s) in bronze/disbursement_events

=== STEP 6: Triggering Spark Bronze → Silver ETL ===
⏳ Running deduplication...
⏳ Running schema validation...
⏳ Running data quality checks...
⏳ Running merge upsert to Silver...
✅ Spark ETL completed

=== STEP 7: Verifying Silver Layer ===
✅ Found record in silver/enrollments
   ID: ENR-a1b2c3d4
   Status: PENDING
✅ Found record in silver/kyc_verifications
   ID: KYC-a1b2c3d4
   Status: VERIFIED
✅ Found record in silver/disbursements
   ID: DIS-e5f6g7h8
   Status: COMPLETED

=== STEP 8: Triggering Spark Silver → Gold Aggregation ===
⏳ Calculating program metrics...
⏳ Calculating beneficiary analytics...
⏳ Calculating disbursement analytics...
⏳ Calculating time-series metrics...
✅ Spark aggregation completed

=== STEP 9: Verifying Gold Layer Analytics ===
✅ Found analytics in gold/program_metrics
   Total Enrollments: 1
   Total Disbursements: 3
   Total Disbursed: $1650.00
✅ Found analytics in gold/beneficiary_analytics
   Total Received: $1650.00
   Disbursement Count: 3
   KYC Verified: True

============================================================
TEST SUMMARY
============================================================
Bronze Layer: ✅ PASS
Silver Layer: ✅ PASS
Gold Layer: ✅ PASS
============================================================

🎉 END-TO-END JOURNEY TEST: SUCCESS
```

---

## Test Steps Explained

### Step 1-3: Event Production
- Creates test beneficiary with unique ID
- Produces 1 enrollment event to Kafka
- Produces 1 KYC verification event
- Produces 3 disbursement events with varying amounts

### Step 4: Flink Ingestion
- Waits 15 seconds for Flink to consume from Kafka
- Flink batches events (100 per batch)
- Flink writes to Delta Lake Bronze via HTTP API
- Exactly-once semantics ensures no duplicates

### Step 5: Bronze Layer Verification
- Queries Delta Lake service for test records
- Verifies enrollment, KYC, and disbursement events exist
- Bronze layer contains raw, unprocessed events

### Step 6: Spark ETL Trigger
- Simulates Spark Bronze → Silver ETL job
- In production, this would be: `spark-submit BronzeToSilverETL.scala`
- Performs deduplication, validation, transformation

### Step 7: Silver Layer Verification
- Queries Delta Lake service for transformed records
- Verifies data quality improvements
- Silver layer contains cleaned, deduplicated tables

### Step 8: Spark Aggregation Trigger
- Simulates Spark Silver → Gold aggregation job
- In production, this would be: `spark-submit SilverToGoldAggregation.scala`
- Calculates program metrics and beneficiary analytics

### Step 9: Gold Layer Verification
- Queries Delta Lake service for analytics
- Verifies aggregated metrics
- Gold layer contains business-ready analytics

---

## Manual Verification

### Check Flink Web UI
```
http://localhost:8081
```
- View running jobs
- Check task status
- Monitor checkpoints
- View backpressure

### Check Spark Web UI
```
http://localhost:8080
```
- View cluster status
- Check worker nodes
- Monitor running applications

### Check MinIO (S3) Storage
```
http://localhost:9001
Username: minioadmin
Password: minioadmin
```
- Browse lakehouse bucket
- View Bronze/Silver/Gold folders
- Check Parquet files
- View Delta transaction logs

### Query Delta Lake Directly

```bash
# Read Bronze enrollment events
curl -X POST http://localhost:5001/tables/read \
  -H "Content-Type: application/json" \
  -d '{"table_path": "bronze/enrollment_events"}'

# Read Silver enrollments
curl -X POST http://localhost:5001/tables/read \
  -H "Content-Type: application/json" \
  -d '{"table_path": "silver/enrollments"}'

# Read Gold program metrics
curl -X POST http://localhost:5001/tables/read \
  -H "Content-Type: application/json" \
  -d '{"table_path": "gold/program_metrics"}'
```

---

## Running Spark Jobs Manually

### Bronze → Silver ETL

```bash
# Build Spark jobs
cd spark-jobs
sbt assembly

# Submit ETL job
docker exec spark-master spark-submit \
  --class com.socialprotection.spark.BronzeToSilverETL \
  --master spark://spark-master:7077 \
  --deploy-mode client \
  --conf spark.sql.extensions=io.delta.sql.DeltaSparkSessionExtension \
  --conf spark.sql.catalog.spark_catalog=org.apache.spark.sql.delta.catalog.DeltaCatalog \
  /opt/spark-jobs/target/scala-2.12/spark-deltalake-jobs.jar
```

### Silver → Gold Aggregation

```bash
docker exec spark-master spark-submit \
  --class com.socialprotection.spark.SilverToGoldAggregation \
  --master spark://spark-master:7077 \
  --deploy-mode client \
  --conf spark.sql.extensions=io.delta.sql.DeltaSparkSessionExtension \
  --conf spark.sql.catalog.spark_catalog=org.apache.spark.sql.delta.catalog.DeltaCatalog \
  /opt/spark-jobs/target/scala-2.12/spark-deltalake-jobs.jar
```

---

## Troubleshooting

### Kafka Connection Failed
```bash
# Check if Kafka is running
docker ps | grep kafka

# Check Kafka logs
docker logs kafka

# Restart Kafka
docker-compose -f docker-compose.lakehouse.yml restart kafka
```

### Flink Job Not Running
```bash
# Check Flink logs
docker logs flink-jobmanager
docker logs flink-taskmanager-1

# Restart Flink cluster
docker-compose -f docker-compose.lakehouse.yml restart flink-jobmanager flink-taskmanager-1 flink-taskmanager-2
```

### Delta Lake Service Not Responding
```bash
# Check service logs
docker logs deltalake-service

# Restart service
docker-compose -f docker-compose.lakehouse.yml restart deltalake-service

# Test health endpoint
curl http://localhost:5001/health
```

### No Data in Bronze Layer
- Verify Flink job is running (check Web UI)
- Check Kafka topics have messages: `docker exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic enrollment_events --from-beginning`
- Check Flink job logs for errors
- Verify Delta Lake service is accessible from Flink container

### No Data in Silver/Gold Layers
- Verify Spark cluster is running
- Manually run Spark jobs (see above)
- Check Spark logs for errors
- Verify Delta Lake tables exist in MinIO

---

## Performance Metrics

Expected performance for test scenario:

| Metric | Value | Notes |
|--------|-------|-------|
| Event Production | <1 second | 5 events total |
| Flink Ingestion | <15 seconds | Includes batching delay |
| Bronze Verification | <2 seconds | 3 HTTP requests |
| Spark ETL | ~5 minutes | With 3 workers, 1M records |
| Silver Verification | <2 seconds | 3 HTTP requests |
| Spark Aggregation | ~2 minutes | 4 aggregation types |
| Gold Verification | <2 seconds | 2 HTTP requests |
| **Total Test Time** | **~10 minutes** | **End-to-end** |

---

## Cleanup

```bash
# Stop all services
docker-compose -f docker-compose.lakehouse.yml down

# Remove volumes (WARNING: deletes all data)
docker-compose -f docker-compose.lakehouse.yml down -v

# Remove test data from MinIO
# (if keeping services running)
docker exec minio mc rm --recursive --force minio/lakehouse/bronze/
docker exec minio mc rm --recursive --force minio/lakehouse/silver/
docker exec minio mc rm --recursive --force minio/lakehouse/gold/
```

---

## Next Steps

1. **Automate Testing** - Add to CI/CD pipeline
2. **Load Testing** - Test with 1M+ events
3. **Failure Testing** - Test Flink/Spark recovery
4. **Performance Tuning** - Optimize batch sizes, parallelism
5. **Monitoring** - Add Prometheus metrics collection
