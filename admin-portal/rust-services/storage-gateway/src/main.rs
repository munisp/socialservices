use axum::{
    body::Body,
    extract::{Multipart, Path, Query, State},
    http::{header, StatusCode},
    response::{IntoResponse, Response},
    routing::{delete, get, post, put},
    Json, Router,
};
use aws_config::BehaviorVersion;
use aws_sdk_s3::{
    config::{Credentials, Region},
    primitives::ByteStream,
    Client as S3Client,
};
use bytes::Bytes;
use chrono::{DateTime, Utc};
use futures::StreamExt;
use lazy_static::lazy_static;
use prometheus::{Counter, Histogram, Registry};
use serde::{Deserialize, Serialize};
use std::{collections::HashMap, env, sync::Arc, time::Duration};
use thiserror::Error;
use tokio::sync::RwLock;
use tower_http::{
    compression::CompressionLayer,
    cors::{Any, CorsLayer},
    trace::TraceLayer,
};
use tracing::{error, info, instrument, warn};
use uuid::Uuid;

mod config;
mod handlers;
mod metrics;
mod middleware;

lazy_static! {
    static ref REGISTRY: Registry = Registry::new();
    static ref UPLOAD_COUNTER: Counter = Counter::new("storage_uploads_total", "Total uploads")
        .expect("metric can be created");
    static ref DOWNLOAD_COUNTER: Counter = Counter::new("storage_downloads_total", "Total downloads")
        .expect("metric can be created");
    static ref UPLOAD_DURATION: Histogram = Histogram::with_opts(
        prometheus::HistogramOpts::new("storage_upload_duration_seconds", "Upload duration")
    ).expect("metric can be created");
    static ref DOWNLOAD_DURATION: Histogram = Histogram::with_opts(
        prometheus::HistogramOpts::new("storage_download_duration_seconds", "Download duration")
    ).expect("metric can be created");
}

#[derive(Error, Debug)]
pub enum StorageError {
    #[error("S3 error: {0}")]
    S3Error(String),
    #[error("Object not found: {0}")]
    NotFound(String),
    #[error("Invalid request: {0}")]
    InvalidRequest(String),
    #[error("Internal error: {0}")]
    Internal(String),
}

impl IntoResponse for StorageError {
    fn into_response(self) -> Response {
        let (status, message) = match self {
            StorageError::S3Error(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg),
            StorageError::NotFound(msg) => (StatusCode::NOT_FOUND, msg),
            StorageError::InvalidRequest(msg) => (StatusCode::BAD_REQUEST, msg),
            StorageError::Internal(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg),
        };

        let body = Json(serde_json::json!({
            "error": message,
            "status": status.as_u16()
        }));

        (status, body).into_response()
    }
}

#[derive(Clone)]
pub struct AppState {
    s3_client: S3Client,
    default_bucket: String,
    presigned_url_expiry: Duration,
    max_upload_size: u64,
    upload_cache: Arc<RwLock<HashMap<String, MultipartUploadState>>>,
}

