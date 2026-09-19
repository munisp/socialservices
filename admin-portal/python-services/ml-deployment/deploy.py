#!/usr/bin/env python3
"""
ML Model Deployment Automation
Automates deployment of trained ML models to production
"""
import os
import json
import shutil
import subprocess
from datetime import datetime
from pathlib import Path

class MLModelDeployer:
    def __init__(self, model_registry_path="/var/ml/models"):
        self.registry_path = Path(model_registry_path)
        self.registry_path.mkdir(parents=True, exist_ok=True)
        self.metadata_file = self.registry_path / "registry.json"
        self.load_registry()
    
    def load_registry(self):
        """Load model registry metadata"""
        if self.metadata_file.exists():
            with open(self.metadata_file, 'r') as f:
                self.registry = json.load(f)
        else:
            self.registry = {"models": {}}
    
    def save_registry(self):
        """Save model registry metadata"""
        with open(self.metadata_file, 'w') as f:
            json.dump(self.registry, f, indent=2)
    
    def register_model(self, model_name, model_path, version, metadata=None):
        """Register a new model version"""
        model_id = f"{model_name}_v{version}"
        model_dir = self.registry_path / model_id
        model_dir.mkdir(exist_ok=True)
        
        # Copy model files
        if os.path.isfile(model_path):
            shutil.copy2(model_path, model_dir / "model.pkl")
        else:
            shutil.copytree(model_path, model_dir, dirs_exist_ok=True)
        
        # Register in metadata
        if model_name not in self.registry["models"]:
            self.registry["models"][model_name] = {"versions": {}}
        
        self.registry["models"][model_name]["versions"][version] = {
            "registered_at": datetime.now().isoformat(),
            "path": str(model_dir),
            "status": "registered",
            "metadata": metadata or {}
        }
        
        self.save_registry()
        print(f"✓ Registered {model_id}")
        return model_id
    
    def deploy_model(self, model_name, version, service_name="ray-service"):
        """Deploy model to production service"""
        model_id = f"{model_name}_v{version}"
        
        if model_name not in self.registry["models"]:
            raise ValueError(f"Model {model_name} not found in registry")
        
        if version not in self.registry["models"][model_name]["versions"]:
            raise ValueError(f"Version {version} not found for {model_name}")
        
        model_info = self.registry["models"][model_name]["versions"][version]
        model_path = model_info["path"]
        
        # Deploy to service (copy to service directory)
        service_model_dir = Path(f"/var/services/{service_name}/models")
        service_model_dir.mkdir(parents=True, exist_ok=True)
        
        target_path = service_model_dir / model_id
        if target_path.exists():
            shutil.rmtree(target_path)
        
        shutil.copytree(model_path, target_path)
        
        # Update model status
        model_info["status"] = "deployed"
        model_info["deployed_at"] = datetime.now().isoformat()
        model_info["service"] = service_name
        self.save_registry()
        
        # Restart service to load new model
        self.restart_service(service_name)
        
        print(f"✓ Deployed {model_id} to {service_name}")
        return True
    
    def rollback_model(self, model_name, target_version):
        """Rollback to previous model version"""
        return self.deploy_model(model_name, target_version)
    
    def list_models(self):
        """List all registered models"""
        for model_name, model_data in self.registry["models"].items():
            print(f"\n{model_name}:")
            for version, info in model_data["versions"].items():
                status = info.get("status", "unknown")
                deployed_at = info.get("deployed_at", "never")
                print(f"  v{version}: {status} (deployed: {deployed_at})")
    
    def restart_service(self, service_name):
        """Restart service to load new model"""
        try:
            # Try Docker restart
            subprocess.run(
                ["docker", "restart", service_name],
                check=True,
                capture_output=True
            )
            print(f"✓ Restarted {service_name}")
        except subprocess.CalledProcessError:
            print(f"⚠ Could not restart {service_name} (not running or not Docker)")
    
    def validate_model(self, model_name, version, test_data_path):
        """Validate model performance before deployment"""
        import pickle
        import numpy as np
        from sklearn.metrics import accuracy_score, f1_score
        
        model_id = f"{model_name}_v{version}"
        model_path = self.registry_path / model_id / "model.pkl"
        
        # Load model
        with open(model_path, 'rb') as f:
            model = pickle.load(f)
        
        # Load test data
        test_data = np.load(test_data_path)
        X_test = test_data['X']
        y_test = test_data['y']
        
        # Predict
        y_pred = model.predict(X_test)
        
        # Calculate metrics
        accuracy = accuracy_score(y_test, y_pred)
        f1 = f1_score(y_test, y_pred, average='weighted')
        
        print(f"\nValidation Results for {model_id}:")
        print(f"  Accuracy: {accuracy:.4f}")
        print(f"  F1 Score: {f1:.4f}")
        
        # Update metadata
        model_info = self.registry["models"][model_name]["versions"][version]
        model_info["validation"] = {
            "accuracy": accuracy,
            "f1_score": f1,
            "validated_at": datetime.now().isoformat()
        }
        self.save_registry()
        
        return {"accuracy": accuracy, "f1_score": f1}

def main():
    import argparse
    
    parser = argparse.ArgumentParser(description="ML Model Deployment Automation")
    parser.add_argument("action", choices=["register", "deploy", "rollback", "list", "validate"])
    parser.add_argument("--model", help="Model name")
    parser.add_argument("--version", help="Model version")
    parser.add_argument("--path", help="Model file/directory path")
    parser.add_argument("--service", default="ray-service", help="Target service name")
    parser.add_argument("--test-data", help="Test data path for validation")
    
    args = parser.parse_args()
    
    deployer = MLModelDeployer()
    
    if args.action == "register":
        if not args.model or not args.version or not args.path:
            parser.error("register requires --model, --version, and --path")
        deployer.register_model(args.model, args.path, args.version)
    
    elif args.action == "deploy":
        if not args.model or not args.version:
            parser.error("deploy requires --model and --version")
        deployer.deploy_model(args.model, args.version, args.service)
    
    elif args.action == "rollback":
        if not args.model or not args.version:
            parser.error("rollback requires --model and --version")
        deployer.rollback_model(args.model, args.version)
    
    elif args.action == "list":
        deployer.list_models()
    
    elif args.action == "validate":
        if not args.model or not args.version or not args.test_data:
            parser.error("validate requires --model, --version, and --test-data")
        deployer.validate_model(args.model, args.version, args.test_data)

if __name__ == "__main__":
    main()
