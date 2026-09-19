"""
Proxy Means Testing (PMT) Service

This service implements proxy means testing algorithms used by social protection
programs worldwide to estimate household welfare/income levels based on observable
characteristics rather than direct income measurement.

Based on methodologies from:
- Brazil's Cadastro Único
- Colombia's SISBEN
- Indonesia's Unified Database (BDT)
- Philippines' Listahanan
- World Bank PMT guidelines
"""

import os
import json
import logging
from datetime import datetime
from typing import Dict, List, Optional, Any, Tuple
from enum import Enum
from dataclasses import dataclass, field, asdict
import hashlib

import numpy as np
import pandas as pd
from fastapi import FastAPI, HTTPException, BackgroundTasks, Depends
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
from sklearn.ensemble import RandomForestRegressor, GradientBoostingRegressor
from sklearn.linear_model import Ridge, Lasso, ElasticNet
from sklearn.preprocessing import StandardScaler, OneHotEncoder
from sklearn.compose import ColumnTransformer
from sklearn.pipeline import Pipeline
from sklearn.model_selection import cross_val_score
import joblib
import redis
from sqlalchemy import create_engine, Column, Integer, String, Float, DateTime, JSON, Text, Boolean
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker, Session
from prometheus_client import Counter, Histogram, generate_latest, CONTENT_TYPE_LATEST
from starlette.responses import Response

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# FastAPI app
app = FastAPI(
    title="Proxy Means Testing Service",
    description="ML-based welfare estimation for social protection targeting",
    version="1.0.0"
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Prometheus metrics
PREDICTION_COUNTER = Counter('pmt_predictions_total', 'Total PMT predictions', ['model_type', 'status'])
PREDICTION_LATENCY = Histogram('pmt_prediction_latency_seconds', 'PMT prediction latency')
MODEL_TRAINING_COUNTER = Counter('pmt_model_training_total', 'Total model training runs', ['status'])

# Database setup
DATABASE_URL = os.getenv("DATABASE_URL", "mysql+pymysql://root:password@localhost:3306/social_protection")
REDIS_URL = os.getenv("REDIS_URL", "redis://localhost:6379/0")

engine = create_engine(DATABASE_URL, pool_pre_ping=True)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()

# Redis for caching
try:
    redis_client = redis.from_url(REDIS_URL)
except:
    redis_client = None
    logger.warning("Redis not available, caching disabled")


class PMTModelType(str, Enum):
    LINEAR = "linear"
    RIDGE = "ridge"
    LASSO = "lasso"
    ELASTIC_NET = "elastic_net"
    RANDOM_FOREST = "random_forest"
    GRADIENT_BOOSTING = "gradient_boosting"


class EligibilityCategory(str, Enum):
    EXTREMELY_POOR = "extremely_poor"
    POOR = "poor"
    VULNERABLE = "vulnerable"
    NON_POOR = "non_poor"


# Database models
class PMTModel(Base):
    __tablename__ = "pmt_models"
    
    id = Column(Integer, primary_key=True, index=True)
    name = Column(String(255), nullable=False)
    model_type = Column(String(64), nullable=False)
    version = Column(String(32), nullable=False)
    features = Column(JSON, nullable=False)
    coefficients = Column(JSON)
    feature_importance = Column(JSON)
    metrics = Column(JSON)
    thresholds = Column(JSON)  # Poverty line thresholds
    model_path = Column(String(512))
    is_active = Column(Boolean, default=False)
    trained_at = Column(DateTime)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)


class PMTScore(Base):
    __tablename__ = "pmt_scores"
    
    id = Column(Integer, primary_key=True, index=True)
    household_id = Column(String(64), nullable=False, index=True)
    beneficiary_id = Column(String(64), index=True)
    model_id = Column(Integer, nullable=False)
    model_version = Column(String(32), nullable=False)
    raw_score = Column(Float, nullable=False)
    normalized_score = Column(Float, nullable=False)
    percentile = Column(Float)
    category = Column(String(32))
    confidence = Column(Float)
    feature_contributions = Column(JSON)
    input_data_hash = Column(String(64))
    calculated_at = Column(DateTime, default=datetime.utcnow)
    expires_at = Column(DateTime)


