# -*- coding: utf-8 -*-
"""
文件名: main.py
描述: 提供健康检查、就绪检查和内存图片 OCR 接口。
"""

import io
import logging
import warnings
from typing import NoReturn

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from PIL import Image, UnidentifiedImageError
from starlette.concurrency import run_in_threadpool
from starlette.datastructures import UploadFile
from starlette.exceptions import HTTPException as StarletteHTTPException
from starlette.formparsers import MultiPartException, MultiPartParser

from app.engine import ModelNotReadyError, Pix2TextEngine

MAX_IMAGE_BYTES = 5 * 1024 * 1024
MAX_IMAGE_PIXELS = 20_000_000
ALLOWED_FORMATS = {
    "image/jpeg": "JPEG",
    "image/png": "PNG",
    "image/webp": "WEBP",
}

logger = logging.getLogger(__name__)
MultiPartParser.spool_max_size = MAX_IMAGE_BYTES + 1

app = FastAPI(title="Local Pix2Text OCR", docs_url=None, redoc_url=None)
engine = Pix2TextEngine()


class SidecarError(Exception):
    def __init__(self, status_code: int, code: str, message: str) -> None:
        super().__init__(message)
        self.status_code = status_code
        self.code = code
        self.message = message


def _error_payload(code: str, message: str) -> dict[str, dict[str, str]]:
    return {"error": {"code": code, "message": message}}


def _fail(status_code: int, code: str, message: str) -> NoReturn:
    raise SidecarError(status_code, code, message)


@app.exception_handler(SidecarError)
async def handle_sidecar_error(_: Request, exc: SidecarError) -> JSONResponse:
    return JSONResponse(
        status_code=exc.status_code,
        content=_error_payload(exc.code, exc.message),
    )


@app.exception_handler(RequestValidationError)
async def handle_validation_error(
    _: Request, exc: RequestValidationError
) -> JSONResponse:
    return JSONResponse(
        status_code=422,
        content=_error_payload("VALIDATION_ERROR", "请求内容未通过校验"),
    )


@app.exception_handler(StarletteHTTPException)
async def handle_http_error(_: Request, exc: StarletteHTTPException) -> JSONResponse:
    return JSONResponse(
        status_code=exc.status_code,
        content=_error_payload("HTTP_ERROR", str(exc.detail)),
    )


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/readyz")
async def readyz() -> dict[str, str]:
    if not engine.is_ready:
        _fail(503, "OCR_NOT_READY", "Pix2Text 模型尚未加载完成")
    return {"status": "ready"}


def _decode_image(payload: bytes, content_type: str) -> Image.Image:
    try:
        with warnings.catch_warnings():
            warnings.simplefilter("error", Image.DecompressionBombWarning)
            with Image.open(io.BytesIO(payload)) as candidate:
                actual_format = candidate.format
                if actual_format not in ALLOWED_FORMATS.values():
                    _fail(415, "UNSUPPORTED_IMAGE", "仅支持 JPEG、PNG 或 WebP 图片")
                if actual_format != ALLOWED_FORMATS[content_type]:
                    _fail(415, "IMAGE_MIME_MISMATCH", "图片真实格式与 MIME 不一致")
                if candidate.width * candidate.height > MAX_IMAGE_PIXELS:
                    _fail(413, "TOO_MANY_PIXELS", "图片像素数不能超过 2000 万")
                candidate.load()
                return candidate.convert("RGB")
    except (Image.DecompressionBombWarning, Image.DecompressionBombError):
        _fail(413, "TOO_MANY_PIXELS", "图片像素数不能超过 2000 万")
    except (UnidentifiedImageError, OSError, SyntaxError, ValueError):
        _fail(422, "INVALID_IMAGE", "图片无效、损坏或无法完整解码")


async def _read_image(request: Request) -> tuple[bytes, str]:
    try:
        async with request.form(
            max_files=1,
            max_fields=0,
            max_part_size=MAX_IMAGE_BYTES + 1,
        ) as form:
            upload = form.get("image")
            if not isinstance(upload, UploadFile):
                _fail(422, "IMAGE_REQUIRED", "multipart 必须包含图片字段 image")
            content_type = (upload.content_type or "").lower()
            if content_type not in ALLOWED_FORMATS:
                _fail(415, "UNSUPPORTED_IMAGE", "仅支持 JPEG、PNG 或 WebP 图片")
            payload = await upload.read(MAX_IMAGE_BYTES + 1)
    except MultiPartException as exc:
        if "exceeded maximum size" in str(exc):
            _fail(413, "IMAGE_TOO_LARGE", "图片大小不能超过 5MB")
        _fail(400, "INVALID_MULTIPART", "multipart 请求格式无效")
    if not payload:
        _fail(400, "EMPTY_IMAGE", "图片内容不能为空")
    if len(payload) > MAX_IMAGE_BYTES:
        _fail(413, "IMAGE_TOO_LARGE", "图片大小不能超过 5MB")
    return payload, content_type


@app.post("/v1/recognize")
async def recognize(request: Request) -> dict[str, str]:
    payload, content_type = await _read_image(request)
    image = _decode_image(payload, content_type)
    try:
        try:
            text = await run_in_threadpool(engine.recognize, image)
        except ModelNotReadyError:
            _fail(503, "OCR_NOT_READY", "Pix2Text 模型正在加载，请稍后重试")
        except Exception:
            logger.exception("Pix2Text recognition failed")
            _fail(502, "OCR_RECOGNITION_FAILED", "Pix2Text 识别失败")
    finally:
        image.close()
    if not text.strip():
        _fail(422, "EMPTY_RECOGNITION", "未识别到文本或公式")
    return {"text": text.strip()}
