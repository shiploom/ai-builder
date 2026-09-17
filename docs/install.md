# Install

## Primary (today)

```bash
pipx install git+https://github.com/shiploom/ai-builder.git
# or: uvx --from git+https://github.com/shiploom/ai-builder shiploom
```

Requires Python ≥3.9, no other runtime. Validators are stdlib-only.

## From source (contributors)

```bash
uv venv && uv pip install -e ".[dev]"
shiploom doctor   # offline compat check, exit 0
```

## Verify the install

```bash
shiploom --version   # tool + core + python versions
shiploom doctor      # schemas, validators, harnesses, project state
```

## Coming later

- `brew tap shiploom/tap` once the tap repo exists (formula template:
  `wrappers/brew/shiploom.rb`; URL + sha256 are filled at release time).
- No Go single binary is planned (see spec §2.4: it pays off at 10k+ users).
