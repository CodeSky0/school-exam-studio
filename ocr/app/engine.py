# -*- coding: utf-8 -*-
"""
文件名: engine.py
描述: 线程安全的 Pix2Text 懒加载与识别引擎。
"""

import os
import threading
from collections.abc import Callable
from typing import Any

from PIL import Image


class ModelNotReadyError(RuntimeError):
    pass


def _create_model() -> Any:
    from pix2text import TextFormulaOCR

    return TextFormulaOCR.from_config(
        enable_formula=True,
        enable_spell_checker=False,
        device=os.getenv("OCR_DEVICE", "cpu"),
    )


def _merge_recognition_result(result: Any) -> str:
    from pix2text import merge_line_texts

    if not isinstance(result, list) or any(
        not isinstance(item, dict) for item in result
    ):
        raise TypeError("Pix2Text 1.1.7 应返回字典列表")
    return merge_line_texts(
        result,
        auto_line_break=True,
        line_sep="\n",
        embed_sep=(" $", "$ "),
        isolated_sep=("$$\n", "\n$$"),
        spellchecker=None,
    )


class Pix2TextEngine:
    def __init__(self, model_factory: Callable[[], Any] | None = None) -> None:
        self._model_factory = model_factory or _create_model
        self._model: Any | None = None
        self._ready = threading.Event()
        self._load_lock = threading.Lock()
        self._inference_lock = threading.Lock()

    @property
    def is_ready(self) -> bool:
        return self._ready.is_set()

    def _get_model(self) -> Any:
        if self._model is not None:
            return self._model
        if not self._load_lock.acquire(blocking=False):
            raise ModelNotReadyError("Pix2Text 模型正在加载")
        try:
            if self._model is None:
                model = self._model_factory()
                if model is None:
                    raise RuntimeError("Pix2Text 模型加载失败")
                self._model = model
                self._ready.set()
            return self._model
        finally:
            self._load_lock.release()

    def recognize(self, image: Image.Image) -> str:
        model = self._get_model()
        with self._inference_lock:
            result = model.recognize(
                image,
                return_text=False,
                save_analysis_res=None,
            )
            return _merge_recognition_result(result)
