param(
    [string]$GoExecutable = "C:\Users\brainplusplus\AppData\Local\go-camoufox\browsers\Official\135.0.1-beta.24\camoufox.exe",
    [string]$PythonExecutable = "C:\Users\brainplusplus\AppData\Local\camoufox\camoufox\Cache\browsers\official\135.0.1-beta.24\camoufox.exe"
)

$ErrorActionPreference = "Continue"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

function Show-Section($name) {
    Write-Host "`n==> $name" -ForegroundColor Cyan
}

Show-Section "Go sample"
& go run ./examples/reference_ports/test_google_playwright
if ($LASTEXITCODE -ne 0) {
    Write-Host "Go sample exited with code $LASTEXITCODE" -ForegroundColor Yellow
}

Show-Section "Python upstream sample"
$env:CAMOUFOX_EXECUTABLE = $PythonExecutable
$env:CAMOUFOX_AUTO_CLOSE_SECONDS = "8"
& .\.venv-ref\Scripts\python.exe .\examples\reference_ports\test_google_python_reference.py
if ($LASTEXITCODE -ne 0) {
    Write-Host "Python upstream sample exited with code $LASTEXITCODE" -ForegroundColor Yellow
}
