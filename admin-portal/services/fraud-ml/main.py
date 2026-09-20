"""CPU-capable PyTorch fraud inference with deterministic experiments and delayed outcomes."""
from __future__ import annotations
import json, logging, os, time
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional
import torch
from fastapi import FastAPI, Header, HTTPException
from pydantic import BaseModel, Field
try:
    from .experiment_store import ExperimentStore
    from .model import load_artifact
except ImportError:
    from experiment_store import ExperimentStore
    from model import load_artifact

logging.basicConfig(level=os.getenv("LOG_LEVEL","INFO")); log=logging.getLogger("fraud-ml")
DEFAULT_ROOT=os.getenv("MODEL_ARTIFACT_DIR","/app/artifacts/fraud"); STORE=ExperimentStore(os.getenv("PREDICTION_DB","/app/data/predictions.sqlite")); EXPERIMENT_ID=os.getenv("MODEL_EXPERIMENT_ID"); OUTCOME_KEY=os.getenv("OUTCOME_API_KEY"); started=time.time(); models={}

class Transaction(BaseModel):
    transaction_id:str; beneficiary_id:str; program_id:str; amount:float=Field(gt=0); transaction_type:str; timestamp:datetime=Field(default_factory=lambda:datetime.now(timezone.utc)); velocity_24h:int=0; device_reuse:bool=False; distance_km:float=0; beneficiary_age_days:int=365; mcc_risk:int=0; metadata:Optional[Dict[str,Any]]=None
class Score(BaseModel):
    transaction_id:str; fraud_score:float; risk_level:str; decision:str; confidence:float; model_version:str; variant:str; factors:List[Dict[str,Any]]; explanation:str; processing_time_ms:float
class Outcome(BaseModel): transaction_id:str; label:int=Field(ge=0,le=1); source:str=Field(min_length=3,max_length=200)

def load_models():
    global models
    configured=json.loads(os.getenv("MODEL_VARIANTS_JSON",json.dumps({"control":DEFAULT_ROOT})))
    loaded={}
    for variant,path in configured.items():
        try: loaded[variant]=load_artifact(path); log.info("loaded %s from %s",variant,path)
        except Exception as exc: log.error("failed loading %s: %s",variant,exc)
    models=loaded
    if EXPERIMENT_ID and len(models)>1: STORE.ensure_experiment(EXPERIMENT_ID,{name:1/len(models) for name in sorted(models)})

def raw_features(record): return {"amount_ngn":record.amount,"hour":record.timestamp.hour,"day_of_year":record.timestamp.timetuple().tm_yday,"velocity_24h":record.velocity_24h,"device_reuse":int(record.device_reuse),"distance_km":record.distance_km,"beneficiary_age_days":record.beneficiary_age_days,"mcc_risk":record.mcc_risk}
def vector(record,prep,manifest):
    features=raw_features(record); x=torch.tensor([[features[name] for name in manifest["features"]]],dtype=torch.float32); return (x-prep["mean"])/prep["std"]
def choose_variant(subject):
    if not models: raise HTTPException(503,"no validated model artifact loaded")
    return STORE.assign(EXPERIMENT_ID,subject) if EXPERIMENT_ID and len(models)>1 else sorted(models)[0]
def score(record):
    variant=choose_variant(record.beneficiary_id); model,prep,manifest=models[variant]; started_at=time.perf_counter()
    with torch.inference_mode(): probability=float(torch.sigmoid(model(vector(record,prep,manifest))).item())
    threshold=float(manifest.get("metrics",{}).get("threshold",.5)); risk="high" if probability>=max(.8,threshold) else "medium" if probability>=threshold else "low"; decision="block" if risk=="high" else "review" if risk=="medium" else "allow"; STORE.log_prediction(EXPERIMENT_ID,record.beneficiary_id,record.transaction_id,variant,manifest["version"],probability,decision,raw_features(record))
    return Score(transaction_id=record.transaction_id,fraud_score=round(probability,6),risk_level=risk,decision=decision,confidence=round(abs(probability-.5)*2,6),model_version=manifest["version"],variant=variant,factors=[],explanation="PyTorch probability; no hand-written scoring rules are used.",processing_time_ms=round((time.perf_counter()-started_at)*1000,3))

app=FastAPI(title="Fraud Detection ML Service",version="3.0.0")
@app.on_event("startup")
def startup(): load_models()
@app.get("/health")
def health(): return {"status":"healthy" if models else "unhealthy","models":{variant:value[2]["version"] for variant,value in models.items()},"experiment_id":EXPERIMENT_ID,"uptime_seconds":time.time()-started,"inference_device":"cpu"}
@app.post("/v1/score/transaction",response_model=Score)
def transaction(record:Transaction): return score(record)
@app.post("/v1/outcomes")
def outcome(value:Outcome,x_outcome_key:Optional[str]=Header(default=None)):
    if OUTCOME_KEY and x_outcome_key!=OUTCOME_KEY: raise HTTPException(403,"invalid outcome key")
    try: STORE.record_outcome(value.transaction_id,value.label,value.source)
    except KeyError as exc: raise HTTPException(404,str(exc)) from exc
    return {"accepted":True}
@app.get("/v1/experiments/{experiment_id}/metrics")
def experiment_metrics(experiment_id:str): return {"experiment_id":experiment_id,"variants":STORE.metrics(experiment_id)}
@app.get("/v1/models")
def model_info(): return {variant:value[2] for variant,value in models.items()}