Base.metadata.create_all(bind=engine)


# Pydantic models
class HouseholdCharacteristics(BaseModel):
    """Observable household characteristics for PMT calculation"""
    household_id: str
    beneficiary_id: Optional[str] = None
    
    # Demographic characteristics
    household_size: int = Field(..., ge=1, le=50)
    dependency_ratio: float = Field(..., ge=0, le=1)
    head_age: int = Field(..., ge=18, le=120)
    head_gender: str = Field(..., pattern="^(male|female)$")
    head_education_years: int = Field(..., ge=0, le=25)
    head_employment_status: str
    num_children_under_5: int = Field(default=0, ge=0)
    num_children_5_to_17: int = Field(default=0, ge=0)
    num_elderly_over_65: int = Field(default=0, ge=0)
    num_disabled_members: int = Field(default=0, ge=0)
    
    # Housing characteristics
    housing_type: str  # permanent, semi-permanent, temporary
    wall_material: str  # brick, concrete, wood, mud, other
    roof_material: str  # concrete, tile, metal, thatch, other
    floor_material: str  # tile, cement, earth, other
    num_rooms: int = Field(..., ge=1, le=20)
    has_electricity: bool
    has_piped_water: bool
    has_flush_toilet: bool
    cooking_fuel: str  # gas, electricity, wood, charcoal, other
    
    # Asset ownership
    owns_land: bool
    land_area_hectares: float = Field(default=0, ge=0)
    owns_livestock: bool
    livestock_value_usd: float = Field(default=0, ge=0)
    owns_vehicle: bool
    vehicle_type: Optional[str] = None  # car, motorcycle, bicycle, none
    owns_refrigerator: bool
    owns_television: bool
    owns_mobile_phone: bool
    owns_computer: bool
    
    # Location
    region: str
    district: str
    urban_rural: str = Field(..., pattern="^(urban|rural|peri-urban)$")
    
    # Additional indicators
    has_bank_account: bool = False
    receives_remittances: bool = False
    has_health_insurance: bool = False
    children_in_school: bool = True
    food_security_score: Optional[int] = Field(default=None, ge=0, le=10)


class PMTScoreResponse(BaseModel):
    household_id: str
    beneficiary_id: Optional[str]
    raw_score: float
    normalized_score: float
    percentile: float
    category: EligibilityCategory
    confidence: float
    feature_contributions: Dict[str, float]
    model_version: str
    calculated_at: datetime
    explanation: str


class ModelTrainingRequest(BaseModel):
    name: str
    model_type: PMTModelType
    training_data_path: Optional[str] = None
    features: List[str]
    target_variable: str = "consumption_per_capita"
    thresholds: Dict[str, float] = Field(
        default={
            "extremely_poor": 1.90,  # $1.90/day PPP
            "poor": 3.20,            # $3.20/day PPP
            "vulnerable": 5.50       # $5.50/day PPP
        }
    )
    hyperparameters: Optional[Dict[str, Any]] = None


class ModelTrainingResponse(BaseModel):
    model_id: int
    name: str
    model_type: str
    version: str
    metrics: Dict[str, float]
    feature_importance: Dict[str, float]
    status: str


