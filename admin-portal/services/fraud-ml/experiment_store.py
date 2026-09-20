from __future__ import annotations
import hashlib, json, sqlite3
from datetime import datetime, timezone
from pathlib import Path

class ExperimentStore:
    def __init__(self, path): self.path=Path(path); self.path.parent.mkdir(parents=True,exist_ok=True); self._init()
    def connect(self): db=sqlite3.connect(self.path); db.row_factory=sqlite3.Row; return db
    def _init(self):
        with self.connect() as db: db.executescript('''CREATE TABLE IF NOT EXISTS experiments(id TEXT PRIMARY KEY,variants_json TEXT NOT NULL,active INTEGER NOT NULL,created_at TEXT NOT NULL); CREATE TABLE IF NOT EXISTS predictions(transaction_id TEXT PRIMARY KEY,experiment_id TEXT,subject_id TEXT NOT NULL,variant TEXT NOT NULL,model_version TEXT NOT NULL,score REAL NOT NULL,decision TEXT NOT NULL,features_json TEXT NOT NULL,created_at TEXT NOT NULL); CREATE TABLE IF NOT EXISTS outcomes(transaction_id TEXT PRIMARY KEY,label INTEGER NOT NULL CHECK(label IN(0,1)),source TEXT NOT NULL,observed_at TEXT NOT NULL);''')
    def ensure_experiment(self, experiment_id, variants):
        if len(variants)<2 or abs(sum(variants.values())-1)>1e-6 or any(v<=0 for v in variants.values()): raise ValueError('variant allocations must be positive and sum to one')
        with self.connect() as db: db.execute('INSERT OR IGNORE INTO experiments VALUES(?,?,1,?)',(experiment_id,json.dumps(variants,sort_keys=True),datetime.now(timezone.utc).isoformat()))
    def assign(self, experiment_id, subject_id):
        with self.connect() as db: row=db.execute('SELECT variants_json,active FROM experiments WHERE id=?',(experiment_id,)).fetchone()
        if not row or not row['active']: raise KeyError('experiment unavailable')
        position=int(hashlib.sha256(f'{experiment_id}:{subject_id}'.encode()).hexdigest()[:16],16)/float(0xFFFFFFFFFFFFFFFF); cumulative=0.0; variants=json.loads(row['variants_json'])
        # Keep assignment stable while preserving JSON's sorted insertion order.
        cumulative=0.0
        for name,weight in variants.items():
            cumulative+=weight
            if position<cumulative: return name
        return list(variants)[-1]
    def log_prediction(self, experiment_id,subject_id,transaction_id,variant,model_version,score,decision,features):
        with self.connect() as db: db.execute('INSERT OR IGNORE INTO predictions VALUES(?,?,?,?,?,?,?,?,?)',(transaction_id,experiment_id,subject_id,variant,model_version,score,decision,json.dumps(features,sort_keys=True),datetime.now(timezone.utc).isoformat()))
    def record_outcome(self,transaction_id,label,source):
        with self.connect() as db:
            if not db.execute('SELECT 1 FROM predictions WHERE transaction_id=?',(transaction_id,)).fetchone(): raise KeyError('prediction not found')
            db.execute('INSERT OR REPLACE INTO outcomes VALUES(?,?,?,?)',(transaction_id,label,source,datetime.now(timezone.utc).isoformat()))
    def metrics(self,experiment_id):
        with self.connect() as db: rows=db.execute('SELECT p.variant,p.score,o.label FROM predictions p JOIN outcomes o USING(transaction_id) WHERE p.experiment_id=?',(experiment_id,)).fetchall()
        groups={}
        for row in rows: groups.setdefault(row['variant'],[]).append(row)
        output=[]
        for variant,values in sorted(groups.items()):
            tp=sum(v['score']>=.5 and v['label']==1 for v in values); fp=sum(v['score']>=.5 and v['label']==0 for v in values); fn=sum(v['score']<.5 and v['label']==1 for v in values); precision=tp/max(1,tp+fp); recall=tp/max(1,tp+fn); output.append({'variant':variant,'labelled_predictions':len(values),'precision':precision,'recall':recall,'f1':2*precision*recall/max(1e-9,precision+recall)})
        return output
