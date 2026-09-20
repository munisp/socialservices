from __future__ import annotations

import hashlib
import json
import sqlite3
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


class ExperimentStore:
    def __init__(self, path: str | Path):
        self.path = Path(path); self.path.parent.mkdir(parents=True, exist_ok=True); self._init()

    def connect(self) -> sqlite3.Connection:
        connection = sqlite3.connect(self.path); connection.row_factory = sqlite3.Row; return connection

    def _init(self) -> None:
        with self.connect() as db:
            db.executescript("""
            CREATE TABLE IF NOT EXISTS experiments(id TEXT PRIMARY KEY, variants_json TEXT NOT NULL, active INTEGER NOT NULL, created_at TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS predictions(id INTEGER PRIMARY KEY AUTOINCREMENT, experiment_id TEXT, subject_id TEXT NOT NULL, transaction_id TEXT NOT NULL UNIQUE, variant TEXT NOT NULL, model_version TEXT NOT NULL, score REAL NOT NULL, decision TEXT NOT NULL, features_json TEXT NOT NULL, created_at TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS outcomes(transaction_id TEXT PRIMARY KEY, label INTEGER NOT NULL CHECK(label IN (0,1)), source TEXT NOT NULL, observed_at TEXT NOT NULL, FOREIGN KEY(transaction_id) REFERENCES predictions(transaction_id));
            """)

    def create_experiment(self, experiment_id: str, variants: dict[str, float]) -> None:
        if len(variants) < 2 or abs(sum(variants.values()) - 1.0) > 1e-6 or any(value <= 0 for value in variants.values()): raise ValueError("variants must contain positive allocations that sum to 1")
        with self.connect() as db: db.execute("INSERT INTO experiments VALUES(?,?,1,?)", (experiment_id, json.dumps(variants, sort_keys=True), datetime.now(timezone.utc).isoformat()))

    def assign(self, experiment_id: str, subject_id: str) -> str:
        with self.connect() as db:
            row = db.execute("SELECT variants_json, active FROM experiments WHERE id=?", (experiment_id,)).fetchone()
        if not row or not row["active"]: raise KeyError("experiment is not active")
        position = int(hashlib.sha256(f"{experiment_id}:{subject_id}".encode()).hexdigest()[:16], 16) / float(0xFFFFFFFFFFFFFFFF)
        cumulative = 0.0
        for name, allocation in json.loads(row["variants_json"]).items():
            cumulative += allocation
            if position < cumulative: return name
        return list(json.loads(row["variants_json"]))[-1]

    def log_prediction(self, experiment_id: str | None, subject_id: str, transaction_id: str, variant: str, model_version: str, score: float, decision: str, features: dict[str, float]) -> None:
        with self.connect() as db:
            db.execute("INSERT INTO predictions(experiment_id,subject_id,transaction_id,variant,model_version,score,decision,features_json,created_at) VALUES(?,?,?,?,?,?,?,?,?)",
                       (experiment_id, subject_id, transaction_id, variant, model_version, score, decision, json.dumps(features, sort_keys=True), datetime.now(timezone.utc).isoformat()))

    def record_outcome(self, transaction_id: str, label: int, source: str) -> None:
        if label not in (0, 1) or not source.strip(): raise ValueError("binary label and provenance source are required")
        with self.connect() as db:
            exists = db.execute("SELECT 1 FROM predictions WHERE transaction_id=?", (transaction_id,)).fetchone()
            if not exists: raise KeyError("prediction not found")
            db.execute("INSERT OR REPLACE INTO outcomes VALUES(?,?,?,?)", (transaction_id, label, source, datetime.now(timezone.utc).isoformat()))

    def metrics(self, experiment_id: str) -> list[dict[str, Any]]:
        with self.connect() as db:
            rows = db.execute("SELECT p.variant,p.score,o.label FROM predictions p JOIN outcomes o USING(transaction_id) WHERE p.experiment_id=?", (experiment_id,)).fetchall()
        grouped: dict[str, list[sqlite3.Row]] = {}
        for row in rows: grouped.setdefault(row["variant"], []).append(row)
        result = []
        for variant, values in sorted(grouped.items()):
            tp = sum(v["score"] >= .5 and v["label"] == 1 for v in values); fp = sum(v["score"] >= .5 and v["label"] == 0 for v in values); fn = sum(v["score"] < .5 and v["label"] == 1 for v in values)
            precision = tp / max(1, tp + fp); recall = tp / max(1, tp + fn)
            result.append({"variant": variant, "labelled_predictions": len(values), "precision": precision, "recall": recall, "f1": 2 * precision * recall / max(1e-9, precision + recall)})
        return result
