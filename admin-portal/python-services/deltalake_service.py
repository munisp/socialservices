"""
Delta Lake Service using delta-rs Python bindings
Provides gRPC API for Go orchestrator to interact with Delta Lake
"""

import os
from typing import Dict, List, Optional, Any
from datetime import datetime
import pyarrow as pa
import pyarrow.parquet as pq
from deltalake import DeltaTable, write_deltalake
from deltalake.writer import write_deltalake
import grpc
from concurrent import futures
import json

# gRPC generated code (would be generated from .proto file)
# For now, using simple HTTP API with Flask

from flask import Flask, request, jsonify
import pandas as pd

app = Flask(__name__)

# Configuration
# RustFS is the default S3-compatible storage (2.3x faster than MinIO)
LAKEHOUSE_BASE_PATH = os.getenv("LAKEHOUSE_PATH", "/data/lakehouse")
S3_ENDPOINT = os.getenv("S3_ENDPOINT", "http://rustfs:9000")
AWS_ACCESS_KEY = os.getenv("AWS_ACCESS_KEY", "rustfsadmin")
AWS_SECRET_KEY = os.getenv("AWS_SECRET_KEY", "rustfsadmin")

# Configure S3 storage options
storage_options = {
    "AWS_ENDPOINT_URL": S3_ENDPOINT,
    "AWS_ACCESS_KEY_ID": AWS_ACCESS_KEY,
    "AWS_SECRET_ACCESS_KEY": AWS_SECRET_KEY,
    "AWS_REGION": "us-east-1",
    "AWS_S3_ALLOW_UNSAFE_RENAME": "true",
}


