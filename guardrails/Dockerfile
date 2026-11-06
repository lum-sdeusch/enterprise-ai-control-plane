# syntax=docker/dockerfile:1.7
FROM python:3.12-slim AS build
WORKDIR /src
COPY pyproject.toml README.md ./
COPY eacp_guardrails ./eacp_guardrails
# The base image carries the regex engines only; add ".[presidio,gliner]" for the ML PII stack
# (cloud-gpu profile builds that variant).
RUN pip install --no-cache-dir --prefix=/install .

FROM python:3.12-slim
RUN useradd --uid 10001 --create-home --shell /usr/sbin/nologin eacp
COPY --from=build /install /usr/local
USER 10001
ENV PYTHONUNBUFFERED=1 EACP_GUARDRAILS_PORT=8000
EXPOSE 8000
CMD ["sh", "-c", "uvicorn eacp_guardrails.app:app --host 0.0.0.0 --port ${EACP_GUARDRAILS_PORT}"]
