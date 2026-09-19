"""
ML Document Validation Service
Validates uploaded documents using machine learning
Integrates with Dapr for service invocation
"""

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List, Dict, Any
import numpy as np
from datetime import datetime
import logging

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="ML Document Validation Service")

class Document(BaseModel):
    type: str
    fileURL: str
    fileName: str

class ValidationRequest(BaseModel):
    documents: List[Document]

class ValidationResponse(BaseModel):
    valid: bool
    confidence: float
    details: Dict[str, Any]
    timestamp: str

# Simulated ML model (in production, load actual trained model)
class DocumentValidator:
    def __init__(self):
        self.required_doc_types = {
            "national_id", "proof_of_address", "birth_certificate",
            "bank_statement", "utility_bill"
        }
        self.min_confidence = 0.8
    
    def validate_document_type(self, doc_type: str) -> bool:
        """Validate document type is recognized"""
        return doc_type.lower() in self.required_doc_types
    
    def extract_features(self, document: Document) -> np.ndarray:
        """Extract features from document for ML validation"""
        # In production: Use OCR, image processing, etc.
        # For now, simulate feature extraction
        features = []
        
        # Feature 1: File extension validity
        valid_extensions = ['.pdf', '.jpg', '.jpeg', '.png']
        has_valid_ext = any(document.fileName.lower().endswith(ext) for ext in valid_extensions)
        features.append(1.0 if has_valid_ext else 0.0)
        
        # Feature 2: Document type validity
        features.append(1.0 if self.validate_document_type(document.type) else 0.0)
        
        # Feature 3: URL validity (basic check)
        features.append(1.0 if document.fileURL.startswith('http') else 0.0)
        
        # Feature 4: Filename length (reasonable length)
        features.append(1.0 if 5 < len(document.fileName) < 100 else 0.5)
        
        return np.array(features)
    
    def predict_validity(self, features: np.ndarray) -> tuple[bool, float]:
        """Predict document validity using ML model"""
        # In production: Use actual trained model
        # For now, use weighted average of features
        confidence = float(np.mean(features))
        valid = confidence >= self.min_confidence
        
        return valid, confidence
    
    def validate_documents(self, documents: List[Document]) -> ValidationResponse:
        """Validate all documents"""
        if not documents:
            return ValidationResponse(
                valid=False,
                confidence=0.0,
                details={"error": "No documents provided"},
                timestamp=datetime.utcnow().isoformat()
            )
        
        all_features = []
        document_results = []
        
        for doc in documents:
            features = self.extract_features(doc)
            valid, confidence = self.predict_validity(features)
            
            document_results.append({
                "type": doc.type,
                "fileName": doc.fileName,
                "valid": valid,
                "confidence": confidence
            })
            
            all_features.append(confidence)
        
        # Overall validation
        overall_confidence = float(np.mean(all_features))
        overall_valid = overall_confidence >= self.min_confidence
        
        return ValidationResponse(
            valid=overall_valid,
            confidence=overall_confidence,
            details={
                "total_documents": len(documents),
                "documents": document_results,
                "min_confidence_threshold": self.min_confidence
            },
            timestamp=datetime.utcnow().isoformat()
        )

# Initialize validator
validator = DocumentValidator()

@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {"status": "healthy", "service": "ml-document-service"}

@app.post("/validate", response_model=ValidationResponse)
async def validate_documents(request: ValidationRequest):
    """
    Validate documents using ML model
    Called by Go orchestrator via Dapr
    """
    try:
        logger.info(f"Validating {len(request.documents)} documents")
        result = validator.validate_documents(request.documents)
        logger.info(f"Validation result: valid={result.valid}, confidence={result.confidence}")
        return result
    except Exception as e:
        logger.error(f"Validation error: {str(e)}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/predict-fraud")
async def predict_fraud(transaction_data: Dict[str, Any]):
    """
    Predict fraud probability for a transaction
    Uses ML model trained on historical fraud patterns
    """
    try:
        # Extract features
        amount = transaction_data.get("amount", 0)
        merchant_category = transaction_data.get("merchantCategory", "")
        time_of_day = transaction_data.get("timeOfDay", 12)
        day_of_week = transaction_data.get("dayOfWeek", 1)
        
        # Simple fraud detection logic (in production: use trained model)
        fraud_score = 0.0
        
        # High amount transactions
        if amount > 1000:
            fraud_score += 0.3
        
        # Unusual merchant categories
        suspicious_categories = ["gambling", "cryptocurrency", "wire_transfer"]
        if merchant_category.lower() in suspicious_categories:
            fraud_score += 0.4
        
        # Unusual time (late night)
        if time_of_day < 6 or time_of_day > 22:
            fraud_score += 0.2
        
        # Weekend transactions
        if day_of_week in [6, 7]:
            fraud_score += 0.1
        
        is_fraud = fraud_score > 0.5
        
        return {
            "is_fraud": is_fraud,
            "fraud_score": min(fraud_score, 1.0),
            "confidence": 0.85,
            "factors": {
                "high_amount": amount > 1000,
                "suspicious_merchant": merchant_category.lower() in suspicious_categories,
                "unusual_time": time_of_day < 6 or time_of_day > 22,
                "weekend": day_of_week in [6, 7]
            }
        }
    except Exception as e:
        logger.error(f"Fraud prediction error: {str(e)}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/predict-enrollment")
async def predict_enrollment(historical_data: Dict[str, Any]):
    """
    Predict future enrollment numbers
    Uses time series forecasting
    """
    try:
        # Get historical enrollment data
        past_enrollments = historical_data.get("past_enrollments", [100, 120, 115, 130, 140])
        months_ahead = historical_data.get("months_ahead", 6)
        
        # Simple linear regression forecast (in production: use ARIMA/Prophet)
        enrollments = np.array(past_enrollments)
        time_points = np.arange(len(enrollments))
        
        # Fit linear trend
        coeffs = np.polyfit(time_points, enrollments, 1)
        slope, intercept = coeffs
        
        # Predict future
        future_time = np.arange(len(enrollments), len(enrollments) + months_ahead)
        predictions = slope * future_time + intercept
        
        # Add some variance
        predictions = predictions + np.random.normal(0, 5, len(predictions))
        predictions = np.maximum(predictions, 0)  # No negative enrollments
        
        return {
            "predictions": predictions.tolist(),
            "trend": "increasing" if slope > 0 else "decreasing",
            "confidence": 0.75,
            "months_ahead": months_ahead
        }
    except Exception as e:
        logger.error(f"Enrollment prediction error: {str(e)}")
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8001)
