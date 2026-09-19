from __future__ import annotations
import argparse, csv, json, random
from pathlib import Path
import torch
from torch import nn
from .models import FraudMLP

FEATURES = ["amount_ngn","hour","day_of_year","velocity_24h","device_reuse","distance_km","beneficiary_age_days","mcc_risk"]

def load_csv(path):
    with open(path, newline="") as f: rows=list(csv.DictReader(f))
    # Encode numeric fields only; categorical fields remain available for downstream lakehouse use.
    x=torch.tensor([[float(r[k]) for k in FEATURES] for r in rows], dtype=torch.float32); y=torch.tensor([float(r["label"]) for r in rows], dtype=torch.float32); return x,y

def metrics(y, p):
    pred=(p>=.5).int(); yy=y.int(); tp=((pred==1)&(yy==1)).sum().item(); tn=((pred==0)&(yy==0)).sum().item(); fp=((pred==1)&(yy==0)).sum().item(); fn=((pred==0)&(yy==1)).sum().item(); precision=tp/max(1,tp+fp); recall=tp/max(1,tp+fn); acc=(tp+tn)/max(1,len(y)); f1=2*precision*recall/max(1e-9,precision+recall); return {"accuracy":acc,"precision":precision,"recall":recall,"f1":f1,"support":len(y)}

def main():
    ap=argparse.ArgumentParser(); ap.add_argument("--data",default="data/synthetic_transactions.csv"); ap.add_argument("--out",default="artifacts/fraud"); ap.add_argument("--epochs",type=int,default=20); ap.add_argument("--seed",type=int,default=42); a=ap.parse_args(); random.seed(a.seed); torch.manual_seed(a.seed)
    x,y=load_csv(a.data); perm=torch.randperm(len(y)); cut=int(len(y)*.8); tr,va=perm[:cut],perm[cut:]; mean=x[tr].mean(0); std=x[tr].std(0).clamp_min(1e-6); x=(x-mean)/std
    m=FraudMLP(len(FEATURES)); pos=max(1., y[tr].sum().item()); neg=max(1.,len(tr)-pos); loss_fn=nn.BCEWithLogitsLoss(pos_weight=torch.tensor([neg/pos])); opt=torch.optim.AdamW(m.parameters(),lr=2e-3,weight_decay=1e-4)
    for epoch in range(1,a.epochs+1):
        m.train(); opt.zero_grad(); loss=loss_fn(m(x[tr]),y[tr]); loss.backward(); opt.step()
        if epoch==1 or epoch%5==0: print(f"epoch={epoch} loss={loss.item():.4f}")
    m.eval(); p=torch.sigmoid(m(x[va])).detach(); met=metrics(y[va],p); out=Path(a.out); out.mkdir(parents=True,exist_ok=True); torch.save(m.state_dict(),out/"fraud_mlp.pt"); torch.save({"mean":mean,"std":std},out/"preprocess.pt"); manifest={"version":f"fraud-ml-{a.seed}-{len(y)}","model_type":"pytorch_mlp","features":FEATURES,"weights":"fraud_mlp.pt","preprocess":"preprocess.pt","metrics":met,"trained_on":"synthetic Nigerian-like data only","seed":a.seed,"framework":"torch"}; (out/"manifest.json").write_text(json.dumps(manifest,indent=2)); print(json.dumps(met,indent=2))
if __name__=="__main__": main()
