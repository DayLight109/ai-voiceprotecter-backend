"""端到端冒烟测试：起 FastAPI test client。

骨架阶段：
  - healthz 始终可用
  - /v1/analyze 在缺模型 + 缺 LLM 时应返回 503 MODEL_UNAVAILABLE
真实推理路径的测试（BLOCK / SAFE）由 P3 阶段接入真模型后补。
"""
from fastapi.testclient import TestClient

from app.main import app

client = TestClient(app)


def test_healthz() -> None:
    r = client.get("/healthz")
    assert r.status_code == 200
    assert r.json()["status"] == "ok"


def test_analyze_without_models_returns_503() -> None:
    """骨架阶段未挂 ONNX、未配 DashScope，应明确告知缺哪一层而非编造结果。"""
    r = client.post(
        "/v1/analyze",
        json={
            "callId": "t-001",
            "shownNumber": "+86-138-0013-4921",
            "signalOriginCC": "MM",
            "audioSeconds": 6.4,
            "transcriptHint": "打到这个安全账户",
        },
    )
    assert r.status_code == 503
    detail = r.json().get("detail", {})
    assert detail.get("code") == "MODEL_UNAVAILABLE"
    failed_layers = {f["layer"] for f in detail.get("failures", [])}
    # 缺哪个都行，但必须明示，不能伪造
    assert failed_layers.issubset({"voiceprint", "script"})
    assert failed_layers  # 至少有一项


def test_analyze_validation() -> None:
    """缺 shownNumber 应是 422 而非 503。"""
    r = client.post("/v1/analyze", json={"callId": "t-x"})
    assert r.status_code == 422
