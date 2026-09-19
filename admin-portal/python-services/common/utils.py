import requests
import logging
from functools import wraps
from flask import jsonify

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def handle_errors(f):
    """Decorator for error handling"""
    @wraps(f)
    def decorated_function(*args, **kwargs):
        try:
            return f(*args, **kwargs)
        except Exception as e:
            logger.error(f"Error in {f.__name__}: {str(e)}")
            return jsonify({"error": str(e)}), 500
    return decorated_function

def validate_required_fields(data, required_fields):
    """Validate required fields in request data"""
    missing = [field for field in required_fields if field not in data]
    if missing:
        raise ValueError(f"Missing required fields: {', '.join(missing)}")
    return True

def download_file(url, timeout=10):
    """Download file from URL"""
    try:
        response = requests.get(url, timeout=timeout)
        response.raise_for_status()
        return response.content
    except requests.RequestException as e:
        raise Exception(f"Failed to download file: {str(e)}")

def call_service(service_url, endpoint, method='POST', data=None, timeout=10):
    """Call another microservice"""
    url = f"{service_url}{endpoint}"
    try:
        if method == 'POST':
            response = requests.post(url, json=data, timeout=timeout)
        elif method == 'GET':
            response = requests.get(url, params=data, timeout=timeout)
        else:
            raise ValueError(f"Unsupported method: {method}")
        
        response.raise_for_status()
        return response.json()
    except requests.RequestException as e:
        raise Exception(f"Service call failed: {str(e)}")
