use serde::Deserialize;
use std::env;

#[derive(Debug, Clone, Deserialize)]
pub struct Config {
    pub server: ServerConfig,
    pub s3: S3Config,
    pub storage: StorageConfig,
}

#[derive(Debug, Clone, Deserialize)]
pub struct ServerConfig {
    pub host: String,
    pub port: u16,
    pub workers: usize,
}

#[derive(Debug, Clone, Deserialize)]
pub struct S3Config {
    pub endpoint: String,
    pub access_key: String,
    pub secret_key: String,
    pub region: String,
    pub force_path_style: bool,
}

#[derive(Debug, Clone, Deserialize)]
pub struct StorageConfig {
    pub default_bucket: String,
    pub max_upload_size_mb: u64,
    pub presigned_url_expiry_secs: u64,
    pub multipart_threshold_mb: u64,
    pub chunk_size_mb: u64,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            server: ServerConfig::default(),
            s3: S3Config::default(),
            storage: StorageConfig::default(),
        }
    }
}

impl Default for ServerConfig {
    fn default() -> Self {
        Self {
            host: env::var("HOST").unwrap_or_else(|_| "0.0.0.0".to_string()),
            port: env::var("PORT")
                .unwrap_or_else(|_| "8090".to_string())
                .parse()
                .unwrap_or(8090),
            workers: env::var("WORKERS")
                .unwrap_or_else(|_| "4".to_string())
                .parse()
                .unwrap_or(4),
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
            force_path_style: env::var("S3_FORCE_PATH_STYLE")
                .unwrap_or_else(|_| "true".to_string())
                .parse()
                .unwrap_or(true),
        }
    }
}

impl Default for StorageConfig {
    fn default() -> Self {
        Self {
            default_bucket: env::var("DEFAULT_BUCKET").unwrap_or_else(|_| "documents".to_string()),
            max_upload_size_mb: env::var("MAX_UPLOAD_SIZE_MB")
                .unwrap_or_else(|_| "100".to_string())
                .parse()
                .unwrap_or(100),
            presigned_url_expiry_secs: env::var("PRESIGNED_URL_EXPIRY_SECS")
                .unwrap_or_else(|_| "3600".to_string())
                .parse()
                .unwrap_or(3600),
            multipart_threshold_mb: env::var("MULTIPART_THRESHOLD_MB")
                .unwrap_or_else(|_| "100".to_string())
                .parse()
                .unwrap_or(100),
            chunk_size_mb: env::var("CHUNK_SIZE_MB")
                .unwrap_or_else(|_| "10".to_string())
                .parse()
                .unwrap_or(10),
        }
    }
}

impl Config {
    pub fn from_env() -> Self {
        Self::default()
    }
}
