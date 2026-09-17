# Homebrew formula template for shiploom (filled at release time).
#
# This template is NOT installable as-is: URL + sha256 come from the
# published GitHub release tarball. The tap repo (shiploom/homebrew-tap,
# post-MVP) will carry the filled formula. Rationale: a formula pointing
# at unpublished artifacts is worse than none.

class Shiploom < Formula
  desc "Portable AI software-engineering layer (specifier/implementer/verifier + deterministic orchestrator)"
  homepage "https://github.com/shiploom/ai-builder"
  # Fill at release: url + sha256 of shiploom-core-<version>.tar.gz
  url "https://github.com/shiploom/ai-builder/releases/download/v0.0.0/shiploom-core-0.0.0.tar.gz"
  sha256 "FILL_AT_RELEASE"
  license "MIT"

  depends_on "python@3.13"

  def install
    virtualenv_install_with_resources
  end

  test do
    assert_match "core", shell_output("#{bin}/shiploom --version")
  end
end
