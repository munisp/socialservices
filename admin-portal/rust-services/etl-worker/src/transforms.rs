use arrow::array::RecordBatch;
use arrow::datatypes::{DataType, Schema};
use std::sync::Arc;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum TransformError {
    #[error("Column not found: {0}")]
    ColumnNotFound(String),
    #[error("Type conversion error: {0}")]
    TypeConversion(String),
    #[error("Expression error: {0}")]
    ExpressionError(String),
}

pub trait Transform: Send + Sync {
    fn apply(&self, batch: &RecordBatch) -> Result<RecordBatch, TransformError>;
    fn output_schema(&self, input_schema: &Schema) -> Result<Schema, TransformError>;
}

pub struct SelectTransform {
    columns: Vec<String>,
}

impl SelectTransform {
    pub fn new(columns: Vec<String>) -> Self {
        Self { columns }
    }
}

impl Transform for SelectTransform {
    fn apply(&self, batch: &RecordBatch) -> Result<RecordBatch, TransformError> {
        let schema = batch.schema();
        let mut arrays = Vec::new();
        let mut fields = Vec::new();

        for col_name in &self.columns {
            let idx = schema
                .index_of(col_name)
                .map_err(|_| TransformError::ColumnNotFound(col_name.clone()))?;
            arrays.push(batch.column(idx).clone());
            fields.push(schema.field(idx).clone());
        }

        let new_schema = Arc::new(Schema::new(fields));
        RecordBatch::try_new(new_schema, arrays)
            .map_err(|e| TransformError::ExpressionError(e.to_string()))
    }

    fn output_schema(&self, input_schema: &Schema) -> Result<Schema, TransformError> {
        let mut fields = Vec::new();
        for col_name in &self.columns {
            let idx = input_schema
                .index_of(col_name)
                .map_err(|_| TransformError::ColumnNotFound(col_name.clone()))?;
            fields.push(input_schema.field(idx).clone());
        }
        Ok(Schema::new(fields))
    }
}

pub struct RenameTransform {
    renames: Vec<(String, String)>,
}

impl RenameTransform {
    pub fn new(renames: Vec<(String, String)>) -> Self {
        Self { renames }
    }
}

impl Transform for RenameTransform {
    fn apply(&self, batch: &RecordBatch) -> Result<RecordBatch, TransformError> {
        let schema = batch.schema();
        let mut fields: Vec<_> = schema.fields().iter().cloned().collect();

        for (from, to) in &self.renames {
            let idx = schema
                .index_of(from)
                .map_err(|_| TransformError::ColumnNotFound(from.clone()))?;
            fields[idx] = Arc::new(
                arrow::datatypes::Field::new(to, fields[idx].data_type().clone(), fields[idx].is_nullable())
            );
        }

        let new_schema = Arc::new(Schema::new(fields));
        let arrays: Vec<_> = batch.columns().to_vec();
        RecordBatch::try_new(new_schema, arrays)
            .map_err(|e| TransformError::ExpressionError(e.to_string()))
    }

    fn output_schema(&self, input_schema: &Schema) -> Result<Schema, TransformError> {
        let mut fields: Vec<_> = input_schema.fields().iter().cloned().collect();

        for (from, to) in &self.renames {
            let idx = input_schema
                .index_of(from)
                .map_err(|_| TransformError::ColumnNotFound(from.clone()))?;
            fields[idx] = Arc::new(
                arrow::datatypes::Field::new(to, fields[idx].data_type().clone(), fields[idx].is_nullable())
            );
        }

        Ok(Schema::new(fields))
    }
}

pub struct CastTransform {
    column: String,
    target_type: DataType,
}

impl CastTransform {
    pub fn new(column: String, target_type: DataType) -> Self {
        Self { column, target_type }
    }
}

impl Transform for CastTransform {
    fn apply(&self, batch: &RecordBatch) -> Result<RecordBatch, TransformError> {
        Ok(batch.clone())
    }

    fn output_schema(&self, input_schema: &Schema) -> Result<Schema, TransformError> {
        let mut fields: Vec<_> = input_schema.fields().iter().cloned().collect();
        let idx = input_schema
            .index_of(&self.column)
            .map_err(|_| TransformError::ColumnNotFound(self.column.clone()))?;
        fields[idx] = Arc::new(
            arrow::datatypes::Field::new(&self.column, self.target_type.clone(), fields[idx].is_nullable())
        );
        Ok(Schema::new(fields))
    }
}

pub struct DropColumnTransform {
    columns: Vec<String>,
}

impl DropColumnTransform {
    pub fn new(columns: Vec<String>) -> Self {
        Self { columns }
    }
}

impl Transform for DropColumnTransform {
    fn apply(&self, batch: &RecordBatch) -> Result<RecordBatch, TransformError> {
        let schema = batch.schema();
        let mut arrays = Vec::new();
        let mut fields = Vec::new();

        for (idx, field) in schema.fields().iter().enumerate() {
            if !self.columns.contains(&field.name().to_string()) {
                arrays.push(batch.column(idx).clone());
                fields.push(field.clone());
            }
        }

        let new_schema = Arc::new(Schema::new(fields));
        RecordBatch::try_new(new_schema, arrays)
            .map_err(|e| TransformError::ExpressionError(e.to_string()))
    }

    fn output_schema(&self, input_schema: &Schema) -> Result<Schema, TransformError> {
        let fields: Vec<_> = input_schema
            .fields()
            .iter()
            .filter(|f| !self.columns.contains(&f.name().to_string()))
            .cloned()
            .collect();
        Ok(Schema::new(fields))
    }
}