# PMT Calculator
class PMTCalculator:
    """Proxy Means Testing Calculator"""
    
    # Feature weights based on World Bank PMT methodology
    DEFAULT_WEIGHTS = {
        # Housing quality (30% weight)
        "housing_score": 0.30,
        # Asset ownership (25% weight)
        "asset_score": 0.25,
        # Demographics (20% weight)
        "demographic_score": 0.20,
        # Location (15% weight)
        "location_score": 0.15,
        # Human capital (10% weight)
        "human_capital_score": 0.10,
    }
    
    HOUSING_MATERIALS_SCORE = {
        "wall": {"brick": 4, "concrete": 4, "wood": 2, "mud": 1, "other": 1},
        "roof": {"concrete": 4, "tile": 3, "metal": 2, "thatch": 1, "other": 1},
        "floor": {"tile": 4, "cement": 3, "earth": 1, "other": 1},
    }
    
    HOUSING_TYPE_SCORE = {
        "permanent": 4,
        "semi-permanent": 2,
        "temporary": 1,
    }
    
    def __init__(self, model: Optional[Pipeline] = None, thresholds: Optional[Dict[str, float]] = None):
        self.model = model
        self.thresholds = thresholds or {
            "extremely_poor": 1.90,
            "poor": 3.20,
            "vulnerable": 5.50,
        }
        self.scaler = StandardScaler()
    
    def calculate_housing_score(self, characteristics: HouseholdCharacteristics) -> float:
        """Calculate housing quality score (0-100)"""
        score = 0
        max_score = 0
        
        # Housing type
        score += self.HOUSING_TYPE_SCORE.get(characteristics.housing_type, 1) * 5
        max_score += 20
        
        # Wall material
        score += self.HOUSING_MATERIALS_SCORE["wall"].get(characteristics.wall_material, 1) * 3
        max_score += 12
        
        # Roof material
        score += self.HOUSING_MATERIALS_SCORE["roof"].get(characteristics.roof_material, 1) * 3
        max_score += 12
        
        # Floor material
        score += self.HOUSING_MATERIALS_SCORE["floor"].get(characteristics.floor_material, 1) * 3
        max_score += 12
        
        # Utilities
        if characteristics.has_electricity:
            score += 10
        max_score += 10
        
        if characteristics.has_piped_water:
            score += 10
        max_score += 10
        
        if characteristics.has_flush_toilet:
            score += 10
        max_score += 10
        
        # Cooking fuel
        fuel_scores = {"gas": 8, "electricity": 10, "wood": 2, "charcoal": 3, "other": 1}
        score += fuel_scores.get(characteristics.cooking_fuel, 1)
        max_score += 10
        
        # Rooms per person
        rooms_per_person = characteristics.num_rooms / characteristics.household_size
        if rooms_per_person >= 1:
            score += 10
        elif rooms_per_person >= 0.5:
            score += 5
        max_score += 10
        
        return (score / max_score) * 100
    
    def calculate_asset_score(self, characteristics: HouseholdCharacteristics) -> float:
        """Calculate asset ownership score (0-100)"""
        score = 0
        max_score = 0
        
        # Land ownership
        if characteristics.owns_land:
            score += 15
            if characteristics.land_area_hectares > 2:
                score += 10
            elif characteristics.land_area_hectares > 0.5:
                score += 5
        max_score += 25
        
        # Livestock
        if characteristics.owns_livestock:
            score += 10
            if characteristics.livestock_value_usd > 1000:
                score += 10
            elif characteristics.livestock_value_usd > 500:
                score += 5
        max_score += 20
        
        # Vehicle
        if characteristics.owns_vehicle:
            vehicle_scores = {"car": 20, "motorcycle": 10, "bicycle": 5, "none": 0}
            score += vehicle_scores.get(characteristics.vehicle_type, 0)
        max_score += 20
        
        # Household appliances
        if characteristics.owns_refrigerator:
            score += 10
        max_score += 10
        
        if characteristics.owns_television:
            score += 5
        max_score += 5
        
        if characteristics.owns_mobile_phone:
            score += 5
        max_score += 5
        
        if characteristics.owns_computer:
            score += 10
        max_score += 10
        
        # Financial assets
        if characteristics.has_bank_account:
            score += 5
        max_score += 5
        
        return (score / max_score) * 100 if max_score > 0 else 0
    
    def calculate_demographic_score(self, characteristics: HouseholdCharacteristics) -> float:
        """Calculate demographic vulnerability score (0-100, lower = more vulnerable)"""
        score = 50  # Start at middle
        
        # Dependency ratio (higher = more vulnerable)
        score -= characteristics.dependency_ratio * 30
        
        # Children under 5 (more = more vulnerable)
        score -= min(characteristics.num_children_under_5 * 5, 15)
        
        # Elderly members (more = more vulnerable)
        score -= min(characteristics.num_elderly_over_65 * 5, 15)
        
        # Disabled members
        score -= min(characteristics.num_disabled_members * 10, 20)
        
        # Household size (larger = economies of scale, but also more mouths)
        if characteristics.household_size > 6:
            score -= 10
        elif characteristics.household_size > 4:
            score -= 5
        
        # Female-headed household (often more vulnerable)
        if characteristics.head_gender == "female":
            score -= 5
        
        # Head age (very young or very old = more vulnerable)
        if characteristics.head_age < 25 or characteristics.head_age > 65:
            score -= 10
        
        return max(0, min(100, score + 50))  # Normalize to 0-100
    
    def calculate_human_capital_score(self, characteristics: HouseholdCharacteristics) -> float:
        """Calculate human capital score (0-100)"""
        score = 0
        max_score = 0
        
        # Head education
        if characteristics.head_education_years >= 16:
            score += 40
        elif characteristics.head_education_years >= 12:
            score += 30
        elif characteristics.head_education_years >= 9:
            score += 20
        elif characteristics.head_education_years >= 6:
            score += 10
        max_score += 40
        
        # Employment status
        employment_scores = {
            "employed_formal": 30,
            "employed_informal": 15,
            "self_employed": 20,
            "unemployed": 0,
            "retired": 10,
            "student": 5,
            "homemaker": 5,
        }
        score += employment_scores.get(characteristics.head_employment_status, 5)
        max_score += 30
        
        # Children in school
        if characteristics.children_in_school:
            score += 15
        max_score += 15
        
        # Health insurance
        if characteristics.has_health_insurance:
            score += 15
        max_score += 15
        
        return (score / max_score) * 100 if max_score > 0 else 0
    
    def calculate_location_score(self, characteristics: HouseholdCharacteristics) -> float:
        """Calculate location-based score (0-100)"""
        score = 50  # Start at middle
        
        # Urban/rural
        if characteristics.urban_rural == "urban":
            score += 20
        elif characteristics.urban_rural == "peri-urban":
            score += 10
        else:  # rural
            score -= 10
        
        # Food security
        if characteristics.food_security_score is not None:
            score += (characteristics.food_security_score - 5) * 5
        
        # Remittances (indicates some external support)
        if characteristics.receives_remittances:
            score += 10
        
        return max(0, min(100, score))
    
    def calculate_pmt_score(self, characteristics: HouseholdCharacteristics) -> Tuple[float, Dict[str, float]]:
        """Calculate overall PMT score"""
        # Calculate component scores
        housing_score = self.calculate_housing_score(characteristics)
        asset_score = self.calculate_asset_score(characteristics)
        demographic_score = self.calculate_demographic_score(characteristics)
        human_capital_score = self.calculate_human_capital_score(characteristics)
        location_score = self.calculate_location_score(characteristics)
        
        # Weighted average
        total_score = (
            housing_score * self.DEFAULT_WEIGHTS["housing_score"] +
            asset_score * self.DEFAULT_WEIGHTS["asset_score"] +
            demographic_score * self.DEFAULT_WEIGHTS["demographic_score"] +
            human_capital_score * self.DEFAULT_WEIGHTS["human_capital_score"] +
            location_score * self.DEFAULT_WEIGHTS["location_score"]
        )
        
        contributions = {
            "housing": housing_score * self.DEFAULT_WEIGHTS["housing_score"],
            "assets": asset_score * self.DEFAULT_WEIGHTS["asset_score"],
            "demographics": demographic_score * self.DEFAULT_WEIGHTS["demographic_score"],
            "human_capital": human_capital_score * self.DEFAULT_WEIGHTS["human_capital_score"],
            "location": location_score * self.DEFAULT_WEIGHTS["location_score"],
        }
        
        return total_score, contributions
    
    def score_to_consumption(self, pmt_score: float) -> float:
        """Convert PMT score to estimated daily consumption per capita (USD PPP)"""
        # Calibrated conversion based on typical PMT-consumption relationships
        # Score 0-20: $0-2/day, 20-40: $2-4/day, 40-60: $4-8/day, 60-80: $8-15/day, 80-100: $15+/day
        if pmt_score <= 20:
            return pmt_score * 0.1
        elif pmt_score <= 40:
            return 2 + (pmt_score - 20) * 0.1
        elif pmt_score <= 60:
            return 4 + (pmt_score - 40) * 0.2
        elif pmt_score <= 80:
            return 8 + (pmt_score - 60) * 0.35
        else:
            return 15 + (pmt_score - 80) * 0.5
    
    def categorize(self, consumption: float) -> EligibilityCategory:
        """Categorize household based on estimated consumption"""
        if consumption <= self.thresholds["extremely_poor"]:
            return EligibilityCategory.EXTREMELY_POOR
        elif consumption <= self.thresholds["poor"]:
            return EligibilityCategory.POOR
        elif consumption <= self.thresholds["vulnerable"]:
            return EligibilityCategory.VULNERABLE
        else:
            return EligibilityCategory.NON_POOR
    
    def calculate_confidence(self, characteristics: HouseholdCharacteristics) -> float:
        """Calculate confidence score based on data completeness and consistency"""
        confidence = 1.0
        
        # Penalize missing optional fields
        if characteristics.food_security_score is None:
            confidence -= 0.05
        if not characteristics.owns_vehicle and characteristics.vehicle_type:
            confidence -= 0.1  # Inconsistent data
        
        # Check for outliers
        if characteristics.household_size > 15:
            confidence -= 0.1
        if characteristics.land_area_hectares > 100:
            confidence -= 0.1
        if characteristics.livestock_value_usd > 50000:
            confidence -= 0.1
        
        return max(0.5, confidence)
    
    def generate_explanation(self, category: EligibilityCategory, contributions: Dict[str, float]) -> str:
        """Generate human-readable explanation of the score"""
        sorted_contributions = sorted(contributions.items(), key=lambda x: x[1])
        lowest = sorted_contributions[0]
        highest = sorted_contributions[-1]
        
        explanations = {
            EligibilityCategory.EXTREMELY_POOR: f"Household classified as extremely poor. Main vulnerability factors: {lowest[0]} ({lowest[1]:.1f}). Consider for immediate assistance programs.",
            EligibilityCategory.POOR: f"Household classified as poor. Strongest factor: {highest[0]} ({highest[1]:.1f}). Main vulnerability: {lowest[0]} ({lowest[1]:.1f}). Eligible for poverty reduction programs.",
            EligibilityCategory.VULNERABLE: f"Household classified as vulnerable. Above poverty line but at risk. Main strength: {highest[0]} ({highest[1]:.1f}). Consider for preventive programs.",
            EligibilityCategory.NON_POOR: f"Household classified as non-poor. Strongest factors: {highest[0]} ({highest[1]:.1f}). May not require direct assistance.",
        }
        
        return explanations.get(category, "Classification complete.")


