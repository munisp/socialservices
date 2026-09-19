"""CPU-capable fraud inference service backed by a shipped PyTorch artifact."""
from __future__ import annotations
import os,time,logging
from datetime import datetime,timezone
from typing import Optional,Dict,Any,List
import torch
from fastapi import FastAPI,HTTPException
from pydantic import BaseModel,Field
try:
    from .model import load_artifact
except ImportError:
    from model import load_artifact
logging.basicConfig(level=os.getenv('LOG_LEVEL','INFO')); log=logging.getLogger('fraud-ml')
ROOT=os.getenv('MODEL_ARTIFACT_DIR','/app/artifacts/fraud'); model=None; prep=None; manifest=None; started=time.time()
class Transaction(BaseModel):
    transaction_id:str; beneficiary_id:str; program_id:str; amount:float=Field(gt=0); transaction_type:str; timestamp:datetime=Field(default_factory=lambda: datetime.now(timezone.utc)); velocity_24h:int=0; device_reuse:bool=False; distance_km:float=0; beneficiary_age_days:int=365; mcc_risk:int=0; metadata:Optional[Dict[str,Any]]=None
class Score(BaseModel):
    transaction_id:str; fraud_score:float; risk_level:str; decision:str; confidence:float; model_version:str; factors:List[Dict[str,Any]]; explanation:str; processing_time_ms:float
class Health(BaseModel):
    status:str; model_loaded:bool; model_version:Optional[str]; uptime_seconds:float; inference_device:str

def load():
    global model,prep,manifest
    try: model,prep,manifest=load_artifact(ROOT); log.info('loaded PyTorch artifact %s',manifest.get('version','unversioned'))
    except Exception as e: model=prep=manifest=None; log.error('model artifact unavailable: %s',e)

def vector(r):
    values=[r.amount,r.timestamp.hour,r.timestamp.timetuple().tm_yday,r.velocity_24h,int(r.device_reuse),r.distance_km,r.beneficiary_age_days,r.mcc_risk]
    x=torch.tensor([values],dtype=torch.float32); return (x-prep['mean'])/prep['std']

def score(r):
    if model is None: raise HTTPException(503,'model artifact unavailable')
    t=time.perf_counter()
    with torch.inference_mode(): p=float(torch.sigmoid(model(vector(r))).item())
    risk='high' if p>=.8 else 'medium' if p>=.5 else 'low'; decision='block' if risk=='high' else 'review' if risk=='medium' else 'allow'; ms=(time.perf_counter()-t)*1000
    return Score(transaction_id=r.transaction_id,fraud_score=round(p,6),risk_level=risk,decision=decision,confidence=round(abs(p-.5)*2,6),model_version=manifest.get('version','artifact'),factors=[],explanation='Neural model probability; review factors are not rule-generated.',processing_time_ms=round(ms,3))
app=FastAPI(title='Fraud Detection ML Service',version='2.0.0')
@app.on_event('startup')
def startup(): load()
@app.get('/health',response_model=Health)
def health(): return Health(status='healthy' if model else 'degraded',model_loaded=model is not None,model_version=manifest.get('version') if manifest else None,uptime_seconds=time.time()-started,inference_device='cpu')
@app.post('/v1/score/transaction',response_model=Score)
def transaction(r:Transaction): return score(r)
@app.get('/v1/model')
def model_info():
    if not manifest: raise HTTPException(503,'model artifact unavailable')
    return manifest
