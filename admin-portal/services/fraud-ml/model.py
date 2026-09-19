from __future__ import annotations
import torch
from torch import nn
class FraudMLP(nn.Module):
    def __init__(self, in_features=8):
        super().__init__(); self.net=nn.Sequential(nn.Linear(in_features,64),nn.ReLU(),nn.Dropout(.15),nn.Linear(64,32),nn.ReLU(),nn.Linear(32,1))
    def forward(self,x): return self.net(x).squeeze(-1)
def load_artifact(root):
    import json, os
    manifest_path=os.path.join(root,'manifest.json')
    with open(manifest_path) as f: manifest=json.load(f)
    m=FraudMLP(len(manifest['features'])); m.load_state_dict(torch.load(os.path.join(root,manifest['weights']),map_location='cpu',weights_only=True)); prep=torch.load(os.path.join(root,manifest['preprocess']),map_location='cpu',weights_only=True); m.eval(); return m,prep,manifest
