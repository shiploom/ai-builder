# deploy/docker — local + single-host target (starter)

- `Dockerfile`: pinned `python:3.12-slim`, multi-stage, non-root UID 10001,
  healthcheck on `/health`. Gate: `docker build .` must pass.
- `compose.yml`: app service, `.env` file, read-only rootfs.
- `.env.example`: copy to `.env`; never commit real values.
