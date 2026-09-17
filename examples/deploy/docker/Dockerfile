# Pinned multi-stage image: build deps never ship; runtime runs as non-root.
FROM python:3.12-slim AS build
WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir --prefix=/pkg -r requirements.txt

FROM python:3.12-slim
RUN useradd -m -u 10001 app
COPY --from=build /pkg /usr/local
WORKDIR /app
COPY --chown=app:app . .
USER 10001
EXPOSE 8000
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:8000/health', timeout=4).status == 200 else 1)"
CMD ["python", "-m", "app"]
