# docs/ — Markdown sources of truth + site

These files are the single source of truth. `../site/` renders them via
`npm run docs:sync` (frontmatter injection + JSX-safe escaping); drift is
CI-gated (`docs:check`). Consumer manual (J1–J4), policy cookbook,
brownfield playbook live here. No hosting wired yet — `npm run build`
produces a static `out/` directory.