# ML Model Trainer
class PMTModelTrainer:
    """Train ML models for PMT prediction"""
    
    def __init__(self, db: Session):
        self.db = db
    
    def prepare_features(self, df: pd.DataFrame, feature_columns: List[str]) -> Tuple[np.ndarray, ColumnTransformer]:
        """Prepare features for training"""
        # Identify categorical and numerical columns
        categorical_cols = df[feature_columns].select_dtypes(include=['object', 'bool']).columns.tolist()
        numerical_cols = df[feature_columns].select_dtypes(include=['int64', 'float64']).columns.tolist()
        
        # Create preprocessor
        preprocessor = ColumnTransformer(
            transformers=[
                ('num', StandardScaler(), numerical_cols),
                ('cat', OneHotEncoder(handle_unknown='ignore'), categorical_cols)
            ]
        )
        
        X = preprocessor.fit_transform(df[feature_columns])
        return X, preprocessor
    
    def train_model(self, request: ModelTrainingRequest, training_data: pd.DataFrame) -> ModelTrainingResponse:
        """Train a PMT model"""
        logger.info(f"Training {request.model_type} model: {request.name}")
        
        # Prepare features
        X, preprocessor = self.prepare_features(training_data, request.features)
        y = training_data[request.target_variable].values
        
        # Select model type
        hyperparams = request.hyperparameters or {}
        if request.model_type == PMTModelType.LINEAR:
            from sklearn.linear_model import LinearRegression
            model = LinearRegression(**hyperparams)
        elif request.model_type == PMTModelType.RIDGE:
            model = Ridge(**hyperparams)
        elif request.model_type == PMTModelType.LASSO:
            model = Lasso(**hyperparams)
        elif request.model_type == PMTModelType.ELASTIC_NET:
            model = ElasticNet(**hyperparams)
        elif request.model_type == PMTModelType.RANDOM_FOREST:
            model = RandomForestRegressor(n_estimators=100, random_state=42, **hyperparams)
        elif request.model_type == PMTModelType.GRADIENT_BOOSTING:
            model = GradientBoostingRegressor(n_estimators=100, random_state=42, **hyperparams)
        else:
            raise ValueError(f"Unknown model type: {request.model_type}")
        
        # Create pipeline
        pipeline = Pipeline([
            ('preprocessor', preprocessor),
            ('model', model)
        ])
        
        # Cross-validation
        cv_scores = cross_val_score(pipeline, training_data[request.features], y, cv=5, scoring='r2')
        
        # Fit final model
        pipeline.fit(training_data[request.features], y)
        
        # Calculate metrics
        y_pred = pipeline.predict(training_data[request.features])
        mse = np.mean((y - y_pred) ** 2)
        rmse = np.sqrt(mse)
        mae = np.mean(np.abs(y - y_pred))
        r2 = 1 - (np.sum((y - y_pred) ** 2) / np.sum((y - np.mean(y)) ** 2))
        
        metrics = {
            "r2": float(r2),
            "rmse": float(rmse),
            "mae": float(mae),
            "cv_r2_mean": float(np.mean(cv_scores)),
            "cv_r2_std": float(np.std(cv_scores)),
        }
        
        # Feature importance
        feature_importance = {}
        if hasattr(model, 'feature_importances_'):
            # For tree-based models
            feature_names = preprocessor.get_feature_names_out()
            for name, importance in zip(feature_names, model.feature_importances_):
                feature_importance[name] = float(importance)
        elif hasattr(model, 'coef_'):
            # For linear models
            feature_names = preprocessor.get_feature_names_out()
            for name, coef in zip(feature_names, model.coef_):
                feature_importance[name] = float(abs(coef))
        
        # Save model
        version = datetime.utcnow().strftime("%Y%m%d%H%M%S")
        model_path = f"/tmp/pmt_models/{request.name}_{version}.joblib"
        os.makedirs(os.path.dirname(model_path), exist_ok=True)
        joblib.dump(pipeline, model_path)
        
        # Store in database
        db_model = PMTModel(
            name=request.name,
            model_type=request.model_type.value,
            version=version,
            features=request.features,
            feature_importance=feature_importance,
            metrics=metrics,
            thresholds=request.thresholds,
            model_path=model_path,
            is_active=False,
            trained_at=datetime.utcnow(),
        )
        self.db.add(db_model)
        self.db.commit()
        self.db.refresh(db_model)
        
        MODEL_TRAINING_COUNTER.labels(status='success').inc()
        
        return ModelTrainingResponse(
            model_id=db_model.id,
            name=db_model.name,
            model_type=db_model.model_type,
            version=db_model.version,
            metrics=metrics,
            feature_importance=feature_importance,
            status="trained"
        )


