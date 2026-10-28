@echo off
cd /d "%~dp0"

:: Find Go
if exist "C:\HACKATHONS\tools\go\bin\go.exe" (
    set "GOROOT=C:\HACKATHONS\tools\go"
    set "PATH=%GOROOT%\bin;%PATH%"
) else if exist "%ProgramFiles%\Go\bin\go.exe" (
    set "GOROOT=%ProgramFiles%\Go"
    set "PATH=%GOROOT%\bin;%PATH%"
)

:: Build if binary doesn't exist
if not exist "odin.exe" (
    echo Building ODIN...
    go build -o odin.exe .
    if errorlevel 1 (
        echo Build failed. Make sure Go 1.22+ is installed.
        pause
        exit /b 1
    )
)

set "ODIN_DB_PATH=%~dp0odin.db"
set "PORT=8081"
set "ODIN_OPEN_BROWSER=true"

echo Starting ODIN on http://localhost:%PORT% ...
start "" "%~dp0odin.exe"
timeout /t 3 /nobreak >nul
start "" "http://localhost:%PORT%"

echo ODIN is running. Close this window to stop.
pause
