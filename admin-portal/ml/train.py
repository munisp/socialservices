from __future__ import annotations

import argparse
import csv
import hashlib
import json
import random
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import torch
from torch import nn
from torch.utils.data import DataLoader, TensorDataset

from .models import FraudMLP

FEATURES = ["amount_ngn", "hour", "day_of_year", "velocity_24h", "device_reuse", "distance_km", "beneficiary_age_days", "mcc_risk"]


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_rows(path: str | Path) -> list[dict[str, str]]:
    source = Path(path)
    if source.suffix.lower() == ".parquet":
        try:
            import pandas as pd
        except ImportError as exc:
            raise RuntimeError("Parquet input requires pandas and pyarrow") from exc
        return [{key: str(value) for key, value in row.items()} for row in pd.read_parquet(source).to_dict("records")]
    with source.open(newline="") as handle:
        return list(csv.DictReader(handle))


def load_dataset(path: str | Path) -> tuple[torch.Tensor, torch.Tensor, list[dict[str, str]]]:
    rows = load_rows(path)
    if len(rows) < 100:
        raise ValueError("at least 100 labelled rows are required")
    missing = [field for field in [*FEATURES, "label"] if field not in rows[0]]
    if missing:
        raise ValueError(f"training data missing required columns: {missing}")
    rows.sort(key=lambda row: row.get("event_time", row.get("transaction_date", "")))
    x = torch.tensor([[float(row[key]) for key in FEATURES] for row in rows], dtype=torch.float32)
    y = torch.tensor([float(row["label"]) for row in rows], dtype=torch.float32)
    if not torch.isfinite(x).all() or not torch.isfinite(y).all():
        raise ValueError("training data contains non-finite values")
    if y.unique().numel() < 2:
        raise ValueError("training data must contain both positive and negative labels")
    return x, y, rows


def classification_metrics(y: torch.Tensor, p: torch.Tensor, threshold: float = 0.5) -> dict[str, float | int]:
    pred = (p >= threshold).int(); actual = y.int()
    tp = int(((pred == 1) & (actual == 1)).sum()); tn = int(((pred == 0) & (actual == 0)).sum())
    fp = int(((pred == 1) & (actual == 0)).sum()); fn = int(((pred == 0) & (actual == 1)).sum())
    precision = tp / max(1, tp + fp); recall = tp / max(1, tp + fn)
    return {"accuracy": (tp + tn) / max(1, len(y)), "precision": precision, "recall": recall,
            "f1": 2 * precision * recall / max(1e-9, precision + recall),
            "brier": float(torch.mean((p - y) ** 2)), "support": len(y), "threshold": threshold,
            "tp": tp, "tn": tn, "fp": fp, "fn": fn}


def best_threshold(y: torch.Tensor, p: torch.Tensor) -> float:
    candidates = [i / 100 for i in range(10, 91, 2)]
    return max(candidates, key=lambda value: float(classification_metrics(y, p, value)["f1"]))


def feature_summary(x: torch.Tensor) -> dict[str, dict[str, Any]]:
    result: dict[str, dict[str, Any]] = {}
    for index, feature in enumerate(FEATURES):
        values = x[:, index]
        quantiles = torch.quantile(values, torch.tensor([0.0, .1, .25, .5, .75, .9, 1.0]))
        result[feature] = {"mean": float(values.mean()), "std": float(values.std()), "quantiles": [float(v) for v in quantiles]}
    return result


