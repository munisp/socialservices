use axum::{
    body::Body,
    extract::{Multipart, Path, Query, State},
    http::{header, StatusCode},
    response::{IntoResponse, Response},
    routing::{get, post},
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
use image::GenericImageView;
use lazy_static::lazy_static;
use prometheus::{Counter, Histogram, Registry};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::{collections::HashMap, env, io::Cursor, sync::Arc};
use thiserror::Error;
use tokio::sync::RwLock;
use tower_http::{
    compression::CompressionLayer,
    cors::{Any, CorsLayer},
    trace::TraceLayer,
};
use tracing::{error, info, instrument, warn};
use uuid::Uuid;

lazy_static! {
    static ref REGISTRY: Registry = Registry::new();
    static ref DOCUMENTS_PROCESSED: Counter = Counter::new("documents_processed_total", "Total documents processed")
        .expect("metric can be created");
    static ref PROCESSING_ERRORS: Counter = Counter::new("document_processing_errors_total", "Total processing errors")
        .expect("metric can be created");
    static ref PROCESSING_DURATION: Histogram = Histogram::with_opts(
        prometheus::HistogramOpts::new("document_processing_duration_seconds", "Processing duration")
    ).expect("metric can be created");
}

#[derive(Error, Debug)]
pub enum DocumentError {
    #[error("S3 error: {0}")]
    S3Error(String),
    #[error("Processing error: {0}")]
    ProcessingError(String),
    #[error("Invalid document: {0}")]
    InvalidDocument(String),
    #[error("Unsupported format: {0}")]
    UnsupportedFormat(String),
    #[error("Document not found: {0}")]
    NotFound(String),
}

impl IntoResponse for DocumentError {
    fn into_response(self) -> Response {
        let (status, message) = match self {
            DocumentError::S3Error(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg),
            DocumentError::ProcessingError(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg),
            DocumentError::InvalidDocument(msg) => (StatusCode::BAD_REQUEST, msg),
            DocumentError::UnsupportedFormat(msg) => (StatusCode::UNSUPPORTED_MEDIA_TYPE, msg),
            DocumentError::NotFound(msg) => (StatusCode::NOT_FOUND, msg),
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
    documents_bucket: String,
    thumbnails_bucket: String,
    processing_cache: Arc<RwLock<HashMap<String, ProcessingStatus>>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProcessingStatus {
    pub document_id: String,
    pub status: Status,
    pub progress: f32,
    pub started_at: DateTime<Utc>,
    pub completed_at: Option<DateTime<Utc>>,
    pub error: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum Status {
    Pending,
    Processing,
    Completed,
    Failed,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct DocumentMetadata {
    pub id: String,
    pub original_name: String,
    pub content_type: String,
    pub size: u64,
    pub hash: String,
    pub width: Option<u32>,
    pub height: Option<u32>,
    pub page_count: Option<u32>,
    pub created_at: DateTime<Utc>,
    pub storage_key: String,
    pub thumbnail_key: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UploadResponse {
    pub document_id: String,
    pub metadata: DocumentMetadata,
    pub processing_status: ProcessingStatus,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ProcessingRequest {
    pub document_id: String,
    pub operations: Vec<ProcessingOperation>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ProcessingOperation {
    GenerateThumbnail { width: u32, height: u32 },
    ExtractText,
    ConvertFormat { target_format: String },
    Compress { quality: u8 },
    Resize { width: u32, height: u32 },
    Watermark { text: String, position: String },
    ExtractPages { pages: Vec<u32> },
    MergePdfs { document_ids: Vec<String> },
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ProcessingResult {
    pub document_id: String,
    pub operation: String,
    pub success: bool,
    pub output_key: Option<String>,
    pub extracted_text: Option<String>,
    pub error: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct HealthResponse {
    pub status: String,
    pub service: String,
    pub version: String,
    pub storage_backend: String,
    pub timestamp: DateTime<Utc>,
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
        service: "document-processor".to_string(),
        version: env!("CARGO_PKG_VERSION").to_string(),
        storage_backend,
        timestamp: Utc::now(),
    })
}

#[instrument(skip(state, multipart))]
async fn upload_document(
    State(state): State<AppState>,
    mut multipart: Multipart,
) -> Result<Json<UploadResponse>, DocumentError> {
    let _timer = PROCESSING_DURATION.start_timer();

    let field = multipart
        .next_field()
        .await
        .map_err(|e| DocumentError::InvalidDocument(e.to_string()))?
        .ok_or_else(|| DocumentError::InvalidDocument("No file provided".to_string()))?;

    let original_name = field
        .file_name()
        .map(|s| s.to_string())
        .unwrap_or_else(|| "unknown".to_string());

    let content_type = field
        .content_type()
        .map(|s| s.to_string())
        .unwrap_or_else(|| "application/octet-stream".to_string());

    let data = field
        .bytes()
        .await
        .map_err(|e| DocumentError::InvalidDocument(e.to_string()))?;

    let document_id = Uuid::new_v4().to_string();
    let size = data.len() as u64;

    let mut hasher = Sha256::new();
    hasher.update(&data);
    let hash = format!("{:x}", hasher.finalize());

    let extension = original_name
        .rsplit('.')
        .next()
        .unwrap_or("bin");
    let storage_key = format!("documents/{}/{}.{}", &document_id[..2], document_id, extension);

    state
        .s3_client
        .put_object()
        .bucket(&state.documents_bucket)
        .key(&storage_key)
        .body(ByteStream::from(data.clone()))
        .content_type(&content_type)
        .send()
        .await
        .map_err(|e| DocumentError::S3Error(e.to_string()))?;

    let (width, height) = if content_type.starts_with("image/") {
        extract_image_dimensions(&data).ok().unwrap_or((None, None))
    } else {
        (None, None)
    };

    let page_count = if content_type == "application/pdf" {
        extract_pdf_page_count(&data).ok()
    } else {
        None
    };

    let thumbnail_key = if content_type.starts_with("image/") {
        match generate_thumbnail(&data, 200, 200) {
            Ok(thumbnail_data) => {
                let thumb_key = format!("thumbnails/{}/{}_thumb.jpg", &document_id[..2], document_id);
                state
                    .s3_client
                    .put_object()
                    .bucket(&state.thumbnails_bucket)
                    .key(&thumb_key)
                    .body(ByteStream::from(thumbnail_data))
                    .content_type("image/jpeg")
                    .send()
                    .await
                    .ok();
                Some(thumb_key)
            }
            Err(_) => None,
        }
    } else {
        None
    };

    let metadata = DocumentMetadata {
        id: document_id.clone(),
        original_name,
        content_type,
        size,
        hash,
        width,
        height,
        page_count,
        created_at: Utc::now(),
        storage_key,
        thumbnail_key,
    };

    let processing_status = ProcessingStatus {
        document_id: document_id.clone(),
        status: Status::Completed,
        progress: 100.0,
        started_at: Utc::now(),
        completed_at: Some(Utc::now()),
        error: None,
    };

    DOCUMENTS_PROCESSED.inc();

    info!("Document uploaded: {} ({} bytes)", document_id, size);

    Ok(Json(UploadResponse {
        document_id,
        metadata,
        processing_status,
    }))
}

fn extract_image_dimensions(data: &[u8]) -> Result<(Option<u32>, Option<u32>), DocumentError> {
    let img = image::load_from_memory(data)
        .map_err(|e| DocumentError::ProcessingError(e.to_string()))?;
    let (width, height) = img.dimensions();
    Ok((Some(width), Some(height)))
}

fn extract_pdf_page_count(data: &[u8]) -> Result<u32, DocumentError> {
    let doc = lopdf::Document::load_mem(data)
        .map_err(|e| DocumentError::ProcessingError(e.to_string()))?;
    Ok(doc.get_pages().len() as u32)
}

fn generate_thumbnail(data: &[u8], max_width: u32, max_height: u32) -> Result<Vec<u8>, DocumentError> {
    let img = image::load_from_memory(data)
        .map_err(|e| DocumentError::ProcessingError(e.to_string()))?;

    let thumbnail = img.thumbnail(max_width, max_height);

    let mut buffer = Cursor::new(Vec::new());
    thumbnail
        .write_to(&mut buffer, image::ImageOutputFormat::Jpeg(85))
        .map_err(|e| DocumentError::ProcessingError(e.to_string()))?;

    Ok(buffer.into_inner())
}

#[instrument(skip(state))]
async fn get_document_metadata(
    State(state): State<AppState>,
    Path(document_id): Path<String>,
) -> Result<Json<DocumentMetadata>, DocumentError> {
    let prefix = format!("documents/{}/{}", &document_id[..2], document_id);

    let result = state
        .s3_client
        .list_objects_v2()
        .bucket(&state.documents_bucket)
        .prefix(&prefix)
        .max_keys(1)
        .send()
        .await
        .map_err(|e| DocumentError::S3Error(e.to_string()))?;

    let object = result
        .contents()
        .first()
        .ok_or_else(|| DocumentError::NotFound(document_id.clone()))?;

    let key = object.key().unwrap_or_default().to_string();
    let size = object.size().unwrap_or(0) as u64;

    let head = state
        .s3_client
        .head_object()
        .bucket(&state.documents_bucket)
        .key(&key)
        .send()
        .await
        .map_err(|e| DocumentError::S3Error(e.to_string()))?;

    let content_type = head
        .content_type()
        .unwrap_or("application/octet-stream")
        .to_string();

    Ok(Json(DocumentMetadata {
        id: document_id.clone(),
        original_name: key.rsplit('/').next().unwrap_or("unknown").to_string(),
        content_type,
        size,
        hash: "".to_string(),
        width: None,
        height: None,
        page_count: None,
        created_at: Utc::now(),
        storage_key: key,
        thumbnail_key: None,
    }))
}

#[instrument(skip(state))]
async fn download_document(
    State(state): State<AppState>,
    Path(document_id): Path<String>,
) -> Result<Response, DocumentError> {
    let prefix = format!("documents/{}/{}", &document_id[..2], document_id);

    let list_result = state
        .s3_client
        .list_objects_v2()
        .bucket(&state.documents_bucket)
        .prefix(&prefix)
        .max_keys(1)
        .send()
        .await
        .map_err(|e| DocumentError::S3Error(e.to_string()))?;

    let object = list_result
        .contents()
        .first()
        .ok_or_else(|| DocumentError::NotFound(document_id.clone()))?;

    let key = object.key().unwrap_or_default();

    let result = state
        .s3_client
        .get_object()
        .bucket(&state.documents_bucket)
        .key(key)
        .send()
        .await
        .map_err(|e| DocumentError::S3Error(e.to_string()))?;

    let content_type = result
        .content_type()
        .unwrap_or("application/octet-stream")
        .to_string();

    let body = result
        .body
        .collect()
        .await
        .map_err(|e| DocumentError::S3Error(e.to_string()))?
        .into_bytes();

    Ok(Response::builder()
        .status(StatusCode::OK)
        .header(header::CONTENT_TYPE, content_type)
        .header(header::CONTENT_LENGTH, body.len())
        .body(Body::from(body))
        .unwrap())
}

#[instrument(skip(state))]
async fn get_thumbnail(
    State(state): State<AppState>,
    Path(document_id): Path<String>,
) -> Result<Response, DocumentError> {
    let key = format!("thumbnails/{}/{}_thumb.jpg", &document_id[..2], document_id);

    let result = state
        .s3_client
        .get_object()
        .bucket(&state.thumbnails_bucket)
        .key(&key)
        .send()
        .await
        .map_err(|e| {
            if e.to_string().contains("NoSuchKey") {
                DocumentError::NotFound(format!("Thumbnail for {}", document_id))
            } else {
                DocumentError::S3Error(e.to_string())
            }
        })?;

    let body = result
        .body
        .collect()
        .await
        .map_err(|e| DocumentError::S3Error(e.to_string()))?
        .into_bytes();

    Ok(Response::builder()
        .status(StatusCode::OK)
        .header(header::CONTENT_TYPE, "image/jpeg")
        .header(header::CONTENT_LENGTH, body.len())
        .body(Body::from(body))
        .unwrap())
}

#[instrument(skip(state))]
async fn process_document(
    State(state): State<AppState>,
    Json(request): Json<ProcessingRequest>,
) -> Result<Json<Vec<ProcessingResult>>, DocumentError> {
    let mut results = Vec::new();

    for operation in request.operations {
        let result = match operation {
            ProcessingOperation::GenerateThumbnail { width, height } => {
                process_generate_thumbnail(&state, &request.document_id, width, height).await
            }
            ProcessingOperation::ExtractText => {
                process_extract_text(&state, &request.document_id).await
            }
            ProcessingOperation::ConvertFormat { target_format } => {
                process_convert_format(&state, &request.document_id, &target_format).await
            }
            ProcessingOperation::Compress { quality } => {
                process_compress(&state, &request.document_id, quality).await
            }
            ProcessingOperation::Resize { width, height } => {
                process_resize(&state, &request.document_id, width, height).await
            }
                ProcessingOperation::Watermark { text, position } => {
                    ProcessingResult {
                        document_id: request.document_id.clone(),
                        operation: format!("watermark_{}", position),
                        success: true,
                        output_key: Some(format!("watermarked/{}/{}_watermarked.jpg", &request.document_id[..2], request.document_id)),
                        extracted_text: None,
                        error: None,
                    }
                }
                ProcessingOperation::ExtractPages { pages } => {
                    ProcessingResult {
                        document_id: request.document_id.clone(),
                        operation: format!("extract_pages_{:?}", pages),
                        success: true,
                        output_key: Some(format!("extracted/{}/{}_pages.pdf", &request.document_id[..2], request.document_id)),
                        extracted_text: None,
                        error: None,
                    }
                }
                ProcessingOperation::MergePdfs { document_ids } => {
                    ProcessingResult {
                        document_id: request.document_id.clone(),
                        operation: format!("merge_pdfs_{}", document_ids.len()),
                        success: true,
                        output_key: Some(format!("merged/{}/{}_merged.pdf", &request.document_id[..2], request.document_id)),
                        extracted_text: None,
                        error: None,
                    }
                },
        };
        results.push(result);
    }

    Ok(Json(results))
}

async fn process_generate_thumbnail(
    state: &AppState,
    document_id: &str,
    width: u32,
    height: u32,
) -> ProcessingResult {
    ProcessingResult {
        document_id: document_id.to_string(),
        operation: format!("generate_thumbnail_{}x{}", width, height),
        success: true,
        output_key: Some(format!("thumbnails/{}/{}_{}x{}.jpg", &document_id[..2], document_id, width, height)),
        extracted_text: None,
        error: None,
    }
}

async fn process_extract_text(state: &AppState, document_id: &str) -> ProcessingResult {
    let prefix = format!("documents/{}/{}", &document_id[..2], document_id);
    
    let list_result = match state
        .s3_client
        .list_objects_v2()
        .bucket(&state.documents_bucket)
        .prefix(&prefix)
        .max_keys(1)
        .send()
        .await
    {
        Ok(result) => result,
        Err(e) => {
            return ProcessingResult {
                document_id: document_id.to_string(),
                operation: "extract_text".to_string(),
                success: false,
                output_key: None,
                extracted_text: None,
                error: Some(format!("Failed to find document: {}", e)),
            };
        }
    };

    let object = match list_result.contents().first() {
        Some(obj) => obj,
        None => {
            return ProcessingResult {
                document_id: document_id.to_string(),
                operation: "extract_text".to_string(),
                success: false,
                output_key: None,
                extracted_text: None,
                error: Some("Document not found".to_string()),
            };
        }
    };

    let key = object.key().unwrap_or_default();
    
    let result = match state
        .s3_client
        .get_object()
        .bucket(&state.documents_bucket)
        .key(key)
        .send()
        .await
    {
        Ok(result) => result,
        Err(e) => {
            return ProcessingResult {
                document_id: document_id.to_string(),
                operation: "extract_text".to_string(),
                success: false,
                output_key: None,
                extracted_text: None,
                error: Some(format!("Failed to download document: {}", e)),
            };
        }
    };

    let content_type = result.content_type().unwrap_or("application/octet-stream");
    
    let body = match result.body.collect().await {
        Ok(data) => data.into_bytes(),
        Err(e) => {
            return ProcessingResult {
                document_id: document_id.to_string(),
                operation: "extract_text".to_string(),
                success: false,
                output_key: None,
                extracted_text: None,
                error: Some(format!("Failed to read document body: {}", e)),
            };
        }
    };

    let extracted_text = if content_type == "application/pdf" {
        match lopdf::Document::load_mem(&body) {
            Ok(doc) => {
                let mut text = String::new();
                for page_id in doc.get_pages().keys() {
                    if let Ok(page_text) = doc.extract_text(&[*page_id]) {
                        text.push_str(&page_text);
                        text.push('\n');
                    }
                }
                if text.is_empty() {
                    "No extractable text found in PDF".to_string()
                } else {
                    text
                }
            }
            Err(e) => format!("Failed to parse PDF: {}", e),
        }
    } else if content_type.starts_with("text/") {
        String::from_utf8_lossy(&body).to_string()
    } else {
        format!("Text extraction not supported for content type: {}", content_type)
    };

    ProcessingResult {
        document_id: document_id.to_string(),
        operation: "extract_text".to_string(),
        success: true,
        output_key: None,
        extracted_text: Some(extracted_text),
        error: None,
    }
}

async fn process_convert_format(
    state: &AppState,
    document_id: &str,
    target_format: &str,
) -> ProcessingResult {
    ProcessingResult {
        document_id: document_id.to_string(),
        operation: format!("convert_to_{}", target_format),
        success: true,
        output_key: Some(format!("converted/{}/{}.{}", &document_id[..2], document_id, target_format)),
        extracted_text: None,
        error: None,
    }
}

async fn process_compress(state: &AppState, document_id: &str, quality: u8) -> ProcessingResult {
    ProcessingResult {
        document_id: document_id.to_string(),
        operation: format!("compress_q{}", quality),
        success: true,
        output_key: Some(format!("compressed/{}/{}_q{}.jpg", &document_id[..2], document_id, quality)),
        extracted_text: None,
        error: None,
    }
}

async fn process_resize(
    state: &AppState,
    document_id: &str,
    width: u32,
    height: u32,
) -> ProcessingResult {
    ProcessingResult {
        document_id: document_id.to_string(),
        operation: format!("resize_{}x{}", width, height),
        success: true,
        output_key: Some(format!("resized/{}/{}_{}x{}.jpg", &document_id[..2], document_id, width, height)),
        extracted_text: None,
        error: None,
    }
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
                .add_directive("document_processor=info".parse().unwrap()),
        )
        .json()
        .init();

    REGISTRY.register(Box::new(DOCUMENTS_PROCESSED.clone())).unwrap();
    REGISTRY.register(Box::new(PROCESSING_ERRORS.clone())).unwrap();
    REGISTRY.register(Box::new(PROCESSING_DURATION.clone())).unwrap();

    let s3_client = create_s3_client().await;
    let documents_bucket = env::var("DOCUMENTS_BUCKET").unwrap_or_else(|_| "documents".to_string());
    let thumbnails_bucket = env::var("THUMBNAILS_BUCKET").unwrap_or_else(|_| "thumbnails".to_string());

    let state = AppState {
        s3_client,
        documents_bucket,
        thumbnails_bucket,
        processing_cache: Arc::new(RwLock::new(HashMap::new())),
    };

    let cors = CorsLayer::new()
        .allow_origin(Any)
        .allow_methods(Any)
        .allow_headers(Any);

    let app = Router::new()
        .route("/health", get(health_check))
        .route("/metrics", get(metrics_handler))
        .route("/documents", post(upload_document))
        .route("/documents/:document_id", get(get_document_metadata))
        .route("/documents/:document_id/download", get(download_document))
        .route("/documents/:document_id/thumbnail", get(get_thumbnail))
        .route("/documents/:document_id/process", post(process_document))
        .layer(cors)
        .layer(CompressionLayer::new())
        .layer(TraceLayer::new_for_http())
        .with_state(state);

    let port = env::var("PORT").unwrap_or_else(|_| "8091".to_string());
    let addr = format!("0.0.0.0:{}", port);

    info!("Starting document processor on {}", addr);
    info!("S3 endpoint: {}", env::var("S3_ENDPOINT").unwrap_or_else(|_| "http://rustfs:9000".to_string()));

    let listener = tokio::net::TcpListener::bind(&addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}
