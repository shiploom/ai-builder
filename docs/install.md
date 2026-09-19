# Install

## Primary: single Go binary (offline-first, stdlib-only)

```bash
brew install shiploom/tap/shiploom
# or: curl -fsSL https://github.com/shiploom/ai-builder/releases/latest/download/install.sh | sh
# or: download shiploom-<version>-<os>-<arch> from
#     https://github.com/shiploom/ai-builder/releases
# or: npx -y @shiploom/cli
```

~6MB, starts in ~5ms, no runtime to install. Requires nothing else;
validators and orchestrator are compiled in.

## From source (contributors)

```bash
go build -trimpath -o shiploom ./cmd/shiploom   # Go binary
uv venv && uv pip install -e ".[dev]"           # Python package
shiploom doctor   # offline compat check, exit 0
```

## Verify the install

```bash
shiploom --version   # tool + core + runtime versions
shiploom doctor      # schemas, validators, harnesses, project state
```
