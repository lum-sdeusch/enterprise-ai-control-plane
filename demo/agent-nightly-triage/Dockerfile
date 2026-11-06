# syntax=docker/dockerfile:1.7
FROM python:3.12-slim
RUN pip install --no-cache-dir "pydantic-ai-slim[openai,mcp]>=1.0" "httpx>=0.27" \
      "opentelemetry-sdk>=1.27" "opentelemetry-exporter-otlp-proto-http>=1.27" \
    && useradd --uid 10001 --create-home --shell /usr/sbin/nologin eacp
WORKDIR /app
COPY agent.py /app/agent.py
USER 10001
ENV PYTHONUNBUFFERED=1
CMD ["python", "/app/agent.py"]
