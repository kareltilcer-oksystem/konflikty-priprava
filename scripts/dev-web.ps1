# Runs the Vite dev server on 9999, proxying /api to the Go API on 9998.
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)
if (-not (Test-Path 'web/node_modules')) { npm --prefix web install }
npm --prefix web run dev
