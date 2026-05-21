"""请求 / 响应 Pydantic 模型。"""
from typing import List, Optional

from pydantic import BaseModel, Field


class AnalyzeRequest(BaseModel):
    callId: str = ""
    shownNumber: str = Field(..., description="显示号码（含国家码）")
    signalOriginCC: str = Field(default="CN", description="信令源 ISO 3166-1 alpha-2")
    audioSeconds: float = 0.1
    transcriptHint: Optional[str] = None


class TraceVerdict(BaseModel):
    shownRegistry: str
    actualOrigin: str
    mismatch: bool
    hopCount: int
    risk: int
    note: str = ""


class VoiceVerdict(BaseModel):
    synthProbability: float
    f0Jitter: float
    breathScore: float
    regularity: float
    risk: int
    verdict: str  # HUMAN / SUSPECT / SYNTH


class ScriptHit(BaseModel):
    category: str
    phrase: str
    weight: int


class ScriptVerdict(BaseModel):
    hits: List[ScriptHit] = []
    risk: int


class AnalyzeResponse(BaseModel):
    callId: str
    ts: str
    trace: TraceVerdict
    voiceprint: VoiceVerdict
    script: ScriptVerdict
    riskScore: int
    riskLevel: str
    action: str
    latencyMillis: int


class TranscribeRequest(BaseModel):
    audioKey: str = Field(..., description="MinIO 对象 key 或本地路径")
    language: Optional[str] = None


class TranscribeResponse(BaseModel):
    text: str
    language: str
    durationMillis: int


class VoiceprintRequest(BaseModel):
    audioSeconds: float


class ClassifyRequest(BaseModel):
    transcript: str
