# 族谱客户端 · 安装程序（免管理员，装到当前用户，创建桌面快捷方式）
$ErrorActionPreference = 'Stop'
$src = Split-Path -Parent $MyInvocation.MyCommand.Path
$dst = Join-Path $env:LOCALAPPDATA 'ZupuClient'
$exe = Join-Path $dst 'ZupuClient.exe'

Write-Host "正在安装到 $dst ..." -ForegroundColor Cyan
New-Item -ItemType Directory -Force -Path $dst | Out-Null
Copy-Item -Path (Join-Path $src 'app\*') -Destination $dst -Recurse -Force

# 桌面快捷方式
$desktop = [Environment]::GetFolderPath('Desktop')
$lnk = Join-Path $desktop '族谱客户端.lnk'
$sh = New-Object -ComObject WScript.Shell
$s = $sh.CreateShortcut($lnk)
$s.TargetPath = $exe
$s.WorkingDirectory = $dst
$s.IconLocation = $exe
$s.Description = '族谱客户端'
$s.Save()

# 开始菜单（可选）
$startMenu = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs'
Copy-Item $lnk (Join-Path $startMenu '族谱客户端.lnk') -Force -ErrorAction SilentlyContinue

# 卸载项
$uninst = Join-Path $dst '卸载.ps1'
@"
Remove-Item -Recurse -Force '$dst'
Remove-Item -Force '$lnk' -ErrorAction SilentlyContinue
Remove-Item -Force '$startMenu\族谱客户端.lnk' -ErrorAction SilentlyContinue
Write-Host '已卸载族谱客户端' -ForegroundColor Green
"@ | Set-Content -Path $uninst -Encoding UTF8

Write-Host "✅ 安装完成：桌面已创建「族谱客户端」快捷方式" -ForegroundColor Green
Start-Process $exe
