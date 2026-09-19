from flask import Flask, request, jsonify
import ray
import numpy as np
from sklearn.ensemble import RandomForestClassifier
from sklearn.preprocessing import StandardScaler
import joblib
import os

app = Flask(__name__)

# Initialize Ray
if not ray.is_initialized():
    ray.init(ignore_reinit_error=True)

# Model storage
models = {}

@ray.remote
class FraudDetectionModel:
    def __init__(self):
        self.model = RandomForestClassifier(n_estimators=100, random_state=42)
        self.scaler = StandardScaler()
        self.is_trained = False
    
    def train(self, X, y):
        X_scaled = self.scaler.fit_transform(X)
        self.model.fit(X_scaled, y)
        self.is_trained = True
        return {"success": True, "samples": len(X)}
    
    def predict(self, X):
        if not self.is_trained:
            raise Exception("Model not trained")
        X_scaled = self.scaler.transform(X)
        predictions = self.model.predict(X_scaled)
        probabilities = self.model.predict_proba(X_scaled)
        return predictions.tolist(), probabilities.tolist()

@ray.remote
def process_batch(data_batch):
    """Process data batch in parallel"""
    results = []
    for item in data_batch:
        # Simulate processing
        result = {
            "id": item.get("id"),
            "processed": True,
            "score": np.random.random()
        }
        results.append(result)
    return results

@app.route('/health', methods=['GET'])
def health():
    return jsonify({
        "status": "healthy",
        "service": "ray",
        "ray_initialized": ray.is_initialized()
    }), 200

@app.route('/ml/train-fraud-model', methods=['POST'])
def train_fraud_model():
    """Train fraud detection model"""
    try:
        data = request.json
        training_data = data.get('training_data')
        model_id = data.get('model_id', 'fraud_default')
        
        if not training_data:
            return jsonify({"error": "training_data required"}), 400
        
        # Extract features and labels
        X = np.array([item['features'] for item in training_data])
        y = np.array([item['label'] for item in training_data])
        
        # Create or get model
        if model_id not in models:
            models[model_id] = FraudDetectionModel.remote()
        
        # Train model
        result = ray.get(models[model_id].train.remote(X, y))
        
        return jsonify({
            "success": True,
            "model_id": model_id,
            **result
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/ml/predict-fraud', methods=['POST'])
def predict_fraud():
    """Predict fraud using trained model"""
    try:
        data = request.json
        features = data.get('features')
        model_id = data.get('model_id', 'fraud_default')
        
        if not features:
            return jsonify({"error": "features required"}), 400
        
        if model_id not in models:
            return jsonify({"error": "Model not found"}), 404
        
        # Predict
        X = np.array([features])
        predictions, probabilities = ray.get(models[model_id].predict.remote(X))
        
        return jsonify({
            "success": True,
            "prediction": int(predictions[0]),
            "fraud_probability": float(probabilities[0][1]),
            "confidence": float(max(probabilities[0]))
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/parallel/process-batch', methods=['POST'])
def parallel_process():
    """Process data in parallel using Ray"""
    try:
        data = request.json
        items = data.get('items', [])
        batch_size = data.get('batch_size', 100)
        
        if not items:
            return jsonify({"error": "items required"}), 400
        
        # Split into batches
        batches = [items[i:i+batch_size] for i in range(0, len(items), batch_size)]
        
        # Process in parallel
        futures = [process_batch.remote(batch) for batch in batches]
        results = ray.get(futures)
        
        # Flatten results
        all_results = [item for batch_result in results for item in batch_result]
        
        return jsonify({
            "success": True,
            "processed_count": len(all_results),
            "batch_count": len(batches),
            "results": all_results
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5005)
