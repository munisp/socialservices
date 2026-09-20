from __future__ import annotations
import torch
from torch import nn

class FraudMLP(nn.Module):
    """Tabular fraud classifier; no rules are used at inference time."""
    def __init__(self, in_features: int):
        super().__init__()
        self.net = nn.Sequential(nn.Linear(in_features, 64), nn.ReLU(), nn.Dropout(.15), nn.Linear(64, 32), nn.ReLU(), nn.Linear(32, 1))
    def forward(self, x): return self.net(x).squeeze(-1)

class CreditRiskMLP(nn.Module):
    """Tabular payment-default risk model; never used as a benefit eligibility rule."""
    def __init__(self, in_features: int):
        super().__init__()
        self.net = nn.Sequential(nn.Linear(in_features, 48), nn.GELU(), nn.Dropout(.1), nn.Linear(48, 24), nn.GELU(), nn.Linear(24, 1))
    def forward(self, x): return self.net(x).squeeze(-1)

class GraphSAGEFraud(nn.Module):
    """Small dependency-free GraphSAGE-style network for beneficiary/device graphs."""
    def __init__(self, in_features: int, hidden: int = 32):
        super().__init__(); self.self1=nn.Linear(in_features, hidden); self.neigh1=nn.Linear(in_features, hidden); self.self2=nn.Linear(hidden, hidden); self.neigh2=nn.Linear(hidden, hidden); self.out=nn.Linear(hidden, 1)
    def forward(self, x, edge_index):
        src, dst = edge_index
        agg = torch.zeros_like(x); agg.index_add_(0, dst, x[src]); deg = torch.zeros(x.size(0), device=x.device); deg.index_add_(0, dst, torch.ones(src.size(0), device=x.device)); agg = agg / deg.clamp_min(1).unsqueeze(1)
        h = torch.relu(self.self1(x) + self.neigh1(agg)); agg2 = torch.zeros_like(h); agg2.index_add_(0, dst, h[src]); agg2 = agg2 / deg.clamp_min(1).unsqueeze(1)
        return self.out(torch.relu(self.self2(h) + self.neigh2(agg2))).squeeze(-1)

def cpu_load(path: str, model: nn.Module):
    state = torch.load(path, map_location=torch.device("cpu"), weights_only=True); model.load_state_dict(state); model.eval(); return model
