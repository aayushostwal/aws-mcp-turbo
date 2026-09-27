class AwsMcpTurbo < Formula
  desc "Token-conscious AWS MCP server with projection and delta log polling"
  homepage "https://github.com/aayushostwal/aws-mcp-turbo"
  license "MIT"
  head "https://github.com/aayushostwal/aws-mcp-turbo.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", "-trimpath", "-ldflags=-s -w -X main.version=head",
           "-o", bin/"aws-mcp-turbo", "./cmd/aws-mcp-turbo"
  end

  test do
    assert_match "head", shell_output("#{bin}/aws-mcp-turbo --version")
  end
end