def train_model(data: str | Path, out: str | Path, epochs: int = 40, seed: int = 42,
                batch_size: int = 512, learning_rate: float = 2e-3, patience: int = 8,
                dataset_kind: str = "synthetic", resume_artifact: str | Path | None = None) -> dict[str, Any]:
    random.seed(seed); torch.manual_seed(seed); torch.set_num_threads(max(1, min(4, torch.get_num_threads())))
    source, output = Path(data), Path(out)
    x, y, rows = load_dataset(source)
    validation_size = max(20, int(len(y) * .2)); split = len(y) - validation_size
    train_x, valid_x = x[:split], x[split:]; train_y, valid_y = y[:split], y[split:]
    mean, std = train_x.mean(0), train_x.std(0).clamp_min(1e-6)
    train_x, valid_x = (train_x - mean) / std, (valid_x - mean) / std
    loader = DataLoader(TensorDataset(train_x, train_y), batch_size=min(batch_size, len(train_y)), shuffle=True,
                        generator=torch.Generator().manual_seed(seed))
    model = FraudMLP(len(FEATURES)); parent_version = None
    if resume_artifact:
        parent = Path(resume_artifact); parent_manifest = json.loads((parent / "manifest.json").read_text())
        if parent_manifest.get("features") != FEATURES: raise ValueError("fine-tuning feature contract does not match parent model")
        model.load_state_dict(torch.load(parent / parent_manifest["weights"], map_location="cpu", weights_only=True)); parent_version = parent_manifest["version"]
    positive = max(1.0, float(train_y.sum())); negative = max(1.0, len(train_y) - positive)
    loss_fn = nn.BCEWithLogitsLoss(pos_weight=torch.tensor([negative / positive])); optimizer = torch.optim.AdamW(model.parameters(), lr=learning_rate, weight_decay=1e-4)
    best_loss, best_state, stale, history = float("inf"), None, 0, []
    for epoch in range(1, epochs + 1):
        model.train(); total = 0.0
        for features, labels in loader:
            optimizer.zero_grad(set_to_none=True); loss = loss_fn(model(features), labels); loss.backward()
            torch.nn.utils.clip_grad_norm_(model.parameters(), 5.0); optimizer.step(); total += float(loss.detach()) * len(labels)
        model.eval()
        with torch.inference_mode(): validation_loss = float(loss_fn(model(valid_x), valid_y))
        history.append({"epoch": epoch, "train_loss": total / len(train_y), "validation_loss": validation_loss})
        if validation_loss < best_loss - 1e-5:
            best_loss, best_state, stale = validation_loss, {key: value.detach().cpu().clone() for key, value in model.state_dict().items()}, 0
        else:
            stale += 1
            if stale >= patience: break
    if best_state is None: raise RuntimeError("training did not produce a model state")
    model.load_state_dict(best_state); model.eval()
    with torch.inference_mode(): probabilities = torch.sigmoid(model(valid_x))
    threshold = best_threshold(valid_y, probabilities); metrics = classification_metrics(valid_y, probabilities, threshold)
    output.mkdir(parents=True, exist_ok=True)
    weights_path, preprocessing_path = output / "fraud_mlp.pt", output / "preprocess.pt"
    torch.save(model.state_dict(), weights_path); torch.save({"mean": mean, "std": std}, preprocessing_path)
    version = f"fraud-ml-{datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')}-{seed}-{len(y)}"
    manifest = {"version": version, "model_type": "pytorch_mlp", "task": "fraud_detection", "framework": "torch",
                "framework_version": torch.__version__, "features": FEATURES, "weights": weights_path.name,
                "preprocess": preprocessing_path.name, "metrics": metrics, "dataset_kind": dataset_kind,
                "training_rows": len(y), "positive_rate": float(y.mean()), "split": "temporal_80_20",
                "seed": seed, "epochs_completed": len(history), "training_history": history,
                "parent_version": parent_version,
                "training_data_sha256": sha256(source), "training_window": {"first": rows[0].get("event_time"), "last": rows[-1].get("event_time")},
                "feature_reference": feature_summary(x), "weights_sha256": sha256(weights_path),
                "preprocess_sha256": sha256(preprocessing_path), "cpu_inference": True,
                "production_validated": False, "limitations": ["Synthetic metrics are not evidence of real fraud performance"] if dataset_kind == "synthetic" else []}
    (output / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    return manifest


def main() -> None:
    parser = argparse.ArgumentParser(); parser.add_argument("--data", default="data/synthetic_transactions.csv"); parser.add_argument("--out", default="artifacts/fraud")
    parser.add_argument("--epochs", type=int, default=40); parser.add_argument("--seed", type=int, default=42); parser.add_argument("--batch-size", type=int, default=512)
    parser.add_argument("--learning-rate", type=float, default=2e-3); parser.add_argument("--patience", type=int, default=8); parser.add_argument("--dataset-kind", choices=["synthetic", "production"], default="synthetic")
    parser.add_argument("--resume-artifact"); args = parser.parse_args(); manifest = train_model(args.data, args.out, args.epochs, args.seed, args.batch_size, args.learning_rate, args.patience, args.dataset_kind, args.resume_artifact)
    print(json.dumps({"version": manifest["version"], "metrics": manifest["metrics"]}, indent=2))


if __name__ == "__main__": main()
