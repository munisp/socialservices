from __future__ import annotations

import argparse
import csv
import hashlib
import json
import os
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlparse

REQUIRED_METADATA = ["schema_version", "label_provenance", "privacy_approval", "watermark"]


def export_mysql(database_url: str, output: Path) -> int:
    try:
        import pymysql
    except ImportError as exc:
        raise RuntimeError("MySQL export requires pymysql") from exc
    parsed = urlparse(database_url.replace("mysql+mysqlconnector://", "mysql://").replace("mysql+pymysql://", "mysql://"))
    connection = pymysql.connect(host=parsed.hostname, port=parsed.port or 3306, user=parsed.username, password=parsed.password, database=parsed.path.lstrip("/"), cursorclass=pymysql.cursors.DictCursor)
    query = """
      SELECT t.transaction_id, t.amount / 100.0 AS amount_ngn,
             HOUR(t.transaction_date) AS hour, DAYOFYEAR(t.transaction_date) AS day_of_year,
             (SELECT COUNT(*) FROM transactions t2 WHERE t2.beneficiary_id=t.beneficiary_id AND t2.transaction_date BETWEEN DATE_SUB(t.transaction_date, INTERVAL 24 HOUR) AND t.transaction_date) - 1 AS velocity_24h,
             0 AS device_reuse, 0.0 AS distance_km,
             DATEDIFF(t.transaction_date, b.created_at) AS beneficiary_age_days,
             CASE WHEN t.mcc_code IN ('4829','5967','6051','7995') THEN 2 ELSE 0 END AS mcc_risk,
             CASE WHEN fa.status='false_positive' THEN 0 WHEN LOWER(COALESCE(fa.resolution,'')) LIKE '%confirm%' THEN 1 ELSE NULL END AS label,
             t.transaction_date AS event_time
      FROM transactions t JOIN beneficiaries b ON b.id=t.beneficiary_id
      JOIN fraud_alerts fa ON fa.transaction_id=t.id
      WHERE fa.status IN ('resolved','false_positive')
      ORDER BY t.transaction_date
    """
    with connection:
        with connection.cursor() as cursor: cursor.execute(query); rows = [row for row in cursor.fetchall() if row["label"] is not None]
    if not rows: raise RuntimeError("no governed resolved fraud labels were available")
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=list(rows[0]), lineterminator="\n"); writer.writeheader(); writer.writerows(rows)
    return len(rows)


def main() -> None:
    parser = argparse.ArgumentParser(); parser.add_argument("--database-url", default=os.getenv("DATABASE_URL")); parser.add_argument("--out", required=True); parser.add_argument("--metadata", required=True)
    args = parser.parse_args(); metadata = json.loads(Path(args.metadata).read_text()); missing = [key for key in REQUIRED_METADATA if not metadata.get(key)]
    if missing: raise SystemExit(f"metadata missing governance fields: {missing}")
    if not args.database_url: raise SystemExit("DATABASE_URL or --database-url is required")
    output = Path(args.out); count = export_mysql(args.database_url, output)
    metadata.update({"row_count": count, "exported_at": datetime.now(timezone.utc).isoformat(), "output_sha256": hashlib.sha256(output.read_bytes()).hexdigest(), "source_kind": "production_mysql", "known_feature_gaps": ["device_reuse and distance_km are unavailable in the current transaction schema and are conservatively imputed"]})
    output.with_suffix(output.suffix + ".metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")
    print(json.dumps(metadata, indent=2))


if __name__ == "__main__": main()
