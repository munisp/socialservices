# ML/DL/GNN runbook

## Local reproducible pipeline

```bash
python3 -m ml.generate_data --rows 50000 --out data/synthetic_transactions.csv
python3 -m ml.train --data data/synthetic_transactions.csv --out artifacts/fraud --epochs 20
```

The generator is explicitly synthetic. The resulting metrics describe only the generated distribution and must not be used as a production performance claim. The artifact consists of `fraud_mlp.pt`, `preprocess.pt`, and `manifest.json`.

## Production continuous training

Export a labelled, consented and access-controlled lakehouse slice to CSV/Parquet and invoke:

```bash
python3 -m ml.continuous_train --input /lakehouse/silver/fraud_training_export.csv --artifact-root /registry/staging
```

The export must include a watermark, schema version, label provenance, time split, and privacy approval. A production scheduler may wrap this command in Ray; Ray is deliberately optional so CPU-only single-worker training remains valid. The training loop must not be replaced with an LLM prompt.

## GNN and Neo4j

`ml.models.GraphSAGEFraud` is a real PyTorch GraphSAGE-style network and is CPU-compatible. The repository does not yet contain a verified Neo4j driver/export job or graph model weights. Until that is added and validated against an approved graph snapshot, the GNN must remain disabled rather than represented as production-ready.

## Promotion gates

Require a signed manifest, reproducible code/data hashes, temporal holdout metrics, subgroup metrics, calibration, human approval, shadow traffic, delayed-label evaluation, rollback, and PSI/performance alerts. A/B results must be computed from persisted prediction/outcome logs, never simulated constants.
