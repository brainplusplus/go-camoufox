param(
    [switch]$SkipDockerBuild,
    [switch]$Live,
    [string]$Executable = $env:GO_CAMOUFOX_EXECUTABLE,
    [string]$DockerTag = "go-camoufox:local",
    [string]$Version = "0.2.0"
)

$ErrorActionPreference = "Stop"

function Step($Name, [scriptblock]$Body) {
    Write-Host "`n==> $Name" -ForegroundColor Cyan
    & $Body
}

function Run($File, [string[]]$CommandArgs) {
    Write-Host "+ $File $($CommandArgs -join ' ')" -ForegroundColor DarkGray
    & $File @CommandArgs
    if ($LASTEXITCODE -ne 0) {
        throw "command failed: $File $($CommandArgs -join ' ')"
    }
}

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

Step "go version" {
    Run "go" @("version")
}

Step "go mod tidy check" {
    $before = git diff -- go.mod go.sum
    Run "go" @("mod", "tidy")
    $after = git diff -- go.mod go.sum
    if (($before -join "`n") -ne ($after -join "`n")) {
        Write-Host $after
        throw "go.mod/go.sum changed after go mod tidy"
    }
}

Step "unit tests" {
    Run "go" @("test", "./...")
}

Step "go vet" {
    Run "go" @("vet", "./...")
}

Step "CLI version" {
    $output = & go run ./cmd/go-camoufox version
    if ($LASTEXITCODE -ne 0) {
        throw "go-camoufox version failed"
    }
    if ($output.Trim() -ne $Version) {
        throw "expected version $Version, got $output"
    }
    Write-Host "version: $output"
}

Step "fetch list smoke" {
    Run "go" @("run", "./cmd/go-camoufox", "fetch", "--list")
}

Step "cross-platform builds" {
    $dist = Join-Path $root "dist"
    New-Item -ItemType Directory -Force -Path $dist | Out-Null
    $targets = @(
        @{ GOOS = "linux"; GOARCH = "amd64"; Name = "linux-amd64" },
        @{ GOOS = "linux"; GOARCH = "arm64"; Name = "linux-arm64" },
        @{ GOOS = "darwin"; GOARCH = "amd64"; Name = "darwin-amd64" },
        @{ GOOS = "darwin"; GOARCH = "arm64"; Name = "darwin-arm64" },
        @{ GOOS = "windows"; GOARCH = "amd64"; Name = "windows-amd64.exe" }
    )
    foreach ($target in $targets) {
        $env:CGO_ENABLED = "0"
        $env:GOOS = $target.GOOS
        $env:GOARCH = $target.GOARCH
        Run "go" @(
            "build",
            "-trimpath",
            "-ldflags=-s -w -X main.version=$Version",
            "-o",
            (Join-Path $dist "go-camoufox-$($target.Name)"),
            "./cmd/go-camoufox"
        )
    }
    Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
}

if ($Live) {
    Step "live BiDi smoke" {
        if (-not $Executable) {
            throw "set -Executable or GO_CAMOUFOX_EXECUTABLE for -Live"
        }
        $env:GO_CAMOUFOX_LIVE = "1"
        $env:GO_CAMOUFOX_EXECUTABLE = $Executable
        Run "go" @("test", "./protocol/bidi", "-run", "TestLiveCamoufoxBiDiSmoke", "-v")
        Remove-Item Env:\GO_CAMOUFOX_LIVE -ErrorAction SilentlyContinue
    }
}

if (-not $SkipDockerBuild) {
    Step "docker build" {
        Run "docker" @("build", "-t", $DockerTag, ".")
    }
}

Step "docker compose config" {
    Run "docker" @("compose", "config")
}

Write-Host "`nRelease check passed." -ForegroundColor Green