class DeltaLakeService:
    """Service for managing Delta Lake operations"""
    
    def __init__(self, base_path: str):
        self.base_path = base_path
        
    def create_table(self, table_path: str, schema: pa.Schema, partition_by: Optional[List[str]] = None) -> Dict[str, Any]:
        """Create a new Delta Lake table"""
        try:
            full_path = f"{self.base_path}/{table_path}"
            
            # Create empty table with schema
            empty_table = pa.Table.from_pydict({}, schema=schema)
            
            write_deltalake(
                full_path,
                empty_table,
                mode="error",  # Fail if exists
                partition_by=partition_by,
                storage_options=storage_options
            )
            
            return {
                "success": True,
                "table_path": full_path,
                "schema": str(schema),
                "partitions": partition_by or []
            }
        except Exception as e:
            return {"success": False, "error": str(e)}
    
    def write_data(self, table_path: str, data: List[Dict], mode: str = "append") -> Dict[str, Any]:
        """Write data to Delta Lake table"""
        try:
            full_path = f"{self.base_path}/{table_path}"
            
            # Convert to PyArrow Table
            df = pd.DataFrame(data)
            table = pa.Table.from_pandas(df)
            
            write_deltalake(
                full_path,
                table,
                mode=mode,  # append, overwrite, error
                storage_options=storage_options
            )
            
            # Get table info
            dt = DeltaTable(full_path, storage_options=storage_options)
            
            return {
                "success": True,
                "rows_written": len(data),
                "version": dt.version(),
                "files": len(dt.files())
            }
        except Exception as e:
            return {"success": False, "error": str(e)}
    
    def read_data(self, table_path: str, filters: Optional[List] = None, columns: Optional[List[str]] = None) -> Dict[str, Any]:
        """Read data from Delta Lake table"""
        try:
            full_path = f"{self.base_path}/{table_path}"
            
            dt = DeltaTable(full_path, storage_options=storage_options)
            
            # Convert to PyArrow Table
            table = dt.to_pyarrow_table(
                filters=filters,
                columns=columns
            )
            
            # Convert to list of dicts
            df = table.to_pandas()
            data = df.to_dict(orient="records")
            
            return {
                "success": True,
                "rows": len(data),
                "data": data,
                "version": dt.version()
            }
        except Exception as e:
            return {"success": False, "error": str(e)}
    
    def time_travel(self, table_path: str, version: Optional[int] = None, timestamp: Optional[str] = None) -> Dict[str, Any]:
        """Query historical version of table"""
        try:
            full_path = f"{self.base_path}/{table_path}"
            
            if version is not None:
                dt = DeltaTable(full_path, version=version, storage_options=storage_options)
            elif timestamp is not None:
                dt = DeltaTable(full_path, storage_options=storage_options)
                dt.load_as_version(timestamp)
            else:
                return {"success": False, "error": "Must provide version or timestamp"}
            
            table = dt.to_pyarrow_table()
            df = table.to_pandas()
            data = df.to_dict(orient="records")
            
            return {
                "success": True,
                "version": dt.version(),
                "rows": len(data),
                "data": data
            }
        except Exception as e:
            return {"success": False, "error": str(e)}
    
    def optimize(self, table_path: str, target_size: int = 134217728) -> Dict[str, Any]:
        """Optimize table by compacting small files"""
        try:
            full_path = f"{self.base_path}/{table_path}"
            
            dt = DeltaTable(full_path, storage_options=storage_options)
            
            # Compact files
            metrics = dt.optimize.compact(target_size=target_size)
            
            return {
                "success": True,
                "files_added": metrics["numFilesAdded"],
                "files_removed": metrics["numFilesRemoved"],
                "version": dt.version()
            }
        except Exception as e:
            return {"success": False, "error": str(e)}
    
    def z_order(self, table_path: str, columns: List[str]) -> Dict[str, Any]:
        """Z-order table by specified columns"""
        try:
            full_path = f"{self.base_path}/{table_path}"
            
            dt = DeltaTable(full_path, storage_options=storage_options)
            
            # Z-order optimization
            metrics = dt.optimize.z_order(columns)
            
            return {
                "success": True,
                "files_added": metrics["numFilesAdded"],
                "files_removed": metrics["numFilesRemoved"],
                "z_order_columns": columns,
                "version": dt.version()
            }
        except Exception as e:
            return {"success": False, "error": str(e)}
    
    def vacuum(self, table_path: str, retention_hours: int = 168, dry_run: bool = False) -> Dict[str, Any]:
        """Remove old files based on retention policy"""
        try:
            full_path = f"{self.base_path}/{table_path}"
            
            dt = DeltaTable(full_path, storage_options=storage_options)
            
            # Vacuum old files
            files_deleted = dt.vacuum(retention_hours=retention_hours, dry_run=dry_run)
            
            return {
                "success": True,
                "files_deleted": len(files_deleted),
                "retention_hours": retention_hours,
                "dry_run": dry_run
            }
        except Exception as e:
            return {"success": False, "error": str(e)}
    
    def get_history(self, table_path: str, limit: int = 10) -> Dict[str, Any]:
        """Get table history"""
        try:
            full_path = f"{self.base_path}/{table_path}"
            
            dt = DeltaTable(full_path, storage_options=storage_options)
            
            history = dt.history(limit=limit)
            
            return {
                "success": True,
                "history": history
            }
        except Exception as e:
            return {"success": False, "error": str(e)}
    
    def get_schema(self, table_path: str) -> Dict[str, Any]:
        """Get table schema"""
        try:
            full_path = f"{self.base_path}/{table_path}"
            
            dt = DeltaTable(full_path, storage_options=storage_options)
            
            schema = dt.schema().to_pyarrow()
            
            return {
                "success": True,
                "schema": str(schema),
                "fields": [{"name": field.name, "type": str(field.type)} for field in schema]
            }
        except Exception as e:
            return {"success": False, "error": str(e)}
    
    def merge(self, table_path: str, source_data: List[Dict], predicate: str, 
              update_set: Dict[str, str], insert_values: Dict[str, str]) -> Dict[str, Any]:
        """Merge (upsert) data into table"""
        try:
            full_path = f"{self.base_path}/{table_path}"
            
            dt = DeltaTable(full_path, storage_options=storage_options)
            
            # Convert source data to PyArrow
            source_df = pd.DataFrame(source_data)
            source_table = pa.Table.from_pandas(source_df)
            
            # Perform merge
            (
                dt.merge(
                    source=source_table,
                    predicate=predicate,
                    source_alias="source",
                    target_alias="target"
                )
                .when_matched_update(updates=update_set)
                .when_not_matched_insert(values=insert_values)
                .execute()
            )
            
            return {
                "success": True,
                "version": dt.version()
            }
        except Exception as e:
            return {"success": False, "error": str(e)}


