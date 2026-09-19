import json
from pathlib import Path
from .generate_data import generate
from .monitor import drift_status, psi

def test_generator_is_deterministic():
    assert generate(5, 7) == generate(5, 7)
    assert all(row['synthetic'] == 1 for row in generate(20, 7))

def test_drift_thresholds():
    assert drift_status(0.01) == 'normal'; assert drift_status(0.12) == 'warning'; assert drift_status(0.30) == 'critical'
    assert psi([1,2,3,4], [1,2,3,4]) < 0.01

def test_manifest_is_real_artifact():
    manifest=Path('artifacts/fraud/manifest.json')
    if manifest.exists():
        data=json.loads(manifest.read_text()); assert data['framework']=='torch'; assert Path('artifacts/fraud/fraud_mlp.pt').exists()
