from __future__ import annotations

import argparse
import hashlib
import json
import random
from datetime import datetime, timezone
from pathlib import Path

import torch
from torch import nn

from .models import GraphSAGEFraud
from .train import classification_metrics

FEATURES = ["transaction_volume", "unique_devices", "unique_merchants", "cashout_ratio", "velocity", "account_age_days"]


def synthetic_graph(nodes: int, seed: int) -> tuple[torch.Tensor, torch.Tensor, torch.Tensor]:
    rng = random.Random(seed); features, labels = [], []
    ring_nodes = set(rng.sample(range(nodes), max(4, nodes // 12)))
    for node in range(nodes):
        risky = node in ring_nodes; volume = rng.lognormvariate(9.5 if risky else 8.2, .7); devices = rng.randint(5, 15) if risky else rng.randint(1, 4); merchants = rng.randint(2, 8); cashout = min(1.0, max(0.0, rng.gauss(.7 if risky else .25, .15))); velocity = rng.randint(12, 40) if risky else rng.randint(0, 10); age = rng.randint(10, 1800)
        features.append([volume, devices, merchants, cashout, velocity, age]); labels.append(float(risky and rng.random() > .08 or (not risky and rng.random() < .025)))
    edges: set[tuple[int, int]] = set()
    ring = sorted(ring_nodes)
    for index, node in enumerate(ring): edges.add((node, ring[(index + 1) % len(ring)])); edges.add((ring[(index + 1) % len(ring)], node))
    for node in range(nodes):
        for _ in range(3):
            other = rng.randrange(nodes)
            if other != node: edges.add((node, other)); edges.add((other, node))
    return torch.tensor(features, dtype=torch.float32), torch.tensor(sorted(edges), dtype=torch.long).t().contiguous(), torch.tensor(labels, dtype=torch.float32)


def train(out: str | Path, nodes: int = 1500, epochs: int = 80, seed: int = 62, graph_path: str | Path | None = None, resume_artifact: str | Path | None = None) -> dict:
    torch.manual_seed(seed)
    if graph_path:
        snapshot = torch.load(graph_path, map_location="cpu", weights_only=True); x, edge_index, labels = snapshot["features"].float(), snapshot["edge_index"].long(), snapshot["labels"].float(); nodes = x.shape[0]; dataset_kind = "neo4j_snapshot"
        if snapshot.get("feature_names", FEATURES) != FEATURES: raise ValueError("Neo4j graph feature contract mismatch")
    else:
        x, edge_index, labels = synthetic_graph(nodes, seed); dataset_kind = "synthetic_graph"
    split = int(nodes * .8); mean, std = x[:split].mean(0), x[:split].std(0).clamp_min(1e-6); normalized = (x - mean) / std
    model = GraphSAGEFraud(len(FEATURES)); parent_version = None
    if resume_artifact:
        parent = Path(resume_artifact); parent_manifest = json.loads((parent / "manifest.json").read_text()); model.load_state_dict(torch.load(parent / parent_manifest["weights"], map_location="cpu", weights_only=True)); parent_version = parent_manifest["version"]
    positive = max(1.0, float(labels[:split].sum())); loss_fn = nn.BCEWithLogitsLoss(pos_weight=torch.tensor([(split - positive) / positive])); optimizer = torch.optim.AdamW(model.parameters(), lr=3e-3, weight_decay=1e-4); best, state = float("inf"), None
    for _ in range(epochs):
        model.train(); optimizer.zero_grad(set_to_none=True); logits = model(normalized, edge_index); loss = loss_fn(logits[:split], labels[:split]); loss.backward(); torch.nn.utils.clip_grad_norm_(model.parameters(), 5.0); optimizer.step()
        model.eval()
        with torch.inference_mode(): value = float(loss_fn(model(normalized, edge_index)[split:], labels[split:]))
        if value < best: best, state = value, {key: tensor.detach().cpu().clone() for key, tensor in model.state_dict().items()}
    if state is None: raise RuntimeError("GNN training failed")
    model.load_state_dict(state); model.eval()
    with torch.inference_mode(): probability = torch.sigmoid(model(normalized, edge_index)[split:])
    output = Path(out); output.mkdir(parents=True, exist_ok=True); weights = output / "graphsage_fraud.pt"; graph = output / "graph_snapshot.pt"; torch.save(model.state_dict(), weights); torch.save({"features": x, "edge_index": edge_index, "labels": labels, "mean": mean, "std": std}, graph)
    manifest = {"version": f"fraud-gnn-{datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')}-{seed}-{nodes}", "task": "fraud_ring_detection", "model_type": "pytorch_graphsage", "framework": "torch", "features": FEATURES, "weights": weights.name, "graph_snapshot": graph.name, "metrics": classification_metrics(labels[split:], probability), "dataset_kind": dataset_kind, "source_graph": str(Path(graph_path).resolve()) if graph_path else None, "parent_version": parent_version, "nodes": nodes, "edges": edge_index.shape[1], "seed": seed, "weights_sha256": hashlib.sha256(weights.read_bytes()).hexdigest(), "cpu_inference": True, "production_validated": False}; (output / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n"); return manifest


def main() -> None:
    parser = argparse.ArgumentParser(); parser.add_argument("--out", default="artifacts/gnn"); parser.add_argument("--nodes", type=int, default=1500); parser.add_argument("--epochs", type=int, default=80); parser.add_argument("--graph"); parser.add_argument("--resume-artifact"); args = parser.parse_args(); print(json.dumps(train(args.out, args.nodes, args.epochs, graph_path=args.graph, resume_artifact=args.resume_artifact), indent=2))


if __name__ == "__main__": main()
