from flask import Flask, request, jsonify
from sedona.spark import *
from pyspark.sql import SparkSession
from shapely.geometry import Point, Polygon
from shapely import wkt
import json

app = Flask(__name__)

# Initialize Spark with Sedona
spark = SparkSession.builder \
    .appName("SedonaService") \
    .config("spark.serializer", "org.apache.spark.serializer.KryoSerializer") \
    .config("spark.kryo.registrator", "org.apache.sedona.core.serde.SedonaKryoRegistrator") \
    .getOrCreate()

SedonaRegistrator.registerAll(spark)

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "healthy", "service": "sedona"}), 200

@app.route('/geo/point-in-polygon', methods=['POST'])
def point_in_polygon():
    """Check if point is within polygon"""
    try:
        data = request.json
        lat = data.get('latitude')
        lon = data.get('longitude')
        polygon_wkt = data.get('polygon')
        
        if lat is None or lon is None or not polygon_wkt:
            return jsonify({"error": "latitude, longitude, and polygon required"}), 400
        
        point = Point(lon, lat)
        polygon = wkt.loads(polygon_wkt)
        
        contains = polygon.contains(point)
        
        return jsonify({
            "success": True,
            "contains": contains,
            "point": {"latitude": lat, "longitude": lon}
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/geo/distance', methods=['POST'])
def calculate_distance():
    """Calculate distance between two points"""
    try:
        data = request.json
        point1 = data.get('point1')  # {lat, lon}
        point2 = data.get('point2')  # {lat, lon}
        
        if not point1 or not point2:
            return jsonify({"error": "point1 and point2 required"}), 400
        
        from geopy.distance import geodesic
        
        p1 = (point1['latitude'], point1['longitude'])
        p2 = (point2['latitude'], point2['longitude'])
        
        distance_km = geodesic(p1, p2).kilometers
        distance_m = distance_km * 1000
        
        return jsonify({
            "success": True,
            "distance_km": distance_km,
            "distance_m": distance_m
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/geo/find-nearby', methods=['POST'])
def find_nearby():
    """Find points within radius"""
    try:
        data = request.json
        center_lat = data.get('latitude')
        center_lon = data.get('longitude')
        radius_km = data.get('radius_km', 10)
        points = data.get('points', [])
        
        if center_lat is None or center_lon is None:
            return jsonify({"error": "latitude and longitude required"}), 400
        
        from geopy.distance import geodesic
        
        center = (center_lat, center_lon)
        nearby = []
        
        for point in points:
            point_coords = (point['latitude'], point['longitude'])
            distance = geodesic(center, point_coords).kilometers
            
            if distance <= radius_km:
                nearby.append({
                    **point,
                    "distance_km": distance
                })
        
        # Sort by distance
        nearby.sort(key=lambda x: x['distance_km'])
        
        return jsonify({
            "success": True,
            "found_count": len(nearby),
            "nearby_points": nearby
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/geo/cluster-analysis', methods=['POST'])
def cluster_analysis():
    """Perform spatial clustering"""
    try:
        data = request.json
        points = data.get('points', [])
        num_clusters = data.get('num_clusters', 5)
        
        if not points:
            return jsonify({"error": "points required"}), 400
        
        from sklearn.cluster import KMeans
        import numpy as np
        
        # Extract coordinates
        coords = np.array([[p['latitude'], p['longitude']] for p in points])
        
        # Perform clustering
        kmeans = KMeans(n_clusters=num_clusters, random_state=42)
        labels = kmeans.fit_predict(coords)
        centers = kmeans.cluster_centers_
        
        # Group points by cluster
        clusters = {}
        for i, label in enumerate(labels):
            label_str = str(label)
            if label_str not in clusters:
                clusters[label_str] = []
            clusters[label_str].append({
                **points[i],
                "cluster_id": int(label)
            })
        
        # Format cluster centers
        cluster_centers = [
            {"latitude": float(center[0]), "longitude": float(center[1])}
            for center in centers
        ]
        
        return jsonify({
            "success": True,
            "num_clusters": num_clusters,
            "cluster_centers": cluster_centers,
            "clusters": clusters
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/geo/heatmap-data', methods=['POST'])
def heatmap_data():
    """Generate heatmap data for visualization"""
    try:
        data = request.json
        points = data.get('points', [])
        grid_size = data.get('grid_size', 0.01)  # degrees
        
        if not points:
            return jsonify({"error": "points required"}), 400
        
        # Create grid and count points
        grid = {}
        for point in points:
            lat = round(point['latitude'] / grid_size) * grid_size
            lon = round(point['longitude'] / grid_size) * grid_size
            key = f"{lat},{lon}"
            grid[key] = grid.get(key, 0) + 1
        
        # Convert to heatmap format
        heatmap = [
            {
                "latitude": float(key.split(',')[0]),
                "longitude": float(key.split(',')[1]),
                "intensity": count
            }
            for key, count in grid.items()
        ]
        
        return jsonify({
            "success": True,
            "heatmap_points": heatmap,
            "grid_size": grid_size
        }), 200
        
    except Exception as e:
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5006)
