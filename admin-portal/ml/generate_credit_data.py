from __future__ import annotations

import argparse
import csv
import math
import random
from pathlib import Path

FEATURES = ["income_ngn", "benefit_amount_ngn", "household_size", "dependency_ratio", "payment_failures_90d", "account_age_days", "income_volatility", "debt_service_ratio"]


def generate(rows: int, seed: int = 52) -> list[dict[str, float | int | str]]:
    rng = random.Random(seed); output = []
    for index in range(rows):
        income = max(12000.0, rng.lognormvariate(math.log(70000), .65)); benefit = rng.choice([5000, 7500, 10000, 15000, 25000]); household = rng.randint(1, 12)
        dependency = min(3.0, max(0.0, rng.gauss(.9, .55))); failures = min(8, int(rng.expovariate(1.5))); age = rng.randint(10, 2500); volatility = min(2.0, max(0.0, rng.gauss(.45, .3))); debt = min(1.5, max(0.0, rng.gauss(.32, .22)))
        logit = -3.0 + .6 * failures + 1.2 * debt + .65 * volatility + .3 * dependency + .25 * (age < 90) - .000006 * income
        probability = 1 / (1 + math.exp(-logit)); label = int(rng.random() < probability)
        output.append({"account_id": f"credit-syn-{seed}-{index:08d}", "income_ngn": round(income, 2), "benefit_amount_ngn": benefit, "household_size": household, "dependency_ratio": round(dependency, 4), "payment_failures_90d": failures, "account_age_days": age, "income_volatility": round(volatility, 4), "debt_service_ratio": round(debt, 4), "label": label, "synthetic": 1})
    return output


def main() -> None:
    parser = argparse.ArgumentParser(); parser.add_argument("--rows", type=int, default=20000); parser.add_argument("--seed", type=int, default=52); parser.add_argument("--out", default="data/synthetic_credit.csv")
    args = parser.parse_args(); records = generate(args.rows, args.seed); path = Path(args.out); path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", newline="") as handle: writer = csv.DictWriter(handle, fieldnames=list(records[0]), lineterminator="\n"); writer.writeheader(); writer.writerows(records)
    print(f"wrote {len(records)} explicitly synthetic credit-risk rows to {path}")


if __name__ == "__main__": main()
