from __future__ import annotations

import argparse
import csv
import hashlib
import json
from datetime import datetime, timezone
from pathlib import Path

import torch
from torch import nn
from torch.utils.data import DataLoader, TensorDataset

from .generate_credit_data import FEATURES
from .models import CreditRiskMLP
from .train import classification_metrics


def train(data: str | Path, out: str | Path, epochs: int = 35, seed: int = 52) -> dict:
    torch.manual_seed(seed); source = Path(data)
    with source.open(newline="") as handle: rows = list(csv.DictReader(handle))
    x = torch.tensor([[float(row[key]) for key in FEATURES] for row in rows]); y = torch.tensor([float(row["label"]) for row in rows])
    split = int(len(y) * .8); train_x, valid_x, train_y, valid_y = x[:split], x[split:], y[:split], y[split:]
    mean, std = train_x.mean(0), train_x.std(0).clamp_min(1e-6); train_x, valid_x = (train_x - mean) / std, (valid_x - mean) / std
    model = CreditRiskMLP(len(FEATURES)); positive = max(1.0, float(train_y.sum())); loss_fn = nn.BCEWithLogitsLoss(pos_weight=torch.tensor([(len(train_y) - positive) / positive])); optimizer = torch.optim.AdamW(model.parameters(), lr=1e-3, weight_decay=1e-4)
    loader = DataLoader(TensorDataset(train_x, train_y), batch_size=512, shuffle=True, generator=torch.Generator().manual_seed(seed)); best, state = float("inf"), None
    for _ in range(epochs):
        model.train()
        for features, labels in loader: optimizer.zero_grad(set_to_none=True); loss = loss_fn(model(features), labels); loss.backward(); optimizer.step()
        model.eval()
        with torch.inference_mode(): value = float(loss_fn(model(valid_x), valid_y))
        if value < best: best, state = value, {key: tensor.detach().cpu().clone() for key, tensor in model.state_dict().items()}
    if state is None: raise RuntimeError("credit training failed")
    model.load_state_dict(state); model.eval()
    with torch.inference_mode(): probability = torch.sigmoid(model(valid_x))
    output = Path(out); output.mkdir(parents=True, exist_ok=True); weights, preprocess = output / "credit_mlp.pt", output / "preprocess.pt"; torch.save(model.state_dict(), weights); torch.save({"mean": mean, "std": std}, preprocess)
    version = f"credit-risk-{datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')}-{seed}-{len(y)}"; manifest = {"version": version, "task": "credit_risk", "model_type": "pytorch_mlp", "framework": "torch", "features": FEATURES, "weights": weights.name, "preprocess": preprocess.name, "metrics": classification_metrics(valid_y, probability), "dataset_kind": "synthetic", "training_rows": len(y), "seed": seed, "training_data_sha256": hashlib.sha256(source.read_bytes()).hexdigest(), "weights_sha256": hashlib.sha256(weights.read_bytes()).hexdigest(), "cpu_inference": True, "production_validated": False, "prohibited_use": "Do not use this synthetic model to determine social-benefit eligibility or credit access."}; (output / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n"); return manifest


def main() -> None:
    parser = argparse.ArgumentParser(); parser.add_argument("--data", default="data/synthetic_credit.csv"); parser.add_argument("--out", default="artifacts/credit"); parser.add_argument("--epochs", type=int, default=35); args = parser.parse_args(); print(json.dumps(train(args.data, args.out, args.epochs), indent=2))


if __name__ == "__main__": main()
