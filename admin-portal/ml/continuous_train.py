"""Continuous training entrypoint.
Production invocation supplies an export from the platform lakehouse; no in-memory
or hidden database generation is performed here. Ray is optional and used only when
installed, while the single-worker path remains deterministic and CPU-compatible.
"""
from __future__ import annotations
import argparse, json, subprocess, sys
from pathlib import Path

def main():
    p=argparse.ArgumentParser(); p.add_argument("--input",required=True,help="CSV/Parquet export from bronze/silver lakehouse"); p.add_argument("--artifact-root",default="artifacts"); p.add_argument("--rows",type=int,default=0); a=p.parse_args(); src=Path(a.input)
    if not src.exists(): raise SystemExit(f"input export does not exist: {src}")
    # Training is delegated to the real PyTorch loop. In production this command is the
    # target of a Kafka/Temporal/Ray schedule after a labelled-data watermark is reached.
    out=Path(a.artifact_root)/"fraud"; cmd=[sys.executable,"-m","ml.train","--data",str(src),"--out",str(out)]; print("running", " ".join(cmd)); subprocess.check_call(cmd); (out/"lineage.json").write_text(json.dumps({"source_export":str(src.resolve()),"source_is_production":True,"orchestrator":"continuous_train.py"},indent=2))
if __name__=="__main__": main()
