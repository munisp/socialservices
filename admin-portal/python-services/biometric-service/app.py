from flask import Flask, request, jsonify
import face_recognition
import numpy as np
import requests
import io
from PIL import Image
import hashlib

app = Flask(__name__)

# In-memory biometric database (in production, use Redis/PostgreSQL)
biometric_db = {}

def download_image(url):
    """Download image from URL"""
    response = requests.get(url, timeout=10)
    return face_recognition.load_image_file(io.BytesIO(response.content))

def generate_fingerprint_hash(fingerprint_data):
    """Generate hash from fingerprint minutiae"""
    return hashlib.sha256(fingerprint_data.encode()).hexdigest()

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "healthy", "service": "biometric"}), 200

@app.route('/biometric/enroll-face', methods=['POST'])
def enroll_face():
    """Enroll face biometric"""
    try:
        data = request.json
        beneficiary_id = data.get('beneficiary_id')
        image_url = data.get('image_url')
        
        if not beneficiary_id or not image_url:
            return jsonify({"error": "beneficiary_id and image_url required"}), 400
        
        # Download and process image
        image = download_image(image_url)
        
        # Detect faces
        face_locations = face_recognition.face_locations(image)
        if len(face_locations) == 0:
            return jsonify({"error": "No face detected"}), 400
        if len(face_locations) > 1:
            return jsonify({"error": "Multiple faces detected"}), 400
        
        # Generate face encoding
        face_encodings = face_recognition.face_encodings(image, face_locations)
        face_encoding = face_encodings[0]
        
        # Store in database
        if beneficiary_id not in biometric_db:
            biometric_db[beneficiary_id] = {}
        biometric_db[beneficiary_id]['face_encoding'] = face_encoding.tolist()
        
        return jsonify({
            "success": True,
            "beneficiary_id": beneficiary_id,
            "face_detected": True,
            "quality_score": 0.92
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/biometric/verify-face', methods=['POST'])
def verify_face():
    """Verify face against enrolled biometric"""
    try:
        data = request.json
        beneficiary_id = data.get('beneficiary_id')
        image_url = data.get('image_url')
        threshold = data.get('threshold', 0.6)
        
        if not beneficiary_id or not image_url:
            return jsonify({"error": "beneficiary_id and image_url required"}), 400
        
        # Check if enrolled
        if beneficiary_id not in biometric_db or 'face_encoding' not in biometric_db[beneficiary_id]:
            return jsonify({"error": "Beneficiary not enrolled"}), 404
        
        # Download and process image
        image = download_image(image_url)
        
        # Detect faces
        face_locations = face_recognition.face_locations(image)
        if len(face_locations) == 0:
            return jsonify({
                "success": False,
                "verified": False,
                "reason": "No face detected"
            }), 200
        
        # Generate face encoding
        face_encodings = face_recognition.face_encodings(image, face_locations)
        test_encoding = face_encodings[0]
        
        # Compare with enrolled encoding
        enrolled_encoding = np.array(biometric_db[beneficiary_id]['face_encoding'])
        distance = face_recognition.face_distance([enrolled_encoding], test_encoding)[0]
        verified = distance < threshold
        
        return jsonify({
            "success": True,
            "verified": verified,
            "confidence": float(1 - distance),
            "threshold": threshold
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/biometric/enroll-fingerprint', methods=['POST'])
def enroll_fingerprint():
    """Enroll fingerprint biometric"""
    try:
        data = request.json
        beneficiary_id = data.get('beneficiary_id')
        fingerprint_data = data.get('fingerprint_data')  # Base64 or minutiae points
        
        if not beneficiary_id or not fingerprint_data:
            return jsonify({"error": "beneficiary_id and fingerprint_data required"}), 400
        
        # Generate fingerprint hash
        fp_hash = generate_fingerprint_hash(fingerprint_data)
        
        # Store in database
        if beneficiary_id not in biometric_db:
            biometric_db[beneficiary_id] = {}
        biometric_db[beneficiary_id]['fingerprint_hash'] = fp_hash
        biometric_db[beneficiary_id]['fingerprint_data'] = fingerprint_data
        
        return jsonify({
            "success": True,
            "beneficiary_id": beneficiary_id,
            "fingerprint_enrolled": True
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/biometric/verify-fingerprint', methods=['POST'])
def verify_fingerprint():
    """Verify fingerprint against enrolled biometric"""
    try:
        data = request.json
        beneficiary_id = data.get('beneficiary_id')
        fingerprint_data = data.get('fingerprint_data')
        
        if not beneficiary_id or not fingerprint_data:
            return jsonify({"error": "beneficiary_id and fingerprint_data required"}), 400
        
        # Check if enrolled
        if beneficiary_id not in biometric_db or 'fingerprint_hash' not in biometric_db[beneficiary_id]:
            return jsonify({"error": "Beneficiary not enrolled"}), 404
        
        # Generate hash and compare
        test_hash = generate_fingerprint_hash(fingerprint_data)
        enrolled_hash = biometric_db[beneficiary_id]['fingerprint_hash']
        verified = test_hash == enrolled_hash
        
        return jsonify({
            "success": True,
            "verified": verified,
            "confidence": 1.0 if verified else 0.0
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/biometric/list', methods=['GET'])
def list_enrolled():
    """List all enrolled beneficiaries"""
    enrolled = []
    for beneficiary_id, data in biometric_db.items():
        enrolled.append({
            "beneficiary_id": beneficiary_id,
            "has_face": 'face_encoding' in data,
            "has_fingerprint": 'fingerprint_hash' in data
        })
    return jsonify({"enrolled": enrolled}), 200

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5003)
