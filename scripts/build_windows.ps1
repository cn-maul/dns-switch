# build_windows.ps1 — DNS-Switch Windows 一键构建脚本
$ErrorActionPreference = "Stop"
Set-Location (Split-Path -Parent $PSScriptRoot)

Write-Host "[1/4] 检查 Go 版本..." -ForegroundColor Cyan
go version

Write-Host "[2/4] 安装 rsrc 工具..." -ForegroundColor Cyan
go install github.com/akavel/rsrc@latest

Write-Host "[3/4] 生成 rsrc.syso (UAC 清单)..." -ForegroundColor Cyan
rsrc -manifest dns-switch.exe.manifest -o rsrc.syso

Write-Host "[4/4] 编译..." -ForegroundColor Cyan
go build -ldflags "-s -w" -o dns-switch.exe

Write-Host "构建完成: dns-switch.exe" -ForegroundColor Green
