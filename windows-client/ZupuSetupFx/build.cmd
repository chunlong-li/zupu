@echo off
rem 编译「族谱客户端安装包」(.NET Framework 版)
rem 依赖系统自带的 csc.exe，不需要装 .NET SDK
rem 产物: out\族谱客户端安装包.exe （约 52 KB）

setlocal
set CSC=%SystemRoot%\Microsoft.NET\Framework64\v4.0.30319\csc.exe
if not exist "%CSC%" (
  echo 找不到 csc.exe: %CSC%
  echo 需要 .NET Framework 4.x
  exit /b 1
)

if not exist out mkdir out

"%CSC%" /nologo /codepage:65001 /target:winexe /optimize+ ^
  /out:out\族谱客户端安装包.exe ^
  /win32icon:..\windows-client\app.ico ^
  /r:System.dll /r:System.Core.dll /r:System.Windows.Forms.dll /r:System.Drawing.dll ^
  /r:System.IO.Compression.dll /r:System.IO.Compression.FileSystem.dll /r:System.Security.dll ^
  Program.cs

if errorlevel 1 exit /b 1
echo 完成: out\族谱客户端安装包.exe
