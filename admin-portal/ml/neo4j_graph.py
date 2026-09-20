from __future__ import annotations

import argparse
import json
import os
from datetime import datetime, timezone
from pathlib import Path

import torch

FEATURES = ["transaction_volume", "unique_devices", "unique_merchants", "cashout_ratio", "velocity", "account_age_days"]


def export(uri: str, user: str, password: str, database: str, output: str | Path) -> dict:
    try:
        from neo4j import GraphDatabase
    except ImportError as exc:
        raise RuntimeError("Neo4j export requires the neo4j Python driver") from exc
    node_query = "MATCH (b:Beneficiary) RETURN b.id AS id, coalesce(b.transaction_volume,0.0) AS transaction_volume, coalesce(b.unique_devices,0.0) AS unique_devices, coalesce(b.unique_merchants,0.0) AS unique_merchants, coalesce(b.cashout_ratio,0.0) AS cashout_ratio, coalesce(b.velocity,0.0) AS velocity, coalesce(b.account_age_days,0.0) AS account_age_days, coalesce(b.confirmed_fraud,false) AS label ORDER BY id"
    edge_query = "MATCH (a:Beneficiary)-[:USES_DEVICE|TRANSACTED_WITH|SHARES_ACCOUNT]-(b:Beneficiary) WHERE a.id < b.id RETURN a.id AS source, b.id AS target"
    with GraphDatabase.driver(uri, auth=(user, password)) as driver:
        driver.verify_connectivity()
        with driver.session(database=database) as session: nodes = [dict(record) for record in session.run(node_query)]; edges = [dict(record) for record in session.run(edge_query)]
    if len(nodes) < 100 or not edges: raise RuntimeError("Neo4j snapshot requires at least 100 nodes and one edge")
    identifiers = {str(row["id"]): index for index, row in enumerate(nodes)}; feature_tensor = torch.tensor([[float(row[key]) for key in FEATURES] for row in nodes]); labels = torch.tensor([float(row["label"]) for row in nodes]); pairs = []
    for row in edges:
        source, target = identifiers.get(str(row["source"])), identifiers.get(str(row["target"]))
        if source is not None and target is not None: pairs.extend([(source, target), (target, source)])
    edge_index = torch.tensor(pairs, dtype=torch.long).t().contiguous(); path = Path(output); path.parent.mkdir(parents=True, exist_ok=True); torch.save({"features": feature_tensor, "edge_index": edge_index, "labels": labels, "node_ids": list(identifiers), "feature_names": FEATURES}, path)
    metadata = {"source": uri, "database": database, "nodes": len(nodes), "edges": edge_index.shape[1], "exported_at": datetime.now(timezone.utc).isoformat(), "contains_labels": bool(labels.sum())}; path.with_suffix(".metadata.json").write_text(json.dumps(metadata, indent=2) + "\n"); return metadata


def main() -> None:
    parser = argparse.ArgumentParser(); parser.add_argument("--uri", default=os.getenv("NEO4J_URI")); parser.add_argument("--user", default=os.getenv("NEO4J_USER")); parser.add_argument("--password", default=os.getenv("NEO4J_PASSWORD")); parser.add_argument("--database", default=os.getenv("NEO4J_DATABASE", "neo4j")); parser.add_argument("--out", required=True); args = parser.parse_args()
    if not all([args.uri, args.user, args.password]): raise SystemExit("NEO4J_URI, NEO4J_USER and NEO4J_PASSWORD are required")
    print(json.dumps(export(args.uri, args.user, args.password, args.database, args.out), indent=2))


if __name__ == "__main__": main()
