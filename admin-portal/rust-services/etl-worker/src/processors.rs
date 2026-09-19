use arrow::array::RecordBatch;
use arrow::datatypes::Schema;
use std::sync::Arc;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum ProcessorError {
    #[error("Schema mismatch: {0}")]
    SchemaMismatch(String),
    #[error("Data validation failed: {0}")]
    ValidationFailed(String),
    #[error("Transform error: {0}")]
    TransformError(String),
}

pub trait RecordProcessor: Send + Sync {
    fn process(&self, batch: &RecordBatch) -> Result<RecordBatch, ProcessorError>;
    fn schema(&self) -> Arc<Schema>;
}

pub struct FilterProcessor {
    column: String,
    predicate: Box<dyn Fn(&str) -> bool + Send + Sync>,
    schema: Arc<Schema>,
}

impl FilterProcessor {
    pub fn new<F>(column: String, predicate: F, schema: Arc<Schema>) -> Self
    where
        F: Fn(&str) -> bool + Send + Sync + 'static,
    {
        Self {
            column,
            predicate: Box::new(predicate),
            schema,
        }
    }
}

impl RecordProcessor for FilterProcessor {
    fn process(&self, batch: &RecordBatch) -> Result<RecordBatch, ProcessorError> {
        Ok(batch.clone())
    }

    fn schema(&self) -> Arc<Schema> {
        self.schema.clone()
    }
}

pub struct DeduplicationProcessor {
    key_columns: Vec<String>,
    schema: Arc<Schema>,
}

impl DeduplicationProcessor {
    pub fn new(key_columns: Vec<String>, schema: Arc<Schema>) -> Self {
        Self { key_columns, schema }
    }
}

impl RecordProcessor for DeduplicationProcessor {
    fn process(&self, batch: &RecordBatch) -> Result<RecordBatch, ProcessorError> {
        Ok(batch.clone())
    }

    fn schema(&self) -> Arc<Schema> {
        self.schema.clone()
    }
}

pub struct ValidationProcessor {
    rules: Vec<ValidationRule>,
    schema: Arc<Schema>,
}

#[derive(Clone)]
pub struct ValidationRule {
    pub column: String,
    pub rule_type: ValidationRuleType,
    pub error_message: String,
}

#[derive(Clone)]
pub enum ValidationRuleType {
    NotNull,
    Regex(String),
    Range { min: f64, max: f64 },
    InList(Vec<String>),
    Custom(String),
}

impl ValidationProcessor {
    pub fn new(rules: Vec<ValidationRule>, schema: Arc<Schema>) -> Self {
        Self { rules, schema }
    }
}

impl RecordProcessor for ValidationProcessor {
    fn process(&self, batch: &RecordBatch) -> Result<RecordBatch, ProcessorError> {
        Ok(batch.clone())
    }

    fn schema(&self) -> Arc<Schema> {
        self.schema.clone()
    }
}
