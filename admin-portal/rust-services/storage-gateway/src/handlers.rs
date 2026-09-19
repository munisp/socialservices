use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    Json,
};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use tracing::{info, instrument};

use crate::{AppState, StorageError};

#[derive(Debug, Serialize, Deserialize)]
pub struct InitiateMultipartUploadRequest {
    pub key: String,
    pub content_type: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct InitiateMultipartUploadResponse {
    pub upload_id: String,
    pub key: String,
    pub bucket: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CompleteMultipartUploadRequest {
    pub upload_id: String,
    pub parts: Vec<PartInfo>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PartInfo {
    pub part_number: i32,
    pub e_tag: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CompleteMultipartUploadResponse {
    pub key: String,
    pub bucket: String,
    pub e_tag: Option<String>,
    pub location: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct AbortMultipartUploadRequest {
    pub upload_id: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ListPartsQuery {
    pub upload_id: String,
    pub max_parts: Option<i32>,
    pub part_number_marker: Option<i32>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ListPartsResponse {
    pub bucket: String,
    pub key: String,
    pub upload_id: String,
    pub parts: Vec<PartDetail>,
    pub is_truncated: bool,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PartDetail {
    pub part_number: i32,
    pub size: i64,
    pub e_tag: Option<String>,
}

#[instrument(skip(state))]
pub async fn initiate_multipart_upload(
    State(state): State<AppState>,
    Path(bucket): Path<String>,
    Json(request): Json<InitiateMultipartUploadRequest>,
) -> Result<Json<InitiateMultipartUploadResponse>, StorageError> {
    let content_type = request
        .content_type
        .unwrap_or_else(|| "application/octet-stream".to_string());

    let result = state
        .s3_client
        .create_multipart_upload()
        .bucket(&bucket)
        .key(&request.key)
        .content_type(&content_type)
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    let upload_id = result
        .upload_id()
        .ok_or_else(|| StorageError::Internal("No upload ID returned".to_string()))?
        .to_string();

    info!(
        "Initiated multipart upload: bucket={}, key={}, upload_id={}",
        bucket, request.key, upload_id
    );

    Ok(Json(InitiateMultipartUploadResponse {
        upload_id,
        key: request.key,
        bucket,
    }))
}

#[instrument(skip(state))]
pub async fn complete_multipart_upload(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
    Json(request): Json<CompleteMultipartUploadRequest>,
) -> Result<Json<CompleteMultipartUploadResponse>, StorageError> {
    use aws_sdk_s3::types::{CompletedMultipartUpload, CompletedPart};

    let parts: Vec<CompletedPart> = request
        .parts
        .iter()
        .map(|p| {
            CompletedPart::builder()
                .part_number(p.part_number)
                .e_tag(&p.e_tag)
                .build()
        })
        .collect();

    let completed_upload = CompletedMultipartUpload::builder()
        .set_parts(Some(parts))
        .build();

    let result = state
        .s3_client
        .complete_multipart_upload()
        .bucket(&bucket)
        .key(&key)
        .upload_id(&request.upload_id)
        .multipart_upload(completed_upload)
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    info!(
        "Completed multipart upload: bucket={}, key={}, upload_id={}",
        bucket, key, request.upload_id
    );

    Ok(Json(CompleteMultipartUploadResponse {
        key,
        bucket: bucket.clone(),
        e_tag: result.e_tag().map(|s| s.to_string()),
        location: result.location().unwrap_or_default().to_string(),
    }))
}

#[instrument(skip(state))]
pub async fn abort_multipart_upload(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
    Json(request): Json<AbortMultipartUploadRequest>,
) -> Result<StatusCode, StorageError> {
    state
        .s3_client
        .abort_multipart_upload()
        .bucket(&bucket)
        .key(&key)
        .upload_id(&request.upload_id)
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    info!(
        "Aborted multipart upload: bucket={}, key={}, upload_id={}",
        bucket, key, request.upload_id
    );

    Ok(StatusCode::NO_CONTENT)
}

#[instrument(skip(state))]
pub async fn list_multipart_uploads(
    State(state): State<AppState>,
    Path(bucket): Path<String>,
) -> Result<Json<Vec<MultipartUploadInfo>>, StorageError> {
    let result = state
        .s3_client
        .list_multipart_uploads()
        .bucket(&bucket)
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    let uploads: Vec<MultipartUploadInfo> = result
        .uploads()
        .iter()
        .map(|u| MultipartUploadInfo {
            key: u.key().unwrap_or_default().to_string(),
            upload_id: u.upload_id().unwrap_or_default().to_string(),
            initiated: u.initiated().map(|d| d.to_string()),
        })
        .collect();

    Ok(Json(uploads))
}

#[derive(Debug, Serialize, Deserialize)]
pub struct MultipartUploadInfo {
    pub key: String,
    pub upload_id: String,
    pub initiated: Option<String>,
}

#[instrument(skip(state))]
pub async fn list_parts(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
    Query(query): Query<ListPartsQuery>,
) -> Result<Json<ListPartsResponse>, StorageError> {
    let mut request = state
        .s3_client
        .list_parts()
        .bucket(&bucket)
        .key(&key)
        .upload_id(&query.upload_id);

    if let Some(max_parts) = query.max_parts {
        request = request.max_parts(max_parts);
    }
    if let Some(marker) = query.part_number_marker {
        request = request.part_number_marker(marker);
    }

    let result = request
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    let parts: Vec<PartDetail> = result
        .parts()
        .iter()
        .map(|p| PartDetail {
            part_number: p.part_number().unwrap_or(0),
            size: p.size().unwrap_or(0),
            e_tag: p.e_tag().map(|s| s.to_string()),
        })
        .collect();

    Ok(Json(ListPartsResponse {
        bucket,
        key,
        upload_id: query.upload_id,
        parts,
        is_truncated: result.is_truncated().unwrap_or(false),
    }))
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ObjectTaggingRequest {
    pub tags: HashMap<String, String>,
}

#[instrument(skip(state))]
pub async fn put_object_tagging(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
    Json(request): Json<ObjectTaggingRequest>,
) -> Result<StatusCode, StorageError> {
    use aws_sdk_s3::types::{Tag, Tagging};

    let tags: Vec<Tag> = request
        .tags
        .iter()
        .map(|(k, v)| Tag::builder().key(k).value(v).build().unwrap())
        .collect();

    let tagging = Tagging::builder().set_tag_set(Some(tags)).build().unwrap();

    state
        .s3_client
        .put_object_tagging()
        .bucket(&bucket)
        .key(&key)
        .tagging(tagging)
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    info!("Set tags for object: {}/{}", bucket, key);

    Ok(StatusCode::NO_CONTENT)
}

#[instrument(skip(state))]
pub async fn get_object_tagging(
    State(state): State<AppState>,
    Path((bucket, key)): Path<(String, String)>,
) -> Result<Json<HashMap<String, String>>, StorageError> {
    let result = state
        .s3_client
        .get_object_tagging()
        .bucket(&bucket)
        .key(&key)
        .send()
        .await
        .map_err(|e| StorageError::S3Error(e.to_string()))?;

    let tags: HashMap<String, String> = result
        .tag_set()
        .iter()
        .map(|t| {
            (
                t.key().to_string(),
                t.value().to_string(),
            )
        })
        .collect();

    Ok(Json(tags))
}
