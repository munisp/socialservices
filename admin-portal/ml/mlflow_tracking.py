from __future__ import annotations

import json
import os
from pathlib import Path


def log_artifact_run(artifact_dir: str | Path, experiment: str, run_name: str | None = None) -> str | None:
    tracking_uri = os.getenv("MLFLOW_TRACKING_URI")
    if not tracking_uri:
        return None
    try:
        import mlflow
    except ImportError as exc:
        raise RuntimeError("MLFLOW_TRACKING_URI is configured but mlflow is not installed") from exc
    artifact = Path(artifact_dir); manifest = json.loads((artifact / "manifest.json").read_text()); mlflow.set_tracking_uri(tracking_uri); mlflow.set_experiment(experiment)
    with mlflow.start_run(run_name=run_name or manifest["version"]) as run:
        mlflow.log_params({"model_type": manifest["model_type"], "dataset_kind": manifest["dataset_kind"], "seed": manifest["seed"], "training_rows": manifest["training_rows"]})
        mlflow.log_metrics({key: float(value) for key, value in manifest["metrics"].items() if isinstance(value, (int, float))})
        mlflow.log_artifacts(str(artifact), artifact_path="model")
        mlflow.set_tags({"model_version": manifest["version"], "production_validated": str(manifest.get("production_validated", False)).lower()})
        return run.info.run_id
