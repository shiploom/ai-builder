# Release process

Version policy lives in `core/README.md` (semver; `core/VERSION` and
`pyproject.toml` move together). Checklist for `vX.Y.Z`:

1. Bump `core/VERSION` + `pyproject.toml` + `wrappers/npx/package.json` + `CHANGELOG.md` entry.
2. Full gate: `pytest tests/unit -q`, `validate --strict .`,
   `sh scripts/ac-demo.sh`, `shiploom conformance --harness all`,
   plus the Go leg: `gofmt`/`go vet`/`go test`, stamped-binary parity,
   `SHIPLOOM_GO_BIN=<bin> sh scripts/ac-demo.sh`.
3. Commit, then tag and push: `git tag vX.Y.Z && git push origin vX.Y.Z`.
   The tag must equal both version files — `.github/workflows/release.yml`
   fails the release otherwise.
4. The workflow validates, builds sdist + wheel, generates `dist/sbom.json`
   (CycloneDX), and attaches everything to the GitHub release.

## Signing (manual until tooled)

After the workflow publishes:

```bash
cosign sign-blob --yes dist/shiploom_core-X.Y.Z.tar.gz > dist/*.sig
gh release upload vX.Y.Z dist/*.sig
```

Threshold signatures and a transparency log stay Advanced; single-maintainer
cosign is the current bar. Never publish without the SBOM artifact.
