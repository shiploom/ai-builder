# examples/deploy/ — deployment target starters

One directory per target; copy the one you need into your project and
adapt. All starters assume secrets-via-env, non-root runtimes, and a
human prod gate — see the `deployment-plan` skill.

- `docker/` — multi-stage Dockerfile + compose (local/single-host).
- `k8s/` — Kustomize base: probed deployment, service, default-deny netpol.
- `aws/` — Terraform stub: pins, backend placeholder, spend-gate workflow.
- `preview/` — Vercel/Netlify/Cloudflare presets + smoke checklist.
