[CmdletBinding()]
param(
  [ValidateSet("amd64", "arm64")]
  [string]$Architecture = "amd64"
)

$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent $PSScriptRoot
$agentDirectory = Join-Path $projectRoot "agents\linux"
$outputDirectory = Join-Path $agentDirectory "bin"
$outputPath = Join-Path $outputDirectory "gaia-agent-linux-$Architecture"

$originalGOOS = $env:GOOS
$originalGOARCH = $env:GOARCH

try {
  New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

  Push-Location $agentDirectory

  go test .\...
  if ($LASTEXITCODE -ne 0) {
    throw "Linux agent tests failed."
  }

  $env:GOOS = "linux"
  $env:GOARCH = $Architecture

  go build -trimpath -o $outputPath .\cmd\agent
  if ($LASTEXITCODE -ne 0) {
    throw "Linux agent build failed."
  }

  Write-Host "Built Linux agent: $outputPath"
}
finally {
  Pop-Location

  if ($null -eq $originalGOOS) {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
  } else {
    $env:GOOS = $originalGOOS
  }

  if ($null -eq $originalGOARCH) {
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
  } else {
    $env:GOARCH = $originalGOARCH
  }
}