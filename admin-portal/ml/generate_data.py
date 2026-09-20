"""Generate reproducible, explicitly synthetic Nigerian social-protection transactions.

The distributions model repeated beneficiaries, device sharing, agent/merchant channels,
geography, payment timing, velocity, and delayed fraud mechanisms. They are useful for
pipeline testing only and are never represented as observed Nigerian fraud prevalence.
"""
from __future__ import annotations

import argparse
import csv
import math
import random
from datetime import datetime, timedelta, timezone
from pathlib import Path

STATES = [("Lagos", .18), ("Kano", .12), ("Kaduna", .10), ("Rivers", .09), ("Oyo", .08), ("Borno", .06), ("FCT", .10), ("Katsina", .07), ("Anambra", .06), ("Enugu", .05), ("Other", .09)]
TX_TYPES = ["disbursement", "merchant_payment", "cashout", "refund"]
CHANNELS = ["bank", "mobile_money", "agent", "merchant"]


def weighted_state(rng: random.Random) -> str:
    value, total = rng.random(), 0.0
    for state, weight in STATES:
        total += weight
        if value <= total: return state
    return STATES[-1][0]


def generate(n: int, seed: int = 42) -> list[dict[str, object]]:
    if n < 1: return []
    rng = random.Random(seed); start = datetime(2024, 1, 1, tzinfo=timezone.utc); beneficiary_count = max(250, n // 8)
    beneficiaries = [{"id": f"ben-{index:07d}", "state": weighted_state(rng), "created_day": rng.randrange(0, 300), "home_latent": rng.gauss(0, 1), "device": f"dev-{rng.randrange(max(100, beneficiary_count // 2)):07d}"} for index in range(beneficiary_count)]
    activity: dict[str, list[int]] = {}; rows: list[dict[str, object]] = []
    for index in range(n):
        beneficiary = beneficiaries[rng.randrange(beneficiary_count)]; day = rng.randrange(365); tx_type = rng.choices(TX_TYPES, weights=[.43, .38, .14, .05])[0]; channel = rng.choices(CHANNELS, weights=[.28, .34, .20, .18])[0]
        hour = rng.choices(range(24), weights=[1,1,1,1,1,2,5,8,9,8,7,7,7,7,8,9,10,10,9,8,6,4,2,1])[0]
        base = 11000 if tx_type == "disbursement" else 4800 if tx_type == "merchant_payment" else 9000; amount = max(500, round(rng.lognormvariate(math.log(base), .62) / 100) * 100)
        history = activity.setdefault(beneficiary["id"], []); velocity_24h = sum(abs(day - prior) <= 1 for prior in history[-20:]); history.append(day)
        collusive = rng.random() < .035; device_id = f"shared-{rng.randrange(40):04d}" if collusive else beneficiary["device"]; device_reuse = collusive or rng.random() < .10; distance_km = max(0.0, rng.gauss(120 if collusive else 9, 35 if collusive else 14)); merchant_id = f"merchant-{rng.randrange(350):05d}"; mcc_risk = 2 if collusive and rng.random() < .55 else rng.choices([0,1,2], weights=[.78,.17,.05])[0]; age_days = max(1, day - int(beneficiary["created_day"]) + 30)
        state_effect = .035 if beneficiary["state"] in {"Lagos", "FCT", "Rivers"} else 0.0; logit = -3.7 + .000018 * amount + .55 * (hour < 5 or hour > 22) + .85 * device_reuse + .16 * velocity_24h + .012 * distance_km + .7 * (mcc_risk == 2) + .65 * (age_days < 30) + .45 * (tx_type == "cashout") + .9 * collusive + state_effect
        probability = 1 / (1 + math.exp(-min(8.0, logit))); label = int(rng.random() < probability)
        rows.append({"transaction_id": f"syn-{seed}-{index:09d}", "beneficiary_id": beneficiary["id"], "device_id": device_id, "merchant_id": merchant_id, "channel": channel, "amount_ngn": amount, "hour": hour, "day_of_year": day, "velocity_24h": velocity_24h, "device_reuse": int(device_reuse), "distance_km": round(distance_km, 3), "beneficiary_age_days": age_days, "mcc_risk": mcc_risk, "state": beneficiary["state"], "transaction_type": tx_type, "label": label, "synthetic": 1, "event_time": (start + timedelta(days=day, hours=hour)).isoformat()})
    return rows


def main() -> None:
    parser = argparse.ArgumentParser(); parser.add_argument("--rows", type=int, default=50000); parser.add_argument("--seed", type=int, default=42); parser.add_argument("--out", default="data/synthetic_transactions.csv"); args = parser.parse_args(); output = Path(args.out); output.parent.mkdir(parents=True, exist_ok=True); rows = generate(args.rows, args.seed)
    if not rows: raise SystemExit("--rows must be positive")
    with output.open("w", newline="") as handle: writer = csv.DictWriter(handle, fieldnames=list(rows[0]), lineterminator="\n"); writer.writeheader(); writer.writerows(rows)
    print(f"wrote {len(rows)} explicitly synthetic rows to {output}")


if __name__ == "__main__": main()
