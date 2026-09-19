#!/bin/bash

# Start ML Document Validation Service with Dapr

echo "Starting ML Document Validation Service..."

# Start with Dapr sidecar
dapr run \
  --app-id ml-document-service \
  --app-port 8001 \
  --dapr-http-port 3501 \
  --dapr-grpc-port 50051 \
  --components-path ./ml-service \
  -- python3 ./ml-service/app.py
