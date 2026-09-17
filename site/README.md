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

## Hosting (GitHub Pages, live)

`.github/workflows/docs-site.yml` builds with `PAGES_BASE_PATH` +
`NEXT_PUBLIC_BASE_PATH` set to `/ai-builder` and publishes `site/out`.
Live at `https://shiploom.github.io/ai-builder` (repo Settings → Pages →
Source: GitHub Actions). Local builds stay unprefixed — `next/link`
picks up the base path automatically.
