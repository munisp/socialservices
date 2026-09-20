from __future__ import annotations

import argparse
import json
import sqlite3
from datetime import datetime, timezone
from pathlib import Path

from .monitor import drift_status, psi


def evaluate(manifest_path: str | Path, experiment_db: str | Path, report_path: str | Path, minimum_labels: int = 50) -> dict:
    manifest = json.loads(Path(manifest_path).read_text()); connection = sqlite3.connect(experiment_db); connection.row_factory = sqlite3.Row
    predictions = connection.execute("SELECT score, features_json FROM predictions WHERE model_version=?", (manifest["version"],)).fetchall(); outcomes = connection.execute("SELECT p.score,o.label FROM predictions p JOIN outcomes o USING(transaction_id) WHERE p.model_version=?", (manifest["version"],)).fetchall(); connection.close()
    feature_scores = {}
    for feature, reference in manifest.get("feature_reference", {}).items():
        current = [json.loads(row["features_json"]).get(feature) for row in predictions]; current = [float(value) for value in current if value is not None]
        quantiles = reference.get("quantiles", []); score = psi([float(value) for value in quantiles], current) if current else None; feature_scores[feature] = {"psi": score, "status": drift_status(score)}
    performance = {"status": "insufficient_labels", "labelled": len(outcomes)}
    if len(outcomes) >= minimum_labels:
        tp = sum(row["score"] >= .5 and row["label"] == 1 for row in outcomes); fp = sum(row["score"] >= .5 and row["label"] == 0 for row in outcomes); fn = sum(row["score"] < .5 and row["label"] == 1 for row in outcomes); precision = tp / max(1, tp + fp); recall = tp / max(1, tp + fn); performance = {"status": "degraded" if recall < .5 else "normal", "labelled": len(outcomes), "precision": precision, "recall": recall}
    overall = "critical" if any(value["status"] == "critical" for value in feature_scores.values()) else "warning" if any(value["status"] == "warning" for value in feature_scores.values()) or performance["status"] == "degraded" else "normal"
    report = {"model_version": manifest["version"], "evaluated_at": datetime.now(timezone.utc).isoformat(), "prediction_count": len(predictions), "features": feature_scores, "performance": performance, "status": overall, "alerts": [f"feature_drift:{name}:{value['status']}" for name, value in feature_scores.items() if value["status"] != "normal"] + (["performance_degraded"] if performance["status"] == "degraded" else [])}; output = Path(report_path); output.parent.mkdir(parents=True, exist_ok=True); output.write_text(json.dumps(report, indent=2) + "\n"); return report


def main() -> None:
    parser = argparse.ArgumentParser(); parser.add_argument("--manifest", required=True); parser.add_argument("--experiment-db", required=True); parser.add_argument("--out", required=True); parser.add_argument("--minimum-labels", type=int, default=50); args = parser.parse_args(); print(json.dumps(evaluate(args.manifest, args.experiment_db, args.out, args.minimum_labels), indent=2))


if __name__ == "__main__": main()
