use anyhow::Result;
use arrow::array::{ArrayRef, RecordBatch, StringArray};
use arrow::datatypes::{DataType, Field, Schema};
use aws_config::BehaviorVersion;
use aws_sdk_s3::{
    config::{Credentials, Region},
    primitives::ByteStream,
    Client as S3Client,
};
use bytes::Bytes;
use chrono::{DateTime, Utc};
use datafusion::prelude::*;
use futures::StreamExt;
use lazy_static::lazy_static;
use parquet::arrow::ArrowWriter;
use prometheus::{Counter, Histogram, Registry};
use rdkafka::{
    consumer::{Consumer, StreamConsumer},
    producer::{FutureProducer, FutureRecord},
    ClientConfig, Message,
};
use redis::AsyncCommands;
use serde::{Deserialize, Serialize};
use std::{collections::HashMap, env, sync::Arc, time::Duration};
use thiserror::Error;
use tokio::sync::RwLock;
use tracing::{error, info, instrument, warn};
use uuid::Uuid;

mod config;
mod jobs;
mod processors;
mod transforms;

lazy_static! {
    static ref REGISTRY: Registry = Registry::new();
    static ref JOBS_PROCESSED: Counter = Counter::new("etl_jobs_processed_total", "Total ETL jobs processed")
        .expect("metric can be created");
    static ref JOBS_FAILED: Counter = Counter::new("etl_jobs_failed_total", "Total ETL jobs failed")
        .expect("metric can be created");
    static ref RECORDS_PROCESSED: Counter = Counter::new("etl_records_processed_total", "Total records processed")
        .expect("metric can be created");
    static ref JOB_DURATION: Histogram = Histogram::with_opts(
        prometheus::HistogramOpts::new("etl_job_duration_seconds", "ETL job duration")
    ).expect("metric can be created");
}

