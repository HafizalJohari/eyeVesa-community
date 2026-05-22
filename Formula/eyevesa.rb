class Eyevesa < Formula
  desc "Identity and trust CLI for the agentic economy"
  homepage "https://github.com/HafizalJohari/eyeVesa-community"
  version "0.1.1"

  on_macos do
    on_arm do
      url "https://raw.githubusercontent.com/HafizalJohari/eyeVesa-community/v0.1.1/cli/eyevesa-arm64"
      sha256 "REPLACE_WITH_REAL_SHA256_ARM64"
    end
    on_intel do
      url "https://raw.githubusercontent.com/HafizalJohari/eyeVesa-community/v0.1.1/cli/eyevesa-amd64"
      sha256 "REPLACE_WITH_REAL_SHA256_AMD64"
    end
  end

  on_linux do
    url "https://raw.githubusercontent.com/HafizalJohari/eyeVesa-community/v0.1.1/cli/eyevesa-amd64"
    sha256 "REPLACE_WITH_REAL_SHA256_AMD64"
  end

  def install
    bin.install Dir["eyevesa-*"].first => "eyevesa"
  end

  test do
    assert_match "eyevesa", shell_output("#{bin}/eyevesa --help")
  end
end
