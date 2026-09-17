# deploy/preview — preview-host target (starter)

- `vercel.json` / `netlify.toml`: build command + output dir presets.
  Cloudflare Pages: same shape (build output `out/`).
- Env bindings per preview (never secrets in files); note egress and
  build-minute limits in the deployment plan.
- Smoke checklist: preview URL returns 200, seeded-data checks pass,
  environment matches pinned versions — all in `smoke-report.md`
  before the prod gate.
