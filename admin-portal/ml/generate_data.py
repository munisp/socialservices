"""Generate realistic, explicitly synthetic Nigerian social-protection transactions.

The generator is deterministic and labels are generated from latent risk mechanisms so
metrics are honest and reproducible. It must never be confused with production data.
"""
from __future__ import annotations
import argparse, csv, math, random
from datetime import datetime, timedelta, timezone
from pathlib import Path

STATES = [("Lagos", .18), ("Kano", .12), ("Kaduna", .10), ("Rivers", .09), ("Oyo", .08), ("Borno", .06), ("FCT", .10), ("Katsina", .07), ("Anambra", .06), ("Enugu", .05), ("Other", .09)]
TX_TYPES = ["disbursement", "merchant_payment", "cashout", "refund"]

def weighted_state(rng):
    x, total = rng.random(), 0
    for state, weight in STATES:
        total += weight
        if x <= total: return state
    return STATES[-1][0]

def generate(n: int, seed: int = 42):
    rng = random.Random(seed)
    start = datetime(2024, 1, 1, tzinfo=timezone.utc)
    rows = []
    for i in range(n):
        state = weighted_state(rng)
        tx_type = rng.choices(TX_TYPES, weights=[.48, .35, .12, .05])[0]
        hour = rng.choices(range(24), weights=[1,1,1,1,1,2,5,8,9,8,7,7,7,7,8,9,10,10,9,8,6,4,2,1])[0]
        day = rng.randrange(365)
        amount = max(500, round(rng.lognormvariate(math.log(8500 if tx_type == "disbursement" else 3500), .65) / 100) * 100)
        device_reuse = rng.random() < .12
        velocity_24h = max(0, int(rng.expovariate(1/2.5)) - 1)
        beneficiary_age_days = rng.randint(1, 1800)
        distance_km = max(0, rng.gauss(8, 15))
        mcc_risk = rng.choice([0, 0, 0, 1, 2])
        synthetic_risk = (.04 + .08*(amount > 50000) + .11*(hour < 5 or hour > 22) + .12*device_reuse + .14*(velocity_24h >= 8) + .10*(distance_km > 80) + .08*(mcc_risk == 2) + .12*(beneficiary_age_days < 14) + .05*(tx_type == "cashout"))
        label = int(rng.random() < min(.92, synthetic_risk))
        rows.append({"transaction_id": f"syn-{seed}-{i:09d}", "amount_ngn": amount, "hour": hour, "day_of_year": day, "velocity_24h": velocity_24h, "device_reuse": int(device_reuse), "distance_km": round(distance_km, 3), "beneficiary_age_days": beneficiary_age_days, "mcc_risk": mcc_risk, "state": state, "transaction_type": tx_type, "label": label, "synthetic": 1, "event_time": (start + timedelta(days=day, hours=hour)).isoformat()})
    return rows

def main():
    p = argparse.ArgumentParser(); p.add_argument("--rows", type=int, default=50000); p.add_argument("--seed", type=int, default=42); p.add_argument("--out", default="data/synthetic_transactions.csv"); a = p.parse_args()
    out = Path(a.out); out.parent.mkdir(parents=True, exist_ok=True)
    rows = generate(a.rows, a.seed)
    with out.open("w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0])); w.writeheader(); w.writerows(rows)
    print(f"wrote {len(rows)} explicitly synthetic rows to {out}")
if __name__ == "__main__": main()
