# ML, deep-learning, and graph-learning runbook

## What is implemented

The repository ships three **real PyTorch artifacts**: a fraud MLP, a credit/payment-default-risk MLP, and a dependency-free GraphSAGE fraud-ring model. Each artifact contains CPU-loadable weights, preprocessing state or graph snapshot, a manifest, hashes, validation metrics, and an explicit `production_validated: false` marker. The shipped fraud candidate was selected from three successful Ray trials. All current metrics are from explicitly synthetic data and are **not evidence of performance on Nigerian production fraud cases**.

| Model | Shipped artifact | Training rows/nodes | Validation F1 | Status |
|---|---|---:|---:|---|
| Fraud MLP | `artifacts/fraud/fraud_mlp.pt` | 50,000 rows | 0.3451 | Staging-only synthetic candidate |
| Credit-risk MLP | `artifacts/credit/credit_mlp.pt` | 30,000 rows | 0.2497 | Staging-only; prohibited for benefit eligibility |
| Fraud-ring GraphSAGE | `artifacts/gnn/graphsage_fraud.pt` | 1,800 nodes / 11,066 directed edges | 0.7391 | Staging-only synthetic graph candidate |

The fraud service performs PyTorch inference on CPU, persists every prediction, accepts delayed investigator outcomes, supports deterministic experiment variants, and reports the exact model version. It no longer contains rule-based scoring or fabricated accuracy.

## Reproduce local training

```bash
python3 -m ml.generate_data --rows 50000 --seed 42 --out data/synthetic_transactions.csv
python3 -m ml.train --data data/synthetic_transactions.csv --out artifacts/fraud --epochs 40 --dataset-kind synthetic
python3 -m ml.generate_credit_data --rows 30000 --seed 52 --out data/synthetic_credit.csv
python3 -m ml.train_credit --data data/synthetic_credit.csv --out artifacts/credit --epochs 35
python3 -m ml.train_gnn --out artifacts/gnn --nodes 1800 --epochs 80
```

The fraud loop uses a temporal 80/20 split, minibatches, class weighting, AdamW, gradient clipping, early stopping, threshold selection, Brier score, confusion counts, data hashes, and feature-reference distributions. Fine-tuning is explicit and records the parent model:

```bash
python3 -m ml.train \
  --data /approved/fraud_training.csv \
  --dataset-kind production \
  --resume-artifact /registry/fraud/previous-version \
  --out /registry/runs/fraud-finetuned
```

## Governed production export

The exporter reads resolved fraud investigations from the application database and refuses unlabeled or ungoverned input. Create a metadata file with `schema_version`, `label_provenance`, `privacy_approval`, and `watermark`, then run:

```bash
python3 -m ml.export_training \
  --database-url "$DATABASE_URL" \
  --metadata /approved/export-metadata.json \
  --out /lakehouse/silver/fraud_training.csv
```

The current transaction schema does not store device identity or geospatial coordinates. The exporter therefore records those feature gaps and conservatively imputes them; production promotion should remain blocked until the schema and collection basis are approved.

## Continuous and distributed training

A governed single-worker run is:

```bash
MLFLOW_TRACKING_URI=http://mlflow:5000 \
python3 -m ml.continuous_train \
  --input /lakehouse/silver/fraud_training.csv \
  --metadata /lakehouse/silver/fraud_training.csv.metadata.json \
  --dataset-kind production \
  --artifact-root /registry/runs \
  --registry-root /registry
```

Add `--ray` for three real Ray PyTorch trials. The repository's Ray path was executed successfully in local multi-process mode. `python-services/ray-service` exposes mounted-path training and distributed batch inference; it never accepts an unbounded in-memory training payload.

`docker-compose.ml.yml` provides MLflow with a PostgreSQL metadata backend, Neo4j, and the Ray service. Set `MLFLOW_DB_PASSWORD` and `NEO4J_PASSWORD` before starting it. Docker Compose syntax could not be runtime-validated in the audit sandbox because Docker is not installed there.

## Registry and promotion

`ml.registry.ModelRegistry` copies immutable version directories, records every file checksum, rejects version collisions, and requires an approver and reason for stage changes. Synthetic artifacts in this repository are promoted only to `staging`, never `production`:

```bash
python3 -m ml.registry --root model-registry register --artifact artifacts/fraud --model fraud
python3 -m ml.registry --root model-registry promote \
  --model fraud --version VERSION --stage staging \
  --approver USER --reason "Synthetic pipeline validation only"
```

A production promotion additionally requires governed real labels, temporal and geographic holdouts, subgroup/fairness analysis, calibration, shadow traffic, delayed-label performance, privacy/security approval, rollback rehearsal, and an accountable human approver.

## A/B experiments and monitoring

`ml.experiments.ExperimentStore` and the fraud service use SHA-256-based sticky assignment, SQLite prediction logs, model-version capture, and delayed outcomes. Configure multiple artifact paths with `MODEL_VARIANTS_JSON` and set `MODEL_EXPERIMENT_ID`. Submit reviewed outcomes to `POST /v1/outcomes`; inspect computed per-variant metrics at `GET /v1/experiments/{id}/metrics`.

Run persisted drift and performance evaluation with:

```bash
python3 -m ml.evaluate_monitoring \
  --manifest artifacts/fraud/manifest.json \
  --experiment-db /var/lib/fraud-ml/predictions.sqlite \
  --out /var/lib/fraud-ml/reports/latest.json
```

The report computes feature PSI, delayed-label precision/recall, alert status, and explicit `insufficient_labels` state. An operations scheduler must execute this command and route non-normal alerts; no scheduler is silently assumed by this repository.

## Neo4j and GraphSAGE

Export a real graph snapshot only with configured credentials:

```bash
python3 -m ml.neo4j_graph --out /approved/beneficiary-graph.pt
python3 -m ml.train_gnn \
  --graph /approved/beneficiary-graph.pt \
  --resume-artifact artifacts/gnn \
  --out /registry/runs/gnn-finetuned
```

The exporter verifies Neo4j connectivity, requires at least 100 nodes and an edge, preserves stable node identifiers, and records source metadata. The GraphSAGE implementation and its fine-tuning path were both executed on CPU during this audit.

## Validation

```bash
python3 -m unittest -v ml.test_ml
python3 -m py_compile ml/*.py services/fraud-ml/*.py python-services/ray-service/app.py
```

The tests cover deterministic generation, drift boundaries, CPU inference for all three models, registry integrity and promotion, sticky experiments and delayed outcomes, and production metadata refusal.
