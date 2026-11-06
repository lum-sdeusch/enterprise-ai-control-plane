"""FastAPI application. One analysis endpoint plus health probes."""

from __future__ import annotations

import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, Request

from . import __version__
from .analyzer import Analyzer, RequiredDetectorUnavailable
from .config import Settings, load
from .models import AnalyzeRequest, AnalyzeResponse

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(levelname)s %(message)s")


def create_app(settings: Settings | None = None) -> FastAPI:
    settings = settings or load()

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        app.state.analyzer = Analyzer(settings)
        app.state.settings = settings
        logging.getLogger("eacp.guardrails").info("ready profile=%s detectors=%s", settings.profile, app.state.analyzer.versions())
        yield

    app = FastAPI(title="eacp-guardrails", version=__version__, lifespan=lifespan)

    @app.post("/v1/analyze", response_model=AnalyzeResponse)
    def analyze(req: AnalyzeRequest, request: Request) -> AnalyzeResponse:
        analyzer: Analyzer = request.app.state.analyzer
        try:
            return analyzer.analyze(req)
        except RequiredDetectorUnavailable as exc:
            # 503 makes the adapter fail closed (ADR 0010).
            raise HTTPException(status_code=503, detail=str(exc)) from exc

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/readyz")
    def readyz(request: Request) -> dict[str, object]:
        analyzer: Analyzer | None = getattr(request.app.state, "analyzer", None)
        if analyzer is None:
            raise HTTPException(status_code=503, detail="not ready")
        return {"status": "ok", "detectors": analyzer.versions()}

    return app


app = create_app()