#[derive(Clone, Debug)]
struct MultipartUploadState {
    upload_id: String,
    bucket: String,
    key: String,
    parts: Vec<CompletedPart>,
    created_at: DateTime<Utc>,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
struct CompletedPart {
    part_number: i32,
    e_tag: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UploadResponse {
    pub key: String,
    pub bucket: String,
    pub e_tag: Option<String>,
    pub version_id: Option<String>,
    pub size: u64,
    pub content_type: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ListObjectsResponse {
    pub objects: Vec<ObjectInfo>,
    pub continuation_token: Option<String>,
    pub is_truncated: bool,
    pub prefix: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ObjectInfo {
    pub key: String,
    pub size: i64,
    pub last_modified: Option<DateTime<Utc>>,
    pub e_tag: Option<String>,
    pub storage_class: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct ListObjectsQuery {
    pub prefix: Option<String>,
    pub continuation_token: Option<String>,
    pub max_keys: Option<i32>,
    pub delimiter: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct PresignedUrlQuery {
    pub expires_in: Option<u64>,
    pub content_type: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct PresignedUrlResponse {
    pub url: String,
    pub expires_at: DateTime<Utc>,
    pub method: String,
}

#[derive(Debug, Serialize)]
pub struct HealthResponse {
    pub status: String,
    pub service: String,
    pub version: String,
    pub storage_backend: String,
    pub timestamp: DateTime<Utc>,
}

#[derive(Debug, Serialize)]
pub struct BucketInfo {
    pub name: String,
    pub creation_date: Option<DateTime<Utc>>,
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

#[instrument(skip(state))]
async fn health_check(State(state): State<AppState>) -> Json<HealthResponse> {
    let storage_backend = env::var("S3_ENDPOINT").unwrap_or_else(|_| "rustfs:9000".to_string());
    
    Json(HealthResponse {
        status: "healthy".to_string(),
        service: "storage-gateway".to_string(),
        version: env!("CARGO_PKG_VERSION").to_string(),
        storage_backend,
        timestamp: Utc::now(),
    })
}

#[instrument(skip(state))]
async fn list_buckets(State(state): State<AppState>) -> Result<Json<Vec<BucketInfo>>, StorageError> {
    let result = state
        .s3_client
        .list_buckets()
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    let buckets: Vec<BucketInfo> = result
        .buckets()
        .iter()
        .map(|b| BucketInfo {
            name: b.name().unwrap_or_default().to_string(),
            creation_date: b.creation_date().map(|d| {
                DateTime::from_timestamp(d.secs(), d.subsec_nanos())
                    .unwrap_or_else(Utc::now)
            }),
        })
        .collect();

    Ok(Json(buckets))
}

#[instrument(skip(state))]
async fn create_bucket(
    State(state): State<AppState>,
    Path(bucket): Path<String>,
) -> Result<StatusCode, StorageError> {
    state
        .s3_client
        .create_bucket()
        .bucket(&bucket)
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    info!("Created bucket: {}", bucket);
    Ok(StatusCode::CREATED)
}

#[instrument(skip(state))]
async fn delete_bucket(
    State(state): State<AppState>,
    Path(bucket): Path<String>,
) -> Result<StatusCode, StorageError> {
    state
        .s3_client
        .delete_bucket()
        .bucket(&bucket)
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    info!("Deleted bucket: {}", bucket);
    Ok(StatusCode::NO_CONTENT)
}

#[instrument(skip(state))]
async fn list_objects(
    State(state): State<AppState>,
    Path(bucket): Path<String>,
    Query(query): Query<ListObjectsQuery>,
) -> Result<Json<ListObjectsResponse>, StorageError> {
    let mut request = state.s3_client.list_objects_v2().bucket(&bucket);

    if let Some(prefix) = &query.prefix {
        request = request.prefix(prefix);
    }
    if let Some(token) = &query.continuation_token {
        request = request.continuation_token(token);
    }
    if let Some(max_keys) = query.max_keys {
        request = request.max_keys(max_keys);
    }
    if let Some(delimiter) = &query.delimiter {
        request = request.delimiter(delimiter);
    }

    let result = request
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    let objects: Vec<ObjectInfo> = result
        .contents()
        .iter()
        .map(|obj| ObjectInfo {
            key: obj.key().unwrap_or_default().to_string(),
            size: obj.size().unwrap_or(0),
            last_modified: obj.last_modified().map(|d| {
                DateTime::from_timestamp(d.secs(), d.subsec_nanos())
                    .unwrap_or_else(Utc::now)
            }),
            e_tag: obj.e_tag().map(|s| s.to_string()),
            storage_class: obj.storage_class().map(|s| s.as_str().to_string()),
        })
        .collect();

    Ok(Json(ListObjectsResponse {
        objects,
        continuation_token: result.next_continuation_token().map(|s| s.to_string()),
        is_truncated: result.is_truncated().unwrap_or(false),
        prefix: query.prefix,
    }))
}

#[instrument(skip(state, body))]
async fn upload_object(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
    body: Bytes,
) -> Result<Json<UploadResponse>, StorageError> {
    let _timer = UPLOAD_DURATION.start_timer();
    UPLOAD_COUNTER.inc();

    let content_type = mime_guess::from_path(&key)
        .first_or_octet_stream()
        .to_string();

    let size = body.len() as u64;

    let result = state
        .s3_client
        .put_object()
        .bucket(&bucket)
        .key(&key)
        .body(ByteStream::from(body))
        .content_type(&content_type)
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    info!("Uploaded object: {}/{} ({} bytes)", bucket, key, size);

    Ok(Json(UploadResponse {
        key,
        bucket,
        e_tag: result.e_tag().map(|s| s.to_string()),
        version_id: result.version_id().map(|s| s.to_string()),
        size,
        content_type,
    }))
}

#[instrument(skip(state, multipart))]
async fn upload_multipart(
    State(state): State<AppState>,
    Path(bucket): Path<String>,
    mut multipart: Multipart,
) -> Result<Json<Vec<UploadResponse>>, StorageError> {
    let mut responses = Vec::new();

    while let Some(field) = multipart
        .next_field()
        .await
        .map_err(|e| StorageError::InvalidRequest(e.to_string()))?
    {
        let file_name = field
            .file_name()
            .map(|s| s.to_string())
            .unwrap_or_else(|| Uuid::new_v4().to_string());

        let content_type = field
            .content_type()
            .map(|s| s.to_string())
            .unwrap_or_else(|| "application/octet-stream".to_string());

        let data = field
            .bytes()
            .await
            .map_err(|e| StorageError::InvalidRequest(e.to_string()))?;

        let size = data.len() as u64;

        let result = state
            .s3_client
            .put_object()
            .bucket(&bucket)
            .key(&file_name)
            .body(ByteStream::from(data))
            .content_type(&content_type)
            .send()
            .await
            .map_err(|e| StorageError::S3Error(e.to_string()))?;

        responses.push(UploadResponse {
            key: file_name,
            bucket: bucket.clone(),
            e_tag: result.e_tag().map(|s| s.to_string()),
            version_id: result.version_id().map(|s| s.to_string()),
            size,
            content_type,
        });
    }

    Ok(Json(responses))
}

#[instrument(skip(state))]
async fn download_object(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
) -> Result<Response, StorageError> {
    let _timer = DOWNLOAD_DURATION.start_timer();
    DOWNLOAD_COUNTER.inc();

    let result = state
        .s3_client
        .get_object()
        .bucket(&bucket)
        .key(&key)
        .send()
        .await
        .map_err(|e| {
            if e.to_string().contains("NoSuchKey") {
                StorageError::NotFound(format!("{}/{}", bucket, key))
            } else {
                StorageError::S3Error(e.to_string())
            }
        })?;

    let content_type = result
        .content_type()
        .unwrap_or("application/octet-stream")
        .to_string();

    let body = result
        .body
        .collect()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?
        .into_bytes();

    info!("Downloaded object: {}/{} ({} bytes)", bucket, key, body.len());

    Ok(Response::builder()
        .status(StatusCode::OK)
        .header(header::CONTENT_TYPE, content_type)
        .header(header::CONTENT_LENGTH, body.len())
        .body(Body::from(body))
        .unwrap())
}

#[instrument(skip(state))]
async fn delete_object(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
) -> Result<StatusCode, StorageError> {
    state
        .s3_client
        .delete_object()
        .bucket(&bucket)
        .key(&key)
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    info!("Deleted object: {}/{}", bucket, key);
    Ok(StatusCode::NO_CONTENT)
}

#[instrument(skip(state))]
async fn head_object(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
) -> Result<Response, StorageError> {
    let result = state
        .s3_client
        .head_object()
        .bucket(&bucket)
        .key(&key)
        .send()
        .await
        .map_err(|e| {
            if e.to_string().contains("NotFound") {
                StorageError::NotFound(format!("{}/{}", bucket, key))
            } else {
                StorageError::S3Error(e.to_string())
            }
        })?;

    let content_type = result
        .content_type()
        .unwrap_or("application/octet-stream")
        .to_string();

    let content_length = result.content_length().unwrap_or(0);

    Ok(Response::builder()
        .status(StatusCode::OK)
        .header(header::CONTENT_TYPE, content_type)
        .header(header::CONTENT_LENGTH, content_length)
        .body(Body::empty())
        .unwrap())
}

#[instrument(skip(state))]
async fn copy_object(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<Json<UploadResponse>, StorageError> {
    let source = params
        .get("source")
        .ok_or_else(|| StorageError::InvalidRequest("Missing source parameter".to_string()))?;

    let result = state
        .s3_client
        .copy_object()
        .bucket(&bucket)
        .key(&key)
        .copy_source(source)
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    info!("Copied object from {} to {}/{}", source, bucket, key);

    Ok(Json(UploadResponse {
        key,
        bucket,
        e_tag: result.copy_object_result().and_then(|r| r.e_tag().map(|s| s.to_string())),
        version_id: result.version_id().map(|s| s.to_string()),
        size: 0,
        content_type: "application/octet-stream".to_string(),
    }))
}

async fn metrics_handler() -> String {
    use prometheus::Encoder;
    let encoder = prometheus::TextEncoder::new();
    let metric_families = REGISTRY.gather();
    let mut buffer = Vec::new();
    encoder.encode(&metric_families, &mut buffer).unwrap();
    String::from_utf8(buffer).unwrap()
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    dotenvy::dotenv().ok();

    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::from_default_env()
                .add_directive("storage_gateway=info".parse().unwrap()),
        )
        .json()
        .init();

    REGISTRY.register(Box::new(UPLOAD_COUNTER.clone())).unwrap();
    REGISTRY.register(Box::new(DOWNLOAD_COUNTER.clone())).unwrap();
    REGISTRY.register(Box::new(UPLOAD_DURATION.clone())).unwrap();
    REGISTRY.register(Box::new(DOWNLOAD_DURATION.clone())).unwrap();

    let s3_client = create_s3_client().await;
    let default_bucket = env::var("DEFAULT_BUCKET").unwrap_or_else(|_| "documents".to_string());
    let presigned_expiry = env::var("PRESIGNED_URL_EXPIRY_SECS")
        .unwrap_or_else(|_| "3600".to_string())
        .parse::<u64>()
        .unwrap_or(3600);
    let max_upload_size = env::var("MAX_UPLOAD_SIZE_MB")
        .unwrap_or_else(|_| "100".to_string())
        .parse::<u64>()
        .unwrap_or(100)
        * 1024
        * 1024;

    let state = AppState {
        s3_client,
        default_bucket,
        presigned_url_expiry: Duration::from_secs(presigned_expiry),
        max_upload_size,
        upload_cache: Arc::new(RwLock::new(HashMap::new())),
    };

    let cors = CorsLayer::new()
        .allow_origin(Any)
        .allow_methods(Any)
        .allow_headers(Any);

    let app = Router::new()
        .route("/health", get(health_check))
        .route("/metrics", get(metrics_handler))
        .route("/buckets", get(list_buckets))
        .route("/buckets/:bucket", post(create_bucket).delete(delete_bucket))
        .route("/buckets/:bucket/objects", get(list_objects))
        .route("/buckets/:bucket/upload", post(upload_multipart))
        .route(
            "/buckets/:bucket/objects/*key",
            get(download_object)
                .put(upload_object)
                .delete(delete_object)
                .head(head_object),
        )
        .route("/buckets/:bucket/copy/*key", post(copy_object))
        .layer(cors)
        .layer(CompressionLayer::new())
        .layer(TraceLayer::new_for_http())
        .with_state(state);

    let port = env::var("PORT").unwrap_or_else(|_| "8090".to_string());
    let addr = format!("0.0.0.0:{}", port);

    info!("Starting storage gateway on {}", addr);
    info!("S3 endpoint: {}", env::var("S3_ENDPOINT").unwrap_or_else(|_| "http://rustfs:9000".to_string()));

    let listener = tokio::net::TcpListener::bind(&addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}
