"""健康检查。"""
from fastapi import APIRouter

router = APIRouter()


@router.get("/healthz")
def healthz() -> dict:
    return {"status": "ok", "service": "sentinel-ai", "version": "0.1.0"}
