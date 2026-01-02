# typed: false
# frozen_string_literal: true

# Homebrew formula for qrlocal
# To install from local tap:
#   brew tap dendysatrya/qrlocal https://github.com/dendysatrya/homebrew-qrlocal
#   brew install qrlocal
#
# Or install directly:
#   brew install dendysatrya/qrlocal/qrlocal

class Qrlocal < Formula
  desc "CLI tool to generate QR codes for sharing local services"
  homepage "https://github.com/dendysatrya/qrlocal"
  version "0.0.1-alpha"
  license "MIT"

  on_macos do
    on_intel do
      url "https://github.com/dendysatrya/qrlocal/releases/download/v#{version}/qrlocal_#{version}_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_DARWIN_AMD64"
    end

    on_arm do
      url "https://github.com/dendysatrya/qrlocal/releases/download/v#{version}/qrlocal_#{version}_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_DARWIN_ARM64"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/dendysatrya/qrlocal/releases/download/v#{version}/qrlocal_#{version}_linux_amd64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_LINUX_AMD64"
    end

    on_arm do
      url "https://github.com/dendysatrya/qrlocal/releases/download/v#{version}/qrlocal_#{version}_linux_arm64.tar.gz"
      sha256 "PLACEHOLDER_SHA256_LINUX_ARM64"
    end
  end

  depends_on "openssh"

  def install
    bin.install "qrlocal"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/qrlocal --version")
  end
end
