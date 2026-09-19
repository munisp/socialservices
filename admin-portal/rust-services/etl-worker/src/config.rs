use serde::Deserialize;
use std::env;

#[derive(Debug, Clone, Deserialize)]
pub struct Config {
    pub worker: WorkerConfig,
    pub s3: S3Config,
    pub kafka: KafkaConfig,
    pub redis: RedisConfig,
}

#[derive(Debug, Clone, Deserialize)]
pub struct WorkerConfig {
    pub id: String,
    pub concurrency: usize,
    pub poll_interval_ms: u64,
    pub job_timeout_secs: u64,
}

#[derive(Debug, Clone, Deserialize)]
pub struct S3Config {
    pub endpoint: String,
    pub access_key: String,
    pub secret_key: String,
    pub region: String,
}

#[derive(Debug, Clone, Deserialize)]
pub struct KafkaConfig {
    pub brokers: Option<String>,
    pub group_id: String,
    pub topics: Vec<String>,
}

#[derive(Debug, Clone, Deserialize)]
pub struct RedisConfig {
    pub url: String,
    pub job_queue: String,
    pub result_queue: String,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            worker: WorkerConfig::default(),
            s3: S3Config::default(),
            kafka: KafkaConfig::default(),
            redis: RedisConfig::default(),
        }
    }
}

impl Default for WorkerConfig {
    fn default() -> Self {
        Self {
            id: env::var("WORKER_ID").unwrap_or_else(|_| uuid::Uuid::new_v4().to_string()),
            concurrency: env::var("WORKER_CONCURRENCY")
                .unwrap_or_else(|_| "4".to_string())
                .parse()
                .unwrap_or(4),
            poll_interval_ms: env::var("POLL_INTERVAL_MS")
                .unwrap_or_else(|_| "1000".to_string())
                .parse()
                .unwrap_or(1000),
            job_timeout_secs: env::var("JOB_TIMEOUT_SECS")
                .unwrap_or_else(|_| "3600".to_string())
                .parse()
                .unwrap_or(3600),
        }
    }
}

impl Default for S3Config {
    fn default() -> Self {
        Self {
            endpoint: env::var("S3_ENDPOINT").unwrap_or_else(|_| "http://rustfs:9000".to_string()),
            access_key: env::var("S3_ACCESS_KEY").unwrap_or_else(|_| "rustfsadmin".to_string()),
            secret_key: env::var("S3_SECRET_KEY").unwrap_or_else(|_| "rustfsadmin".to_string()),
            region: env::var("S3_REGION").unwrap_or_else(|_| "us-east-1".to_string()),
        }
    }
}

impl Default for KafkaConfig {
    fn default() -> Self {
        Self {
            brokers: env::var("KAFKA_BROKERS").ok(),
            group_id: env::var("KAFKA_GROUP_ID").unwrap_or_else(|_| "etl-worker".to_string()),
            topics: env::var("KAFKA_TOPICS")
                .unwrap_or_else(|_| "etl-jobs".to_string())
                .split(',')
                .map(|s| s.to_string())
                .collect(),
        }
    }
}

impl Default for RedisConfig {
    fn default() -> Self {
        Self {
            url: env::var("REDIS_URL").unwrap_or_else(|_| "redis://localhost:6379".to_string()),
            job_queue: env::var("REDIS_JOB_QUEUE").unwrap_or_else(|_| "etl:jobs".to_string()),
            result_queue: env::var("REDIS_RESULT_QUEUE").unwrap_or_else(|_| "etl:results".to_string()),
        }
    }
}
