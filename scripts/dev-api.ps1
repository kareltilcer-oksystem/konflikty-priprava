# Runs the Go API for development. Reads .env (or .env.example as a fallback)
# and starts the server with WEB_PORT forced to 0, since Vite owns 9999.
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$envFile = Join-Path $root '.env'
if (-not (Test-Path $envFile)) {
    Write-Host 'No .env found — using .env.example. Copy it to .env to customise.'
    $envFile = Join-Path $root '.env.example'
}

Get-Content $envFile | ForEach-Object {
    $line = $_.Trim()
    # Only '#' starts a comment. ';' must never be treated as one: it separates
    # the accounts inside AUTH_USERS.
    if ($line -eq '' -or $line.StartsWith('#')) { return }
    $i = $line.IndexOf('=')
    if ($i -lt 1) { return }
    $name = $line.Substring(0, $i).Trim()
    $value = $line.Substring($i + 1)
    Set-Item -Path "env:$name" -Value $value
}

$env:WEB_PORT = '0'
Write-Host "API on http://localhost:$($env:API_PORT); the SPA listener is off (Vite owns 9999)."
go run ./cmd/server