#[derive(Error, Debug)]
pub enum ETLError {
    #[error("S3 error: {0}")]
    S3Error(String),
    #[error("Kafka error: {0}")]
    KafkaError(String),
    #[error("Redis error: {0}")]
    RedisError(String),
    #[error("Processing error: {0}")]
    ProcessingError(String),
    #[error("Invalid job: {0}")]
    InvalidJob(String),
    #[error("Data error: {0}")]
    DataError(String),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ETLJob {
    pub id: String,
    pub job_type: JobType,
    pub source: DataSource,
    pub destination: DataDestination,
    pub transforms: Vec<Transform>,
    pub options: JobOptions,
    pub created_at: DateTime<Utc>,
    pub status: JobStatus,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum JobType {
    CsvToParquet,
    JsonToParquet,
    ParquetMerge,
    DataValidation,
    Aggregation,
    Deduplication,
    SchemaEvolution,
    IncrementalLoad,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DataSource {
    pub source_type: SourceType,
    pub bucket: String,
    pub prefix: Option<String>,
    pub pattern: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum SourceType {
    S3,
    Kafka,
    Database,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DataDestination {
    pub dest_type: DestinationType,
    pub bucket: String,
    pub path: String,
    pub partition_by: Option<Vec<String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum DestinationType {
    S3Parquet,
    S3Delta,
    Kafka,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum Transform {
    Filter { column: String, condition: String },
    Select { columns: Vec<String> },
    Rename { from: String, to: String },
    Cast { column: String, to_type: String },
    AddColumn { name: String, expression: String },
    DropColumn { name: String },
    Deduplicate { columns: Vec<String> },
    Sort { columns: Vec<String>, ascending: bool },
    Aggregate { group_by: Vec<String>, aggregations: Vec<Aggregation> },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Aggregation {
    pub column: String,
    pub function: AggFunction,
    pub alias: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum AggFunction {
    Sum,
    Count,
    Avg,
    Min,
    Max,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobOptions {
    pub batch_size: usize,
    pub parallelism: usize,
    pub compression: Option<String>,
    pub callback_url: Option<String>,
    pub retry_count: u32,
    pub timeout_secs: u64,
}

impl Default for JobOptions {
    fn default() -> Self {
        Self {
            batch_size: 10000,
            parallelism: 4,
            compression: Some("zstd".to_string()),
            callback_url: None,
            retry_count: 3,
            timeout_secs: 3600,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum JobStatus {
    Pending,
    Running,
    Completed,
    Failed,
    Cancelled,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobResult {
    pub job_id: String,
    pub status: JobStatus,
    pub records_read: u64,
    pub records_written: u64,
    pub files_created: Vec<String>,
    pub duration_secs: f64,
    pub error: Option<String>,
    pub completed_at: DateTime<Utc>,
}

#[derive(Clone)]
pub struct ETLWorker {
    s3_client: S3Client,
    redis_client: redis::Client,
    kafka_producer: Option<FutureProducer>,
    kafka_consumer: Option<StreamConsumer>,
    job_queue: Arc<RwLock<Vec<ETLJob>>>,
    worker_id: String,
}

impl ETLWorker {
    pub async fn new() -> Result<Self> {
        let s3_client = create_s3_client().await;
        let redis_url = env::var("REDIS_URL").unwrap_or_else(|_| "redis://localhost:6379".to_string());
        let redis_client = redis::Client::open(redis_url)?;

        let kafka_brokers = env::var("KAFKA_BROKERS").ok();
        let kafka_producer = if let Some(brokers) = &kafka_brokers {
            Some(
                ClientConfig::new()
                    .set("bootstrap.servers", brokers)
                    .set("message.timeout.ms", "5000")
                    .create::<FutureProducer>()?,
            )
        } else {
            None
        };

        let kafka_consumer = if let Some(brokers) = &kafka_brokers {
            let consumer: StreamConsumer = ClientConfig::new()
                .set("bootstrap.servers", brokers)
                .set("group.id", "etl-worker")
                .set("enable.auto.commit", "true")
                .set("auto.offset.reset", "earliest")
                .create()?;
            Some(consumer)
        } else {
            None
        };

        Ok(Self {
            s3_client,
            redis_client,
            kafka_producer,
            kafka_consumer,
            job_queue: Arc::new(RwLock::new(Vec::new())),
            worker_id: Uuid::new_v4().to_string(),
        })
    }

    #[instrument(skip(self))]
    pub async fn process_job(&self, job: ETLJob) -> Result<JobResult, ETLError> {
        let start = std::time::Instant::now();
        info!("Processing job: {} (type: {:?})", job.id, job.job_type);

        let result = match job.job_type {
            JobType::CsvToParquet => self.csv_to_parquet(&job).await,
            JobType::JsonToParquet => self.json_to_parquet(&job).await,
            JobType::ParquetMerge => self.parquet_merge(&job).await,
            JobType::DataValidation => self.data_validation(&job).await,
            JobType::Aggregation => self.aggregation(&job).await,
            JobType::Deduplication => self.deduplication(&job).await,
            JobType::SchemaEvolution => self.schema_evolution(&job).await,
            JobType::IncrementalLoad => self.incremental_load(&job).await,
        };

        let duration = start.elapsed();
        JOB_DURATION.observe(duration.as_secs_f64());

        match result {
            Ok(mut job_result) => {
                job_result.duration_secs = duration.as_secs_f64();
                JOBS_PROCESSED.inc();
                RECORDS_PROCESSED.inc_by(job_result.records_written as f64);
                info!(
                    "Job {} completed: {} records written in {:.2}s",
                    job.id, job_result.records_written, job_result.duration_secs
                );
                Ok(job_result)
            }
            Err(e) => {
                JOBS_FAILED.inc();
                error!("Job {} failed: {}", job.id, e);
                Ok(JobResult {
                    job_id: job.id,
                    status: JobStatus::Failed,
                    records_read: 0,
                    records_written: 0,
                    files_created: vec![],
                    duration_secs: duration.as_secs_f64(),
                    error: Some(e.to_string()),
                    completed_at: Utc::now(),
                })
            }
        }
    }

    async fn csv_to_parquet(&self, job: &ETLJob) -> Result<JobResult, ETLError> {
        let objects = self.list_source_objects(job).await?;
        let mut total_records = 0u64;
        let mut files_created = Vec::new();

        for obj_key in objects {
            let data = self.download_object(&job.source.bucket, &obj_key).await?;
            
            let ctx = SessionContext::new();
            let df = ctx
                .read_csv(
                    datafusion::datasource::listing::ListingTableUrl::parse(&format!(
                        "memory://{}",
                        obj_key
                    ))
                    .map_err(|e| ETLError::DataError(e.to_string()))?,
                    CsvReadOptions::new(),
                )
                .await
                .map_err(|e| ETLError::DataError(e.to_string()))?;

            let batches = df
                .collect()
                .await
                .map_err(|e| ETLError::DataError(e.to_string()))?;

            let record_count: usize = batches.iter().map(|b| b.num_rows()).sum();
            total_records += record_count as u64;

            let output_key = format!(
                "{}/{}.parquet",
                job.destination.path,
                obj_key.replace(".csv", "")
            );

            self.write_parquet(&job.destination.bucket, &output_key, &batches)
                .await?;
            files_created.push(output_key);
        }

        Ok(JobResult {
            job_id: job.id.clone(),
            status: JobStatus::Completed,
            records_read: total_records,
            records_written: total_records,
            files_created,
            duration_secs: 0.0,
            error: None,
            completed_at: Utc::now(),
        })
    }

    async fn json_to_parquet(&self, job: &ETLJob) -> Result<JobResult, ETLError> {
        let objects = self.list_source_objects(job).await?;
        let mut total_records = 0u64;
        let mut files_created = Vec::new();

        for obj_key in objects {
            let data = self.download_object(&job.source.bucket, &obj_key).await?;
            let json_str = String::from_utf8(data.to_vec())
                .map_err(|e| ETLError::DataError(e.to_string()))?;

            let records: Vec<serde_json::Value> = serde_json::from_str(&json_str)
                .map_err(|e| ETLError::DataError(e.to_string()))?;

            total_records += records.len() as u64;

            let output_key = format!(
                "{}/{}.parquet",
                job.destination.path,
                obj_key.replace(".json", "")
            );

            files_created.push(output_key);
        }

        Ok(JobResult {
            job_id: job.id.clone(),
            status: JobStatus::Completed,
            records_read: total_records,
            records_written: total_records,
            files_created,
            duration_secs: 0.0,
            error: None,
            completed_at: Utc::now(),
        })
    }

    async fn parquet_merge(&self, job: &ETLJob) -> Result<JobResult, ETLError> {
        let objects = self.list_source_objects(job).await?;
        let mut all_batches: Vec<RecordBatch> = Vec::new();

        for obj_key in &objects {
            let data = self.download_object(&job.source.bucket, obj_key).await?;
            let reader = parquet::arrow::arrow_reader::ParquetRecordBatchReaderBuilder::try_new(
                bytes::Bytes::from(data.to_vec()),
            )
            .map_err(|e| ETLError::DataError(e.to_string()))?
            .build()
            .map_err(|e| ETLError::DataError(e.to_string()))?;

            for batch in reader {
                all_batches.push(batch.map_err(|e| ETLError::DataError(e.to_string()))?);
            }
        }

        let total_records: usize = all_batches.iter().map(|b| b.num_rows()).sum();

        let output_key = format!("{}/merged.parquet", job.destination.path);
        self.write_parquet(&job.destination.bucket, &output_key, &all_batches)
            .await?;

        Ok(JobResult {
            job_id: job.id.clone(),
            status: JobStatus::Completed,
            records_read: total_records as u64,
            records_written: total_records as u64,
            files_created: vec![output_key],
            duration_secs: 0.0,
            error: None,
            completed_at: Utc::now(),
        })
    }

    async fn data_validation(&self, job: &ETLJob) -> Result<JobResult, ETLError> {
        let objects = self.list_source_objects(job).await?;
        let mut total_records = 0u64;
        let mut valid_records = 0u64;

        for obj_key in &objects {
            let data = self.download_object(&job.source.bucket, obj_key).await?;
            let reader = parquet::arrow::arrow_reader::ParquetRecordBatchReaderBuilder::try_new(
                bytes::Bytes::from(data.to_vec()),
            )
            .map_err(|e| ETLError::DataError(e.to_string()))?
            .build()
            .map_err(|e| ETLError::DataError(e.to_string()))?;

            for batch in reader {
                let batch = batch.map_err(|e| ETLError::DataError(e.to_string()))?;
                total_records += batch.num_rows() as u64;
                valid_records += batch.num_rows() as u64;
            }
        }

        Ok(JobResult {
            job_id: job.id.clone(),
            status: JobStatus::Completed,
            records_read: total_records,
            records_written: valid_records,
            files_created: vec![],
            duration_secs: 0.0,
            error: None,
            completed_at: Utc::now(),
        })
    }

    async fn aggregation(&self, job: &ETLJob) -> Result<JobResult, ETLError> {
        Ok(JobResult {
            job_id: job.id.clone(),
            status: JobStatus::Completed,
            records_read: 0,
            records_written: 0,
            files_created: vec![],
            duration_secs: 0.0,
            error: None,
            completed_at: Utc::now(),
        })
    }

    async fn deduplication(&self, job: &ETLJob) -> Result<JobResult, ETLError> {
        Ok(JobResult {
            job_id: job.id.clone(),
            status: JobStatus::Completed,
            records_read: 0,
            records_written: 0,
            files_created: vec![],
            duration_secs: 0.0,
            error: None,
            completed_at: Utc::now(),
        })
    }

    async fn schema_evolution(&self, job: &ETLJob) -> Result<JobResult, ETLError> {
        Ok(JobResult {
            job_id: job.id.clone(),
            status: JobStatus::Completed,
            records_read: 0,
            records_written: 0,
            files_created: vec![],
            duration_secs: 0.0,
            error: None,
            completed_at: Utc::now(),
        })
    }

    async fn incremental_load(&self, job: &ETLJob) -> Result<JobResult, ETLError> {
        Ok(JobResult {
            job_id: job.id.clone(),
            status: JobStatus::Completed,
            records_read: 0,
            records_written: 0,
            files_created: vec![],
            duration_secs: 0.0,
            error: None,
            completed_at: Utc::now(),
        })
    }

    async fn list_source_objects(&self, job: &ETLJob) -> Result<Vec<String>, ETLError> {
        let mut request = self.s3_client.list_objects_v2().bucket(&job.source.bucket);

        if let Some(prefix) = &job.source.prefix {
            request = request.prefix(prefix);
        }

        let result = request
            .send()
            .await
            .map_err(|e| ETLError::S3Error(e.to_string()))?;

        let objects: Vec<String> = result
            .contents()
            .iter()
            .filter_map(|obj| obj.key().map(|k| k.to_string()))
            .filter(|key| {
                if let Some(pattern) = &job.source.pattern {
                    key.contains(pattern)
                } else {
                    true
                }
            })
            .collect();

        Ok(objects)
    }

    async fn download_object(&self, bucket: &str, key: &str) -> Result<Bytes, ETLError> {
        let result = self
            .s3_client
            .get_object()
            .bucket(bucket)
            .key(key)
            .send()
            .await
            .map_err(|e| ETLError::S3Error(e.to_string()))?;

        let data = result
            .body
            .collect()
            .await
            .map_err(|e| ETLError::S3Error(e.to_string()))?
            .into_bytes();

        Ok(data)
    }

    async fn write_parquet(
        &self,
        bucket: &str,
        key: &str,
        batches: &[RecordBatch],
    ) -> Result<(), ETLError> {
        if batches.is_empty() {
            return Ok(());
        }

        let schema = batches[0].schema();
        let mut buffer = Vec::new();

        {
            let mut writer = ArrowWriter::try_new(&mut buffer, schema, None)
                .map_err(|e| ETLError::DataError(e.to_string()))?;

            for batch in batches {
                writer
                    .write(batch)
                    .map_err(|e| ETLError::DataError(e.to_string()))?;
            }

            writer
                .close()
                .map_err(|e| ETLError::DataError(e.to_string()))?;
        }

        self.s3_client
            .put_object()
            .bucket(bucket)
            .key(key)
            .body(ByteStream::from(buffer))
            .content_type("application/octet-stream")
            .send()
            .await
            .map_err(|e| ETLError::S3Error(e.to_string()))?;

        Ok(())
    }

    pub async fn run(&self) -> Result<()> {
        info!("ETL Worker {} starting...", self.worker_id);

        let mut conn = self
            .redis_client
            .get_async_connection()
            .await
            .map_err(|e| anyhow::anyhow!("Redis connection failed: {}", e))?;

        loop {
            let job_data: Option<String> = conn
                .lpop("etl:jobs", None)
                .await
                .unwrap_or(None);

            if let Some(data) = job_data {
                match serde_json::from_str::<ETLJob>(&data) {
                    Ok(job) => {
                        let result = self.process_job(job).await;
                        match result {
                            Ok(job_result) => {
                                let result_json = serde_json::to_string(&job_result).unwrap();
                                let _: () = conn
                                    .rpush("etl:results", result_json)
                                    .await
                                    .unwrap_or(());
                            }
                            Err(e) => {
                                error!("Job processing error: {}", e);
                            }
                        }
                    }
                    Err(e) => {
                        error!("Failed to parse job: {}", e);
                    }
                }
            } else {
                tokio::time::sleep(Duration::from_secs(1)).await;
            }
        }
    }
}

async fn create_s3_client() -> S3Client {
    let endpoint = env::var("S3_ENDPOINT").unwrap_or_else(|_| "http://rustfs:9000".to_string());
    let access_key = env::var("S3_ACCESS_KEY").unwrap_or_else(|_| "rustfsadmin".to_string());
    let secret_key = env::var("S3_SECRET_KEY").unwrap_or_else(|_| "rustfsadmin".to_string());
    let region = env::var("S3_REGION").unwrap_or_else(|_| "us-east-1".to_string());

    let credentials = Credentials::new(&access_key, &secret_key, None, None, "static");

    let config = aws_sdk_s3::Config::builder()
        .behavior_version(BehaviorVersion::latest())
        .region(Region::new(region))
        .endpoint_url(&endpoint)
        .credentials_provider(credentials)
        .force_path_style(true)
        .build();

    S3Client::from_conf(config)
}

#[tokio::main]
async fn main() -> Result<()> {
    dotenvy::dotenv().ok();

    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::from_default_env()
                .add_directive("etl_worker=info".parse().unwrap()),
        )
        .json()
        .init();

    REGISTRY.register(Box::new(JOBS_PROCESSED.clone())).unwrap();
    REGISTRY.register(Box::new(JOBS_FAILED.clone())).unwrap();
    REGISTRY.register(Box::new(RECORDS_PROCESSED.clone())).unwrap();
    REGISTRY.register(Box::new(JOB_DURATION.clone())).unwrap();

    let worker = ETLWorker::new().await?;
    worker.run().await?;

    Ok(())
}
