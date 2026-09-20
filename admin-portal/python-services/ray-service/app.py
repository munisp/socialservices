from __future__ import annotations

import json
import os
from pathlib import Path

import ray
import torch
from flask import Flask, jsonify, request

from ml.distributed_train import run_trials
from ml.models import FraudMLP

app = Flask(__name__)
if not ray.is_initialized(): ray.init(address=os.getenv("RAY_ADDRESS") or None, include_dashboard=False, ignore_reinit_error=True)


def load_model(artifact: Path):
    manifest = json.loads((artifact / "manifest.json").read_text()); model = FraudMLP(len(manifest["features"])); model.load_state_dict(torch.load(artifact / manifest["weights"], map_location="cpu", weights_only=True)); model.eval(); preprocess = torch.load(artifact / manifest["preprocess"], map_location="cpu", weights_only=True); return manifest, model, preprocess


@ray.remote(num_cpus=1)
def infer_partition(artifact_path: str, records: list[list[float]]):
    manifest, model, preprocess = load_model(Path(artifact_path)); tensor = torch.tensor(records, dtype=torch.float32); tensor = (tensor - preprocess["mean"]) / preprocess["std"]
    with torch.inference_mode(): scores = torch.sigmoid(model(tensor)).tolist()
    return [{"score": score, "model_version": manifest["version"]} for score in scores]


@app.get("/health")
def health(): return jsonify({"status": "healthy", "ray_initialized": ray.is_initialized(), "nodes": len(ray.nodes())})


@app.post("/ml/train-fraud-model")
def train_fraud_model():
    body = request.get_json(force=True); data_path = Path(body.get("data_path", "")); output = Path(body.get("output_path", "/registry/runs/fraud")); dataset_kind = body.get("dataset_kind", "synthetic")
    if not data_path.exists(): return jsonify({"error": "data_path must be an existing mounted CSV or Parquet export"}), 400
    manifest = run_trials(data_path, output, dataset_kind); return jsonify({"success": True, "artifact_path": str(output), "manifest": manifest})


@app.post("/ml/predict-fraud-batch")
def predict_fraud_batch():
    body = request.get_json(force=True); artifact = Path(body.get("artifact_path", "")); records = body.get("records", []); batch_size = min(5000, max(1, int(body.get("batch_size", 512))))
    if not (artifact / "manifest.json").exists() or not records: return jsonify({"error": "artifact_path and records are required"}), 400
    futures = [infer_partition.remote(str(artifact), records[index:index + batch_size]) for index in range(0, len(records), batch_size)]; values = [item for partition in ray.get(futures) for item in partition]; return jsonify({"success": True, "count": len(values), "predictions": values})


if __name__ == "__main__": app.run(host="0.0.0.0", port=5005)
