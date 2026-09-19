use lazy_static::lazy_static;
use prometheus::{
    Counter, CounterVec, Gauge, GaugeVec, Histogram, HistogramOpts, HistogramVec, IntCounter,
    IntCounterVec, IntGauge, IntGaugeVec, Opts, Registry,
};

lazy_static! {
    pub static ref REGISTRY: Registry = Registry::new();

    pub static ref HTTP_REQUESTS_TOTAL: IntCounterVec = IntCounterVec::new(
        Opts::new("http_requests_total", "Total HTTP requests"),
        &["method", "path", "status"]
    )
    .expect("metric can be created");

    pub static ref HTTP_REQUEST_DURATION: HistogramVec = HistogramVec::new(
        HistogramOpts::new("http_request_duration_seconds", "HTTP request duration"),
        &["method", "path"]
    )
    .expect("metric can be created");

    pub static ref STORAGE_OPERATIONS_TOTAL: IntCounterVec = IntCounterVec::new(
        Opts::new("storage_operations_total", "Total storage operations"),
        &["operation", "bucket", "status"]
    )
    .expect("metric can be created");

    pub static ref STORAGE_OPERATION_DURATION: HistogramVec = HistogramVec::new(
        HistogramOpts::new("storage_operation_duration_seconds", "Storage operation duration"),
        &["operation", "bucket"]
    )
    .expect("metric can be created");

    pub static ref STORAGE_BYTES_TRANSFERRED: IntCounterVec = IntCounterVec::new(
        Opts::new("storage_bytes_transferred_total", "Total bytes transferred"),
        &["direction", "bucket"]
    )
    .expect("metric can be created");

    pub static ref ACTIVE_UPLOADS: IntGauge = IntGauge::new(
        "storage_active_uploads",
        "Number of active uploads"
    )
    .expect("metric can be created");

    pub static ref ACTIVE_DOWNLOADS: IntGauge = IntGauge::new(
        "storage_active_downloads",
        "Number of active downloads"
    )
    .expect("metric can be created");

    pub static ref MULTIPART_UPLOADS_IN_PROGRESS: IntGaugeVec = IntGaugeVec::new(
        Opts::new("storage_multipart_uploads_in_progress", "Multipart uploads in progress"),
        &["bucket"]
    )
    .expect("metric can be created");

    pub static ref CACHE_HITS: IntCounter = IntCounter::new(
        "storage_cache_hits_total",
        "Total cache hits"
    )
    .expect("metric can be created");

    pub static ref CACHE_MISSES: IntCounter = IntCounter::new(
        "storage_cache_misses_total",
        "Total cache misses"
    )
    .expect("metric can be created");

    pub static ref S3_CONNECTION_ERRORS: IntCounterVec = IntCounterVec::new(
        Opts::new("storage_s3_connection_errors_total", "S3 connection errors"),
        &["error_type"]
    )
    .expect("metric can be created");

    pub static ref OBJECT_SIZE_HISTOGRAM: Histogram = Histogram::with_opts(
        HistogramOpts::new("storage_object_size_bytes", "Object size distribution")
            .buckets(vec![
                1024.0,           // 1KB
                10240.0,          // 10KB
                102400.0,         // 100KB
                1048576.0,        // 1MB
                10485760.0,       // 10MB
                104857600.0,      // 100MB
                1073741824.0,     // 1GB
            ])
    )
    .expect("metric can be created");
}

pub fn register_metrics() {
    REGISTRY
        .register(Box::new(HTTP_REQUESTS_TOTAL.clone()))
        .expect("collector can be registered");
    REGISTRY
        .register(Box::new(HTTP_REQUEST_DURATION.clone()))
        .expect("collector can be registered");
    REGISTRY
        .register(Box::new(STORAGE_OPERATIONS_TOTAL.clone()))
        .expect("collector can be registered");
    REGISTRY
        .register(Box::new(STORAGE_OPERATION_DURATION.clone()))
        .expect("collector can be registered");
    REGISTRY
        .register(Box::new(STORAGE_BYTES_TRANSFERRED.clone()))
        .expect("collector can be registered");
    REGISTRY
        .register(Box::new(ACTIVE_UPLOADS.clone()))
        .expect("collector can be registered");
    REGISTRY
        .register(Box::new(ACTIVE_DOWNLOADS.clone()))
        .expect("collector can be registered");
    REGISTRY
        .register(Box::new(MULTIPART_UPLOADS_IN_PROGRESS.clone()))
        .expect("collector can be registered");
    REGISTRY
        .register(Box::new(CACHE_HITS.clone()))
        .expect("collector can be registered");
    REGISTRY
        .register(Box::new(CACHE_MISSES.clone()))
        .expect("collector can be registered");
    REGISTRY
        .register(Box::new(S3_CONNECTION_ERRORS.clone()))
        .expect("collector can be registered");
    REGISTRY
        .register(Box::new(OBJECT_SIZE_HISTOGRAM.clone()))
        .expect("collector can be registered");
}

pub fn record_upload(bucket: &str, size: u64, duration_secs: f64, success: bool) {
    let status = if success { "success" } else { "error" };
    STORAGE_OPERATIONS_TOTAL
        .with_label_values(&["upload", bucket, status])
        .inc();
    STORAGE_OPERATION_DURATION
        .with_label_values(&["upload", bucket])
        .observe(duration_secs);
    if success {
        STORAGE_BYTES_TRANSFERRED
            .with_label_values(&["upload", bucket])
            .inc_by(size as i64);
        OBJECT_SIZE_HISTOGRAM.observe(size as f64);
    }
}

pub fn record_download(bucket: &str, size: u64, duration_secs: f64, success: bool) {
    let status = if success { "success" } else { "error" };
    STORAGE_OPERATIONS_TOTAL
        .with_label_values(&["download", bucket, status])
        .inc();
    STORAGE_OPERATION_DURATION
        .with_label_values(&["download", bucket])
        .observe(duration_secs);
    if success {
        STORAGE_BYTES_TRANSFERRED
            .with_label_values(&["download", bucket])
            .inc_by(size as i64);
    }
}

pub fn record_delete(bucket: &str, success: bool) {
    let status = if success { "success" } else { "error" };
    STORAGE_OPERATIONS_TOTAL
        .with_label_values(&["delete", bucket, status])
        .inc();
}

pub fn record_list(bucket: &str, duration_secs: f64, success: bool) {
    let status = if success { "success" } else { "error" };
    STORAGE_OPERATIONS_TOTAL
        .with_label_values(&["list", bucket, status])
        .inc();
    STORAGE_OPERATION_DURATION
        .with_label_values(&["list", bucket])
        .observe(duration_secs);
}

pub fn record_http_request(method: &str, path: &str, status: u16, duration_secs: f64) {
    HTTP_REQUESTS_TOTAL
        .with_label_values(&[method, path, &status.to_string()])
        .inc();
    HTTP_REQUEST_DURATION
        .with_label_values(&[method, path])
        .observe(duration_secs);
}

pub fn get_metrics() -> String {
    use prometheus::Encoder;
    let encoder = prometheus::TextEncoder::new();
    let metric_families = REGISTRY.gather();
    let mut buffer = Vec::new();
    encoder.encode(&metric_families, &mut buffer).unwrap();
    String::from_utf8(buffer).unwrap()
}