# Initialize service
delta_service = DeltaLakeService(LAKEHOUSE_BASE_PATH)


# Flask API endpoints

@app.route("/health", methods=["GET"])
def health():
    """Health check endpoint"""
    return jsonify({"status": "healthy", "service": "delta-lake"})


@app.route("/tables/create", methods=["POST"])
def create_table():
    """Create new Delta Lake table"""
    data = request.json
    
    # Parse schema from JSON
    schema_dict = data.get("schema", {})
    fields = [
        pa.field(field["name"], getattr(pa, field["type"])())
        for field in schema_dict.get("fields", [])
    ]
    schema = pa.schema(fields)
    
    result = delta_service.create_table(
        table_path=data["table_path"],
        schema=schema,
        partition_by=data.get("partition_by")
    )
    
    return jsonify(result)


@app.route("/tables/write", methods=["POST"])
def write_data():
    """Write data to Delta Lake table"""
    data = request.json
    
    result = delta_service.write_data(
        table_path=data["table_path"],
        data=data["data"],
        mode=data.get("mode", "append")
    )
    
    return jsonify(result)


@app.route("/tables/read", methods=["POST"])
def read_data():
    """Read data from Delta Lake table"""
    data = request.json
    
    result = delta_service.read_data(
        table_path=data["table_path"],
        filters=data.get("filters"),
        columns=data.get("columns")
    )
    
    return jsonify(result)


@app.route("/tables/time_travel", methods=["POST"])
def time_travel():
    """Query historical version"""
    data = request.json
    
    result = delta_service.time_travel(
        table_path=data["table_path"],
        version=data.get("version"),
        timestamp=data.get("timestamp")
    )
    
    return jsonify(result)


@app.route("/tables/optimize", methods=["POST"])
def optimize():
    """Optimize table"""
    data = request.json
    
    result = delta_service.optimize(
        table_path=data["table_path"],
        target_size=data.get("target_size", 134217728)
    )
    
    return jsonify(result)


@app.route("/tables/z_order", methods=["POST"])
def z_order():
    """Z-order table"""
    data = request.json
    
    result = delta_service.z_order(
        table_path=data["table_path"],
        columns=data["columns"]
    )
    
    return jsonify(result)


@app.route("/tables/vacuum", methods=["POST"])
def vacuum():
    """Vacuum old files"""
    data = request.json
    
    result = delta_service.vacuum(
        table_path=data["table_path"],
        retention_hours=data.get("retention_hours", 168),
        dry_run=data.get("dry_run", False)
    )
    
    return jsonify(result)


@app.route("/tables/history", methods=["POST"])
def get_history():
    """Get table history"""
    data = request.json
    
    result = delta_service.get_history(
        table_path=data["table_path"],
        limit=data.get("limit", 10)
    )
    
    return jsonify(result)


@app.route("/tables/schema", methods=["POST"])
def get_schema():
    """Get table schema"""
    data = request.json
    
    result = delta_service.get_schema(
        table_path=data["table_path"]
    )
    
    return jsonify(result)


@app.route("/tables/merge", methods=["POST"])
def merge():
    """Merge (upsert) data"""
    data = request.json
    
    result = delta_service.merge(
        table_path=data["table_path"],
        source_data=data["source_data"],
        predicate=data["predicate"],
        update_set=data["update_set"],
        insert_values=data["insert_values"]
    )
    
    return jsonify(result)


if __name__ == "__main__":
    # Create lakehouse directories
    os.makedirs(f"{LAKEHOUSE_BASE_PATH}/bronze", exist_ok=True)
    os.makedirs(f"{LAKEHOUSE_BASE_PATH}/silver", exist_ok=True)
    os.makedirs(f"{LAKEHOUSE_BASE_PATH}/gold", exist_ok=True)
    
    print(f"Starting Delta Lake Service on port 5001...")
    print(f"Lakehouse path: {LAKEHOUSE_BASE_PATH}")
    print(f"S3 endpoint: {S3_ENDPOINT}")
    
    app.run(host="0.0.0.0", port=5001, debug=False)
