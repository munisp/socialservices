from __future__ import annotations
import json, math
from pathlib import Path

def psi(reference, current, bins=10):
    """Population Stability Index; caller must provide comparable numeric samples."""
    if not reference or not current: return None
    lo,hi=min(reference+current),max(reference+current); width=max((hi-lo)/bins,1e-9); score=0.0
    for i in range(bins):
        upper = lo+(i+1)*width
        a=sum(lo+i*width<=v<upper or (i==bins-1 and v==hi) for v in reference)/len(reference); b=sum(lo+i*width<=v<upper or (i==bins-1 and v==hi) for v in current)/len(current); a=max(a,1e-6); b=max(b,1e-6); score+=(b-a)*math.log(b/a)
    return score

def drift_status(score, warning=.1, critical=.25):
    return "critical" if score is not None and score>=critical else "warning" if score is not None and score>=warning else "normal"

def write_report(path, model_version, feature_scores):
    p=Path(path); p.parent.mkdir(parents=True,exist_ok=True); payload={"model_version":model_version,"features":feature_scores,"status":"critical" if any(v.get("status")=="critical" for v in feature_scores.values()) else "warning" if any(v.get("status")=="warning" for v in feature_scores.values()) else "normal"}; p.write_text(json.dumps(payload,indent=2)); return payload
