# -*- coding: utf-8 -*-
"""
文件名: main_test.py
描述: 使用内存引擎替身验证 OCR HTTP 适配层。
"""

import io
from collections.abc import Iterator

import pytest
from fastapi.testclient import TestClient
from PIL import Image

from app import main
from app.engine import ModelNotReadyError


class StubEngine:
    def __init__(self) -> None:
        self.ready = False
        self.text = ""
        self.error: Exception | None = None
        self.images: list[Image.Image] = []

    @property
    def is_ready(self) -> bool:
        return self.ready

    def recognize(self, image: Image.Image) -> str:
        self.images.append(image)
        if self.error is not None:
            raise self.error
        return self.text


@pytest.fixture(autouse=True)
def stub_engine(monkeypatch: pytest.MonkeyPatch) -> Iterator[StubEngine]:
    stub = StubEngine()
    monkeypatch.setattr(main, "engine", stub)
    yield stub
    for image in stub.images:
        image.close()


@pytest.fixture
def client() -> TestClient:
    return TestClient(main.app)


def make_image(image_format: str = "PNG", size: tuple[int, int] = (16, 16)) -> bytes:
    buffer = io.BytesIO()
    Image.new("RGB", size, "white").save(buffer, format=image_format)
    return buffer.getvalue()


def post_image(
    client: TestClient,
    payload: bytes,
    content_type: str = "image/png",
):
    return client.post(
        "/v1/recognize",
        files={"image": ("question", payload, content_type)},
    )


def test_healthz_does_not_require_model(client: TestClient, stub_engine: StubEngine):
    response = client.get("/healthz")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}
    assert stub_engine.images == []


def test_readyz_waits_for_real_model_load(client: TestClient, stub_engine: StubEngine):
    assert client.get("/readyz").status_code == 503

    stub_engine.ready = True
    response = client.get("/readyz")

    assert response.status_code == 200
    assert response.json() == {"status": "ready"}


def test_recognize_rejects_unsupported_mime(client: TestClient):
    response = post_image(client, make_image(), "image/gif")

    assert response.status_code == 415
    assert response.json()["error"]["code"] == "UNSUPPORTED_IMAGE"


def test_recognize_rejects_mime_spoofing(client: TestClient):
    response = post_image(client, make_image("PNG"), "image/jpeg")

    assert response.status_code == 415
    assert response.json()["error"]["code"] == "IMAGE_MIME_MISMATCH"


def test_recognize_rejects_empty_upload(client: TestClient):
    response = post_image(client, b"")

    assert response.status_code == 400
    assert response.json()["error"]["code"] == "EMPTY_IMAGE"


def test_recognize_rejects_oversized_upload(client: TestClient):
    response = post_image(client, b"x" * (main.MAX_IMAGE_BYTES + 1))

    assert response.status_code == 413
    assert response.json()["error"]["code"] == "IMAGE_TOO_LARGE"


def test_recognize_rejects_too_many_pixels(client: TestClient):
    payload = make_image(size=(5001, 4001))

    response = post_image(client, payload)

    assert response.status_code == 413
    assert response.json()["error"]["code"] == "TOO_MANY_PIXELS"


def test_recognize_rejects_corrupt_image(client: TestClient):
    response = post_image(client, b"not-an-image")

    assert response.status_code == 422
    assert response.json()["error"]["code"] == "INVALID_IMAGE"


def test_recognize_returns_503_when_model_is_loading(
    client: TestClient, stub_engine: StubEngine
):
    stub_engine.error = ModelNotReadyError("loading")

    response = post_image(client, make_image())

    assert response.status_code == 503
    assert response.json()["error"]["code"] == "OCR_NOT_READY"


def test_recognize_returns_422_for_empty_result(
    client: TestClient, stub_engine: StubEngine
):
    stub_engine.text = " \n"

    response = post_image(client, make_image())

    assert response.status_code == 422
    assert response.json()["error"]["code"] == "EMPTY_RECOGNITION"


def test_recognize_returns_502_for_engine_failure(
    client: TestClient, stub_engine: StubEngine
):
    stub_engine.error = RuntimeError("inference failed")

    response = post_image(client, make_image())

    assert response.status_code == 502
    assert response.json()["error"]["code"] == "OCR_RECOGNITION_FAILED"


def test_recognize_preserves_chinese_and_math_delimiters(
    client: TestClient, stub_engine: StubEngine
):
    expected = "已知 $x^2=4$\n$$\ny=2x+1\n$$"
    stub_engine.text = expected

    response = post_image(client, make_image())

    assert response.status_code == 200
    assert response.json() == {"text": expected}
    assert stub_engine.images[0].mode == "RGB"


@pytest.mark.parametrize(
    ("image_format", "content_type"),
    [("JPEG", "image/jpeg"), ("WEBP", "image/webp")],
)
def test_recognize_accepts_jpeg_and_webp(
    client: TestClient,
    stub_engine: StubEngine,
    image_format: str,
    content_type: str,
):
    stub_engine.text = "识别文本"

    response = post_image(client, make_image(image_format), content_type)

    assert response.status_code == 200
    assert response.json() == {"text": "识别文本"}
