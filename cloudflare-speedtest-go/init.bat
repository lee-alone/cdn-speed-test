@echo off
REM Cloudflare Speed Test (Go) - Project Initialization Script for Windows

echo.
echo ==========================================
echo Cloudflare Speed Test (Go) - Initialization
echo ==========================================
echo.

REM Check Go installation
go version >nul 2>&1
if errorlevel 1 (
    echo X Go is not installed. Please install Go 1.21 or higher.
    exit /b 1
)

for /f "tokens=3" %%i in ('go version') do set GO_VERSION=%%i
echo [OK] Go version: %GO_VERSION%
echo.

REM Download dependencies
echo Downloading dependencies...
call go mod download
if errorlevel 1 (
    echo X Failed to download dependencies
    exit /b 1
)
echo [OK] Dependencies downloaded successfully
echo.

REM Tidy dependencies
echo Tidying dependencies...
call go mod tidy
if errorlevel 1 (
    echo X Failed to tidy dependencies
    exit /b 1
)
echo [OK] Dependencies tidied successfully
echo.

REM Build the project
echo Building the project...
call make build
if errorlevel 1 (
    echo X Build failed
    exit /b 1
)
echo [OK] Build successful
echo.

echo ==========================================
echo [OK] Initialization complete!
echo ==========================================
echo.
echo Next steps:
echo 1. Run the application: make run
echo 2. Or run directly: .\bin\cloudflare-speedtest.exe
echo.
echo For more information, see:
echo - QUICKSTART.md - Quick start guide
echo - ARCHITECTURE.md - Project architecture
echo - README.md - Project overview
echo.
pause
