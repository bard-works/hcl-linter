class HLCLinter < Formula
  desc "Configurable HCL linter with built-in rule sets for Terragrunt and Terraform"
  homepage "https://github.com/bard-works/hcl-linter"
  url "https://github.com/bard-works/hcl-linter/releases/download/v0.1.0/hcl-linter_0.1.0_darwin_arm64.tar.gz"
  version "0.1.0"
  sha256 "REPLACE_WITH_ACTUAL_SHA256"

  license "Apache-2.0"

  def install
    bin.install "hcl-linter"

    # Install man page (optional)
    # man1.install "hcl-linter.1"
  end

  test do
    system "#{bin}/hcl-linter", "--version"
  end
end