# Dependency injection
def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()


def get_calculator():
    return PMTCalculator()


# API Endpoints
@app.get("/health")
async def health_check():
    return {"status": "healthy", "service": "pmt-service", "timestamp": datetime.utcnow().isoformat()}


@app.get("/metrics")
async def metrics():
    return Response(generate_latest(), media_type=CONTENT_TYPE_LATEST)


@app.post("/calculate", response_model=PMTScoreResponse)
async def calculate_pmt_score(
    characteristics: HouseholdCharacteristics,
    db: Session = Depends(get_db),
    calculator: PMTCalculator = Depends(get_calculator)
):
    """Calculate PMT score for a household"""
    with PREDICTION_LATENCY.time():
        try:
            # Check cache
            input_hash = hashlib.sha256(characteristics.json().encode()).hexdigest()
            if redis_client:
                cached = redis_client.get(f"pmt:{input_hash}")
                if cached:
                    PREDICTION_COUNTER.labels(model_type='rule_based', status='cache_hit').inc()
                    return PMTScoreResponse(**json.loads(cached))
            
            # Calculate score
            raw_score, contributions = calculator.calculate_pmt_score(characteristics)
            consumption = calculator.score_to_consumption(raw_score)
            category = calculator.categorize(consumption)
            confidence = calculator.calculate_confidence(characteristics)
            explanation = calculator.generate_explanation(category, contributions)
            
            # Normalize score to 0-100
            normalized_score = raw_score
            
            # Calculate percentile (simplified - in production would use population distribution)
            percentile = min(99, max(1, raw_score))
            
            response = PMTScoreResponse(
                household_id=characteristics.household_id,
                beneficiary_id=characteristics.beneficiary_id,
                raw_score=raw_score,
                normalized_score=normalized_score,
                percentile=percentile,
                category=category,
                confidence=confidence,
                feature_contributions=contributions,
                model_version="rule_based_v1",
                calculated_at=datetime.utcnow(),
                explanation=explanation
            )
            
            # Store score
            db_score = PMTScore(
                household_id=characteristics.household_id,
                beneficiary_id=characteristics.beneficiary_id,
                model_id=0,  # Rule-based model
                model_version="rule_based_v1",
                raw_score=raw_score,
                normalized_score=normalized_score,
                percentile=percentile,
                category=category.value,
                confidence=confidence,
                feature_contributions=contributions,
                input_data_hash=input_hash,
            )
            db.add(db_score)
            db.commit()
            
            # Cache result
            if redis_client:
                redis_client.setex(f"pmt:{input_hash}", 3600, response.json())
            
            PREDICTION_COUNTER.labels(model_type='rule_based', status='success').inc()
            return response
            
        except Exception as e:
            PREDICTION_COUNTER.labels(model_type='rule_based', status='error').inc()
            logger.error(f"PMT calculation error: {e}")
            raise HTTPException(status_code=500, detail=str(e))


