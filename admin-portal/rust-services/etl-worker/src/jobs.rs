use serde::{Deserialize, Serialize};
use chrono::{DateTime, Utc};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobMetadata {
    pub created_by: String,
    pub priority: i32,
    pub tags: Vec<String>,
    pub retry_count: u32,
    pub max_retries: u32,
}

impl Default for JobMetadata {
    fn default() -> Self {
        Self {
            created_by: "system".to_string(),
            priority: 0,
            tags: vec![],
            retry_count: 0,
            max_retries: 3,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobProgress {
    pub job_id: String,
    pub stage: String,
    pub progress_percent: f32,
    pub records_processed: u64,
    pub estimated_remaining_secs: Option<u64>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobSchedule {
    pub cron_expression: Option<String>,
    pub run_once_at: Option<DateTime<Utc>>,
    pub repeat_interval_secs: Option<u64>,
    pub enabled: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobDependency {
    pub depends_on: Vec<String>,
    pub wait_for_completion: bool,
    pub fail_on_dependency_failure: bool,
}
