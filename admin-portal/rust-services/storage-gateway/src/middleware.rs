use axum::{
    body::Body,
    extract::Request,
    http::{header, HeaderValue, StatusCode},
    middleware::Next,
    response::Response,
};
use std::time::Instant;
use tracing::{info, warn};
use uuid::Uuid;

use crate::metrics;

pub async fn request_id_middleware(mut request: Request, next: Next) -> Response {
    let request_id = request
        .headers()
        .get("x-request-id")
        .and_then(|v| v.to_str().ok())
        .map(|s| s.to_string())
        .unwrap_or_else(|| Uuid::new_v4().to_string());

    request
        .headers_mut()
        .insert("x-request-id", HeaderValue::from_str(&request_id).unwrap());

    let mut response = next.run(request).await;

    response
        .headers_mut()
        .insert("x-request-id", HeaderValue::from_str(&request_id).unwrap());

    response
}

pub async fn logging_middleware(request: Request, next: Next) -> Response {
    let method = request.method().to_string();
    let uri = request.uri().to_string();
    let request_id = request
        .headers()
        .get("x-request-id")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("unknown")
        .to_string();

    let start = Instant::now();

    let response = next.run(request).await;

    let duration = start.elapsed();
    let status = response.status().as_u16();

    if status >= 400 {
        warn!(
            request_id = %request_id,
            method = %method,
            uri = %uri,
            status = %status,
            duration_ms = %duration.as_millis(),
            "Request failed"
        );
    } else {
        info!(
            request_id = %request_id,
            method = %method,
            uri = %uri,
            status = %status,
            duration_ms = %duration.as_millis(),
            "Request completed"
        );
    }

    metrics::record_http_request(&method, &uri, status, duration.as_secs_f64());

    response
}

pub async fn auth_middleware(request: Request, next: Next) -> Result<Response, StatusCode> {
    let auth_header = request.headers().get(header::AUTHORIZATION);

    match auth_header {
        Some(value) => {
            let auth_str = value.to_str().unwrap_or("");
            if auth_str.starts_with("Bearer ") || auth_str.starts_with("AWS4-HMAC-SHA256") {
                Ok(next.run(request).await)
            } else {
                warn!("Invalid authorization header format");
                Err(StatusCode::UNAUTHORIZED)
            }
        }
        None => {
            let skip_auth = std::env::var("SKIP_AUTH").unwrap_or_else(|_| "false".to_string());
            if skip_auth == "true" {
                Ok(next.run(request).await)
            } else {
                warn!("Missing authorization header");
                Err(StatusCode::UNAUTHORIZED)
            }
        }
    }
}

pub async fn rate_limit_middleware(request: Request, next: Next) -> Result<Response, StatusCode> {
    Ok(next.run(request).await)
}

pub async fn content_length_limit_middleware(
    request: Request,
    next: Next,
) -> Result<Response, StatusCode> {
    let max_size: u64 = std::env::var("MAX_UPLOAD_SIZE_MB")
        .unwrap_or_else(|_| "100".to_string())
        .parse()
        .unwrap_or(100)
        * 1024
        * 1024;

    if let Some(content_length) = request.headers().get(header::CONTENT_LENGTH) {
        if let Ok(length_str) = content_length.to_str() {
            if let Ok(length) = length_str.parse::<u64>() {
                if length > max_size {
                    warn!(
                        "Request body too large: {} bytes (max: {} bytes)",
                        length, max_size
                    );
                    return Err(StatusCode::PAYLOAD_TOO_LARGE);
                }
            }
        }
    }

    Ok(next.run(request).await)
}

pub async fn cors_preflight_middleware(request: Request, next: Next) -> Response {
    if request.method() == axum::http::Method::OPTIONS {
        return Response::builder()
            .status(StatusCode::NO_CONTENT)
            .header(header::ACCESS_CONTROL_ALLOW_ORIGIN, "*")
            .header(
                header::ACCESS_CONTROL_ALLOW_METHODS,
                "GET, POST, PUT, DELETE, HEAD, OPTIONS",
            )
            .header(
                header::ACCESS_CONTROL_ALLOW_HEADERS,
                "Content-Type, Authorization, X-Request-ID",
            )
            .header(header::ACCESS_CONTROL_MAX_AGE, "86400")
            .body(Body::empty())
            .unwrap();
    }

    next.run(request).await
}
