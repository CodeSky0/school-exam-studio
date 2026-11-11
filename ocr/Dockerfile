FROM python:3.11-slim-bookworm

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIX2TEXT_HOME=/models/pix2text \
    CNOCR_HOME=/models/cnocr \
    CNSTD_HOME=/models/cnstd \
    HF_HOME=/models/huggingface \
    HF_HUB_CACHE=/models/huggingface/hub \
    TORCH_HOME=/models/torch \
    XDG_CACHE_HOME=/models/cache \
    OCR_DEVICE=cpu

RUN apt-get update \
    && apt-get install -y --no-install-recommends libgl1 libglib2.0-0 libgomp1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY requirements.txt ./
RUN python -m pip install --no-cache-dir -r requirements.txt

COPY app ./app

RUN groupadd --system ocr \
    && useradd --system --gid ocr --no-create-home ocr \
    && mkdir -p /models \
    && chown -R ocr:ocr /models /app

USER ocr

EXPOSE 8000

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000", "--workers", "1"]
