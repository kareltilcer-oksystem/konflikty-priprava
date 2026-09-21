# Production build: the frontend first, then the binary that embeds it.
# The order is load-bearing — building Go first produces a binary whose SPA is
# the placeholder page.
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

npm --prefix web ci
npm --prefix web run build

$index = Join-Path $root 'internal/spa/dist/index.html'
if (-not (Test-Path $index)) {
    throw "The frontend build produced no internal/spa/dist/index.html; the binary would ship without a UI."
}

$env:CGO_ENABLED = '0'
go build -trimpath -ldflags "-s -w -X main.version=1.0.0" -o konflikty-priprava.exe ./cmd/server
Write-Host "Built konflikty-priprava.exe"
