from flask import Flask, request, jsonify
import datafusion
from datafusion import SessionContext
import pyarrow.parquet as pq
import requests
import json

app = Flask(__name__)

# Create DataFusion session context
ctx = SessionContext()

# Registered tables cache
registered_tables = set()

def register_delta_table(table_name, table_path):
    """Register Delta Lake table in DataFusion"""
    try:
        # Read Delta table as Parquet
        delta_service_url = "http://deltalake-service:5001"
        response = requests.post(
            f"{delta_service_url}/tables/read",
            json={"table_path": table_path},
            timeout=30
        )
        
        if response.status_code != 200:
            raise Exception(f"Failed to read Delta table: {response.text}")
        
        data = response.json()
        if not data.get("success"):
            raise Exception(f"Delta read failed: {data.get('error')}")
        
        # Convert to PyArrow table
        records = data.get("data", [])
        if not records:
            return False
        
        # Register in DataFusion
        ctx.register_json(table_name, json.dumps(records))
        registered_tables.add(table_name)
        return True
        
    except Exception as e:
        raise Exception(f"Failed to register table {table_name}: {str(e)}")

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "healthy", "service": "datafusion"}), 200

@app.route('/query/execute', methods=['POST'])
def execute_query():
    """Execute SQL query using DataFusion"""
    try:
        data = request.json
        sql = data.get('sql')
        
        if not sql:
            return jsonify({"error": "sql required"}), 400
        
        # Execute query
        df = ctx.sql(sql)
        result = df.collect()
        
        # Convert to JSON
        rows = []
        for batch in result:
            for i in range(batch.num_rows):
                row = {}
                for col_name in batch.schema.names:
                    col_data = batch.column(batch.schema.get_field_index(col_name))
                    row[col_name] = col_data[i].as_py()
                rows.append(row)
        
        return jsonify({
            "success": True,
            "rows": rows,
            "row_count": len(rows)
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/query/register-table', methods=['POST'])
def register_table():
    """Register Delta Lake table for querying"""
    try:
        data = request.json
        table_name = data.get('table_name')
        table_path = data.get('table_path')
        
        if not table_name or not table_path:
            return jsonify({"error": "table_name and table_path required"}), 400
        
        success = register_delta_table(table_name, table_path)
        
        return jsonify({
            "success": success,
            "table_name": table_name,
            "registered": table_name in registered_tables
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/query/tables', methods=['GET'])
def list_tables():
    """List registered tables"""
    return jsonify({
        "tables": list(registered_tables)
    }), 200

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5004)