@app.post("/calculate/batch", response_model=List[PMTScoreResponse])
async def calculate_pmt_scores_batch(
    households: List[HouseholdCharacteristics],
    db: Session = Depends(get_db),
    calculator: PMTCalculator = Depends(get_calculator)
):
    """Calculate PMT scores for multiple households"""
    results = []
    for household in households:
        try:
            raw_score, contributions = calculator.calculate_pmt_score(household)
            consumption = calculator.score_to_consumption(raw_score)
            category = calculator.categorize(consumption)
            confidence = calculator.calculate_confidence(household)
            explanation = calculator.generate_explanation(category, contributions)
            
            results.append(PMTScoreResponse(
                household_id=household.household_id,
                beneficiary_id=household.beneficiary_id,
                raw_score=raw_score,
                normalized_score=raw_score,
                percentile=min(99, max(1, raw_score)),
                category=category,
                confidence=confidence,
                feature_contributions=contributions,
                model_version="rule_based_v1",
                calculated_at=datetime.utcnow(),
                explanation=explanation
            ))
        except Exception as e:
            logger.error(f"Error processing household {household.household_id}: {e}")
    
    return results


@app.post("/train", response_model=ModelTrainingResponse)
async def train_model(
    request: ModelTrainingRequest,
    background_tasks: BackgroundTasks,
    db: Session = Depends(get_db)
):
    """Train a new PMT model"""
    # In production, this would load training data from a data warehouse
    # For now, generate synthetic training data
    np.random.seed(42)
    n_samples = 10000
    
    training_data = pd.DataFrame({
        'household_size': np.random.randint(1, 10, n_samples),
        'dependency_ratio': np.random.uniform(0, 1, n_samples),
        'head_education_years': np.random.randint(0, 20, n_samples),
        'has_electricity': np.random.choice([True, False], n_samples, p=[0.7, 0.3]),
        'has_piped_water': np.random.choice([True, False], n_samples, p=[0.5, 0.5]),
        'owns_land': np.random.choice([True, False], n_samples, p=[0.4, 0.6]),
        'urban_rural': np.random.choice(['urban', 'rural', 'peri-urban'], n_samples),
        'consumption_per_capita': np.random.lognormal(1.5, 0.8, n_samples)
    })
    
    trainer = PMTModelTrainer(db)
    return trainer.train_model(request, training_data)


