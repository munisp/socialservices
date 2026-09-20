from __future__ import annotations

import json
import shutil
from pathlib import Path
from typing import Any


def run_trials(data: str | Path, output: str | Path, dataset_kind: str = "synthetic") -> dict[str, Any]:
    try:
        import ray
    except ImportError as exc:
        raise RuntimeError("Ray execution requested but ray is not installed") from exc
    from .train import train_model
    source, destination = str(Path(data).resolve()), Path(output)
    destination.parent.mkdir(parents=True, exist_ok=True)
    configs = [{"learning_rate": 1e-3, "batch_size": 256, "seed": 41}, {"learning_rate": 2e-3, "batch_size": 512, "seed": 42}, {"learning_rate": 5e-4, "batch_size": 1024, "seed": 43}]

    @ray.remote(num_cpus=1)
    def trial(config: dict[str, Any], trial_root: str) -> dict[str, Any]:
        from ml.train import train_model as worker_train
        trial_dir = Path(trial_root) / f"trial-{config['seed']}"
        return {"directory": str(trial_dir), "manifest": worker_train(source, trial_dir, epochs=35, seed=config["seed"], batch_size=config["batch_size"], learning_rate=config["learning_rate"], dataset_kind=dataset_kind), "config": config}

    if not ray.is_initialized(): ray.init(address="auto" if __import__("os").getenv("RAY_ADDRESS") else None, include_dashboard=False, ignore_reinit_error=True)
    trial_root = str(destination.parent / (destination.name + "-trials")); results = ray.get([trial.remote(config, trial_root) for config in configs])
    best = max(results, key=lambda result: result["manifest"]["metrics"]["f1"])
    if destination.exists(): shutil.rmtree(destination)
    shutil.copytree(best["directory"], destination)
    manifest_path = destination / "manifest.json"; manifest = json.loads(manifest_path.read_text()); manifest["distributed_training"] = {"engine": "ray", "trials": [{"config": result["config"], "f1": result["manifest"]["metrics"]["f1"]} for result in results], "selected": best["config"]}; manifest_path.write_text(json.dumps(manifest, indent=2) + "\n")
    return manifest
