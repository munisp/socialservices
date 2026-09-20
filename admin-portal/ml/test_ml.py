import json
import tempfile
import unittest
from pathlib import Path

import torch

from .continuous_train import validate_source
from .experiments import ExperimentStore
from .generate_data import generate
from .models import CreditRiskMLP, FraudMLP, GraphSAGEFraud
from .monitor import drift_status, psi
from .registry import ModelRegistry

ROOT = Path(__file__).resolve().parents[1]


class MLPipelineTests(unittest.TestCase):
    def test_generator_is_deterministic_and_has_repeated_entities(self):
        first = generate(100, 7); self.assertEqual(first, generate(100, 7)); self.assertTrue(all(row["synthetic"] == 1 for row in first)); self.assertLess(len({row["beneficiary_id"] for row in first}), 100)

    def test_drift_thresholds_and_boundary(self):
        self.assertEqual(drift_status(0.01), "normal"); self.assertEqual(drift_status(0.12), "warning"); self.assertEqual(drift_status(0.30), "critical"); self.assertLess(psi([1, 2, 3, 4], [1, 2, 3, 4]), 0.01)

    def test_fraud_artifact_cpu_inference(self):
        artifact = ROOT / "artifacts/fraud"; manifest = json.loads((artifact / "manifest.json").read_text()); model = FraudMLP(len(manifest["features"])); model.load_state_dict(torch.load(artifact / manifest["weights"], map_location="cpu", weights_only=True)); preprocess = torch.load(artifact / manifest["preprocess"], map_location="cpu", weights_only=True); model.eval()
        with torch.inference_mode(): result = torch.sigmoid(model(torch.zeros((2, len(manifest["features"]))) - preprocess["mean"] / preprocess["std"]))
        self.assertEqual(tuple(result.shape), (2,)); self.assertTrue(torch.isfinite(result).all()); self.assertFalse(manifest["production_validated"])

    def test_credit_artifact_cpu_inference(self):
        artifact = ROOT / "artifacts/credit"; manifest = json.loads((artifact / "manifest.json").read_text()); model = CreditRiskMLP(len(manifest["features"])); model.load_state_dict(torch.load(artifact / manifest["weights"], map_location="cpu", weights_only=True)); model.eval()
        with torch.inference_mode(): result = torch.sigmoid(model(torch.zeros((1, len(manifest["features"])))))
        self.assertEqual(tuple(result.shape), (1,)); self.assertIn("prohibited_use", manifest)

    def test_gnn_artifact_cpu_inference(self):
        artifact = ROOT / "artifacts/gnn"; manifest = json.loads((artifact / "manifest.json").read_text()); snapshot = torch.load(artifact / manifest["graph_snapshot"], map_location="cpu", weights_only=True); model = GraphSAGEFraud(len(manifest["features"])); model.load_state_dict(torch.load(artifact / manifest["weights"], map_location="cpu", weights_only=True)); model.eval()
        with torch.inference_mode(): scores = torch.sigmoid(model((snapshot["features"] - snapshot["mean"]) / snapshot["std"], snapshot["edge_index"]))
        self.assertEqual(scores.shape[0], manifest["nodes"]); self.assertTrue(torch.isfinite(scores).all())

    def test_registry_resolves_only_promoted_versions_and_checks_immutability(self):
        with tempfile.TemporaryDirectory() as directory:
            registry = ModelRegistry(Path(directory) / "registry"); record = registry.register(ROOT / "artifacts/fraud", "fraud");
            with self.assertRaises(KeyError): registry.resolve("fraud")
            registry.promote("fraud", record["version"], "staging", "ci", "artifact integrity test"); self.assertTrue(registry.resolve("fraud", "staging").exists())

    def test_experiment_assignment_and_delayed_outcomes(self):
        with tempfile.TemporaryDirectory() as directory:
            store = ExperimentStore(Path(directory) / "experiments.sqlite"); store.create_experiment("fraud-v2", {"control": .5, "candidate": .5}); first = store.assign("fraud-v2", "beneficiary-1"); self.assertEqual(first, store.assign("fraud-v2", "beneficiary-1")); store.log_prediction("fraud-v2", "beneficiary-1", "tx-1", first, "v1", .9, "block", {"amount_ngn": 1}); store.record_outcome("tx-1", 1, "case-review"); self.assertEqual(store.metrics("fraud-v2")[0]["labelled_predictions"], 1)

    def test_production_source_requires_governance_metadata(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "data.csv"; source.write_text("label\n1\n")
            with self.assertRaises(ValueError): validate_source(source, "production", None)


if __name__ == "__main__": unittest.main()
