FROM python:3.11-slim
WORKDIR /app
COPY python-services/ray-service/requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt
COPY ml /app/ml
COPY python-services/ray-service/app.py /app/app.py
RUN useradd -m -u 1000 appuser && mkdir -p /registry /data && chown -R appuser:appuser /app /registry /data
USER appuser
EXPOSE 5005
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:5005/health')" || exit 1
CMD ["python", "/app/app.py"]
