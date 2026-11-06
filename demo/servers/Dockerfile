# syntax=docker/dockerfile:1.7
# One Dockerfile for the three demo servers: docker build --build-arg SERVER=tickets -f demo/servers/Dockerfile demo/servers
ARG SERVER=tickets
FROM python:3.12-slim
ARG SERVER
RUN pip install --no-cache-dir "fastmcp>=2.12" && useradd --uid 10001 --create-home --shell /usr/sbin/nologin eacp
WORKDIR /app
COPY ${SERVER}/server.py /app/server.py
USER 10001
ENV PORT=8000 PYTHONUNBUFFERED=1
EXPOSE 8000
CMD ["python", "/app/server.py"]
