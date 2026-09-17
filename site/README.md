# site/ — documentation site (Next.js + Fumadocs)

Landing page + `/docs` generated from `../docs/` — the Markdown files at the
repo root are the single source of truth.

```bash
cd site
npm install
npm run dev        # local preview (syncs docs first via predev)
npm run build      # static production build (syncs via prebuild)
```

## Docs pipeline

- `npm run docs:sync` — regenerate `content/docs/*.mdx` from `../docs/*.md`
  (frontmatter injection + JSX-safe escaping of prose).
- `npm run docs:check` — fail (exit 2) if generated content drifts (CI gate).
- `node scripts/sync-docs.mjs --self-test` — escaper unit checks.
- `content/docs/index.mdx` + `meta.json` are hand-authored site structure,
  not synced.

No hosting wired yet (deploy decision open); `npm run build` output is a
static export candidate.
