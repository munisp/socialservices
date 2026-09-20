from __future__ import annotations

import argparse
import fcntl
import hashlib
import json
import shutil
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

VALID_STAGES = {"candidate", "staging", "production", "archived", "rejected"}


def digest(path: Path) -> str:
    value = hashlib.sha256();
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""): value.update(chunk)
    return value.hexdigest()


class ModelRegistry:
    def __init__(self, root: str | Path):
        self.root = Path(root); self.root.mkdir(parents=True, exist_ok=True)
        self.index_path = self.root / "registry.json"; self.lock_path = self.root / ".registry.lock"

    def _read(self) -> dict[str, Any]:
        return json.loads(self.index_path.read_text()) if self.index_path.exists() else {"models": {}, "stage_pointers": {}}

    def _write(self, data: dict[str, Any]) -> None:
        temporary = self.index_path.with_suffix(".tmp"); temporary.write_text(json.dumps(data, indent=2) + "\n"); temporary.replace(self.index_path)

    def register(self, artifact_dir: str | Path, model_name: str, stage: str = "candidate") -> dict[str, Any]:
        if stage not in VALID_STAGES: raise ValueError(f"invalid stage {stage}")
        source = Path(artifact_dir); manifest = json.loads((source / "manifest.json").read_text()); version = manifest["version"]
        destination = self.root / model_name / version
        with self.lock_path.open("a+") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            if destination.exists():
                existing = json.loads((destination / "manifest.json").read_text())
                if existing.get("weights_sha256") != manifest.get("weights_sha256"): raise ValueError("immutable model version collision")
            else:
                shutil.copytree(source, destination)
            files = {path.name: digest(path) for path in destination.iterdir() if path.is_file()}
            index = self._read(); record = {"model": model_name, "version": version, "stage": stage, "path": str(Path(model_name) / version), "files": files,
                                                 "registered_at": datetime.now(timezone.utc).isoformat(), "approval": None}
            index["models"].setdefault(model_name, {})[version] = record; self._write(index); return record

    def promote(self, model_name: str, version: str, stage: str, approver: str, reason: str) -> dict[str, Any]:
        if stage not in VALID_STAGES - {"candidate"}: raise ValueError(f"invalid promotion stage {stage}")
        if not approver.strip() or not reason.strip(): raise ValueError("approver and reason are required")
        with self.lock_path.open("a+") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX); index = self._read(); record = index["models"].get(model_name, {}).get(version)
            if not record: raise KeyError(f"unknown model {model_name}:{version}")
            artifact = self.root / record["path"]
            for name, expected in record["files"].items():
                if digest(artifact / name) != expected: raise ValueError(f"artifact checksum mismatch: {name}")
            record["stage"] = stage; record["approval"] = {"approver": approver, "reason": reason, "at": datetime.now(timezone.utc).isoformat()}
            if stage in {"staging", "production"}: index["stage_pointers"].setdefault(model_name, {})[stage] = version
            self._write(index); return record

    def resolve(self, model_name: str, stage: str = "production") -> Path:
        index = self._read(); version = index["stage_pointers"].get(model_name, {}).get(stage)
        if not version: raise KeyError(f"no {stage} version for {model_name}")
        return self.root / index["models"][model_name][version]["path"]


def main() -> None:
    parser = argparse.ArgumentParser(); parser.add_argument("--root", default="model-registry"); sub = parser.add_subparsers(dest="command", required=True)
    register = sub.add_parser("register"); register.add_argument("--artifact", required=True); register.add_argument("--model", required=True); register.add_argument("--stage", default="candidate")
    promote = sub.add_parser("promote"); promote.add_argument("--model", required=True); promote.add_argument("--version", required=True); promote.add_argument("--stage", required=True); promote.add_argument("--approver", required=True); promote.add_argument("--reason", required=True)
    resolve = sub.add_parser("resolve"); resolve.add_argument("--model", required=True); resolve.add_argument("--stage", default="production")
    args = parser.parse_args(); registry = ModelRegistry(args.root)
    if args.command == "register": result = registry.register(args.artifact, args.model, args.stage)
    elif args.command == "promote": result = registry.promote(args.model, args.version, args.stage, args.approver, args.reason)
    else: result = {"path": str(registry.resolve(args.model, args.stage))}
    print(json.dumps(result, indent=2))


if __name__ == "__main__": main()
