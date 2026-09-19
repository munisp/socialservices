from flask import Flask, request, jsonify
import pytesseract
from PIL import Image
import io
import requests
import cv2
import numpy as np
import re

app = Flask(__name__)

def preprocess_image(image):
    """Preprocess image for better OCR accuracy"""
    img_array = np.array(image)
    gray = cv2.cvtColor(img_array, cv2.COLOR_RGB2GRAY)
    denoised = cv2.fastNlMeansDenoising(gray)
    _, binary = cv2.threshold(denoised, 0, 255, cv2.THRESH_BINARY + cv2.THRESH_OTSU)
    return Image.fromarray(binary)

def extract_id_fields(text):
    """Extract structured fields from ID document"""
    fields = {}
    
    # ID number patterns
    id_patterns = [
        r'ID\s*(?:NO|NUMBER)?[:\s]+([A-Z0-9]{8,15})',
        r'(?:NATIONAL|IDENTITY)\s+(?:ID|NUMBER)[:\s]+([A-Z0-9]{8,15})',
    ]
    for pattern in id_patterns:
        match = re.search(pattern, text, re.IGNORECASE)
        if match:
            fields['id_number'] = match.group(1)
            break
    
    # Name patterns
    name_patterns = [
        r'NAME[:\s]+([A-Z\s]+)',
        r'FULL\s+NAME[:\s]+([A-Z\s]+)',
    ]
    for pattern in name_patterns:
        match = re.search(pattern, text, re.IGNORECASE)
        if match:
            fields['name'] = match.group(1).strip()
            break
    
    # Date of birth
    dob_patterns = [
        r'(?:DOB|DATE\s+OF\s+BIRTH)[:\s]+(\d{1,2}[/-]\d{1,2}[/-]\d{2,4})',
        r'BORN[:\s]+(\d{1,2}[/-]\d{1,2}[/-]\d{2,4})',
    ]
    for pattern in dob_patterns:
        match = re.search(pattern, text, re.IGNORECASE)
        if match:
            fields['date_of_birth'] = match.group(1)
            break
    
    return fields

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "healthy", "service": "ocr"}), 200

@app.route('/ocr/extract', methods=['POST'])
def extract_text():
    """Extract text from image"""
    try:
        data = request.json
        image_url = data.get('image_url')
        
        if not image_url:
            return jsonify({"error": "image_url required"}), 400
        
        # Download image
        response = requests.get(image_url, timeout=10)
        image = Image.open(io.BytesIO(response.content))
        
        # Extract text
        text = pytesseract.image_to_string(image)
        
        return jsonify({
            "success": True,
            "text": text,
            "confidence": 0.85
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/ocr/extract-id', methods=['POST'])
def extract_id():
    """Extract structured data from ID document"""
    try:
        data = request.json
        image_url = data.get('image_url')
        
        if not image_url:
            return jsonify({"error": "image_url required"}), 400
        
        # Download image
        response = requests.get(image_url, timeout=10)
        image = Image.open(io.BytesIO(response.content))
        
        # Preprocess
        processed = preprocess_image(image)
        
        # Extract text
        text = pytesseract.image_to_string(processed)
        
        # Extract fields
        fields = extract_id_fields(text)
        
        return jsonify({
            "success": True,
            "raw_text": text,
            "fields": fields
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5002)
