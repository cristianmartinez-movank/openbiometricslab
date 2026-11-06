FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    TORCH_HOME=/opt/openbiometrics/ocr-cache \
    DOCTR_CACHE_DIR=/opt/openbiometrics/ocr-cache \
    HOME=/opt/openbiometrics

WORKDIR /opt/openbiometrics

RUN apt-get update && apt-get install -y --no-install-recommends \
    libegl1 \
    libgl1 \
    libgles2 \
    libglib2.0-0 \
    patch \
    && rm -rf /var/lib/apt/lists/*

COPY openbiometrics/VERSION ./VERSION
COPY openbiometrics/engine/ ./engine/
COPY openbiometrics/api/ ./api/
COPY containers/patches/0001-pipeline-config.patch /tmp/0001-pipeline-config.patch
COPY containers/patches/0002-enable-document-face-extraction.patch /tmp/0002-enable-document-face-extraction.patch
COPY containers/patches/0003-improve-document-face-detection.patch /tmp/0003-improve-document-face-detection.patch

RUN cp VERSION engine/VERSION \
    && cp VERSION api/VERSION \
    && sed -i 's|file = "../VERSION"|file = "VERSION"|' engine/pyproject.toml api/pyproject.toml \
    && patch --directory=engine --strip=1 --input=/tmp/0001-pipeline-config.patch \
    && patch --directory=api --strip=1 --input=/tmp/0002-enable-document-face-extraction.patch \
    && patch --directory=api --strip=1 --input=/tmp/0003-improve-document-face-detection.patch \
    && rm /tmp/0001-pipeline-config.patch /tmp/0002-enable-document-face-extraction.patch /tmp/0003-improve-document-face-detection.patch \
    && mkdir -p /opt/openbiometrics/ocr-cache \
    && python -m pip install --no-cache-dir './engine[document,liveness]' \
    && python -m pip install --no-cache-dir ./api \
    && python -c "from openbiometrics.document.ocr import DocumentOCR; DocumentOCR()._ensure_loaded()" \
    && python -c "from openbiometrics.liveness.landmarks import FaceMeshDetector; FaceMeshDetector()" \
    && groupadd --system --gid 10001 openbiometrics \
    && useradd --system --uid 10001 --gid openbiometrics --no-create-home openbiometrics \
    && mkdir -p /opt/openbiometrics/api/watchlists \
    && chown -R 10001:10001 /opt/openbiometrics/api/watchlists /opt/openbiometrics/ocr-cache

WORKDIR /opt/openbiometrics/api
USER 10001:10001

EXPOSE 8000

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
