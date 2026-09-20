"""Governed continuous-training entrypoint for synthetic or production exports."""
from __future__ import annotations

import argparse
import json
from datetime import datetime, timezone
from pathlib import Path

from .registry import ModelRegistry
from .train import train_model

REQUIRED_PRODUCTION_METADATA = ["schema_version", "label_provenance", "privacy_approval", "watermark", "output_sha256"]


def validate_source(source: Path, dataset_kind: str, metadata_path: Path | None) -> dict:
    if not source.exists(): raise FileNotFoundError(source)
    metadata = json.loads(metadata_path.read_text()) if metadata_path else {}
    if dataset_kind == "production":
        missing = [field for field in REQUIRED_PRODUCTION_METADATA if not metadata.get(field)]
        if missing: raise ValueError(f"production training metadata missing: {missing}")
        if metadata.get("source_kind") != "production_mysql": raise ValueError("production data must originate from the governed exporter")
    return metadata


def main() -> None:
    parser = argparse.ArgumentParser(); parser.add_argument("--input", required=True); parser.add_argument("--metadata"); parser.add_argument("--artifact-root", default="artifacts/runs")
    parser.add_argument("--registry-root", default="model-registry"); parser.add_argument("--dataset-kind", choices=["synthetic", "production"], required=True); parser.add_argument("--ray", action="store_true"); parser.add_argument("--parent-artifact")
    args = parser.parse_args(); source = Path(args.input); metadata = validate_source(source, args.dataset_kind, Path(args.metadata) if args.metadata else None)
    output = Path(args.artifact_root) / datetime.now(timezone.utc).strftime("fraud-%Y%m%dT%H%M%SZ")
    if args.ray:
        if args.parent_artifact: raise ValueError("Ray tuning and parent fine-tuning are separate controlled run modes")
        from .distributed_train import run_trials
        manifest = run_trials(source, output, args.dataset_kind)
    else:
        manifest = train_model(source, output, dataset_kind=args.dataset_kind, resume_artifact=args.parent_artifact)
    lineage = {"source_export": str(source.resolve()), "dataset_kind": args.dataset_kind, "source_metadata": metadata, "trained_at": datetime.now(timezone.utc).isoformat(), "orchestrator": "continuous_train.py", "ray": args.ray}
    (output / "lineage.json").write_text(json.dumps(lineage, indent=2) + "\n")
    record = ModelRegistry(args.registry_root).register(output, "fraud", "candidate")
    from .mlflow_tracking import log_artifact_run
    mlflow_run_id = log_artifact_run(output, "social-protection-fraud")
    print(json.dumps({"manifest": manifest, "registry": record, "mlflow_run_id": mlflow_run_id}, indent=2))


if __name__ == "__main__": main()