@app.get("/models")
async def list_models(db: Session = Depends(get_db)):
    """List all trained PMT models"""
    models = db.query(PMTModel).order_by(PMTModel.created_at.desc()).all()
    return [
        {
            "id": m.id,
            "name": m.name,
            "model_type": m.model_type,
            "version": m.version,
            "is_active": m.is_active,
            "metrics": m.metrics,
            "trained_at": m.trained_at,
        }
        for m in models
    ]


@app.post("/models/{model_id}/activate")
async def activate_model(model_id: int, db: Session = Depends(get_db)):
    """Activate a trained model for production use"""
    # Deactivate all other models
    db.query(PMTModel).update({"is_active": False})
    
    # Activate the specified model
    model = db.query(PMTModel).filter(PMTModel.id == model_id).first()
    if not model:
        raise HTTPException(status_code=404, detail="Model not found")
    
    model.is_active = True
    db.commit()
    
    return {"status": "activated", "model_id": model_id, "version": model.version}


@app.get("/scores/{household_id}")
async def get_household_scores(household_id: str, db: Session = Depends(get_db)):
    """Get historical PMT scores for a household"""
    scores = db.query(PMTScore).filter(
        PMTScore.household_id == household_id
    ).order_by(PMTScore.calculated_at.desc()).limit(10).all()
    
    return [
        {
            "id": s.id,
            "raw_score": s.raw_score,
            "normalized_score": s.normalized_score,
            "category": s.category,
            "confidence": s.confidence,
            "model_version": s.model_version,
            "calculated_at": s.calculated_at,
        }
        for s in scores
    ]


@app.get("/statistics")
async def get_statistics(db: Session = Depends(get_db)):
    """Get PMT scoring statistics"""
    from sqlalchemy import func
    
    total_scores = db.query(func.count(PMTScore.id)).scalar()
    
    category_counts = db.query(
        PMTScore.category,
        func.count(PMTScore.id)
    ).group_by(PMTScore.category).all()
    
    avg_score = db.query(func.avg(PMTScore.normalized_score)).scalar()
    
    return {
        "total_scores": total_scores,
        "category_distribution": {cat: count for cat, count in category_counts},
        "average_score": float(avg_score) if avg_score else 0,
        "timestamp": datetime.utcnow().isoformat()
    }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
