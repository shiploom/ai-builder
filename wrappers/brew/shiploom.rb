# Homebrew formula template for shiploom (filled at release time).
#
# This template is NOT installable as-is: URL + sha256 come from the
# published GitHub release source tarball. The tap repo (shiploom/homebrew-tap)
# carries the filled formula.
#
# The formula builds the stdlib-only Go binary from source (no Python
# runtime needed at install or run time).

class Shiploom < Formula
  desc "Portable AI software-engineering layer (specifier/implementer/verifier + deterministic orchestrator)"
  homepage "https://github.com/shiploom/ai-builder"
  # Fill at release: url + sha256 of the vX.Y.Z source tarball.
  url "https://github.com/shiploom/ai-builder/archive/refs/tags/v0.0.0.tar.gz"
  sha256 "FILL_AT_RELEASE"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", "-trimpath",
           "-ldflags", "-X github.com/shiploom/ai-builder/internal/version.CoreVersion=#{version}",
           "-o", bin/"shiploom", "./cmd/shiploom"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/shiploom --version")
    assert_match "OK", shell_output("#{bin}/shiploom doctor")
  end
end
