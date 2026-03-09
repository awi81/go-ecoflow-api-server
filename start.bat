
@echo off
echo ============================================
echo EcoFlow Smart Charging - Starter
echo ============================================
echo.

REM Load environment variables from .env file if it exists
if exist "%~dp0.env" (
    echo Loading environment from .env file...
    for /f "usebackq tokens=*" %%a in ("%~dp0.env") do set "%%a"
)

REM Check if TIBBER_API_KEY is set
if "%TIBBER_API_KEY%"=="" (
    echo WARNING: TIBBER_API_KEY not set!
    echo Please create a .env file with TIBBER_API_KEY=your_token
    echo Or set the environment variable before running this script.
    echo.
)

REM Check if Docker is running
echo [1/5] Checking Docker...
docker info >nul 2>&1
if errorlevel 1 (
    echo ERROR: Docker is not running. Please start Docker Desktop.
    pause
    exit /b 1
)
echo OK - Docker is running
echo.

REM Check if image exists, build if not
echo [2/5] Checking Docker image...
docker image inspect ecoflow-server >nul 2>&1
if errorlevel 1 (
    echo Building Docker image...
    docker build -t ecoflow-server .
    if errorlevel 1 (
        echo ERROR: Failed to build Docker image.
        pause
        exit /b 1
    )
    echo OK - Docker image built
) else (
    echo OK - Docker image already exists
)
echo.

REM Stop existing container if running
echo [3/5] Stopping existing container...
docker stop ecoflow-server >nul 2>&1
docker rm ecoflow-server >nul 2>&1
echo OK
echo.

REM Start Backend (Docker)
echo [4/5] Starting Backend (EcoFlow API)...
if "%TIBBER_API_KEY%"=="" (
    start "EcoFlow API" docker run -d -p 8080:8080 --name ecoflow-server ecoflow-server
) else (
    start "EcoFlow API" docker run -d -p 8080:8080 -e TIBBER_API_KEY=%TIBBER_API_KEY% --name ecoflow-server ecoflow-server
)
echo OK - Backend starting on http://localhost:8080
echo.

REM Wait a bit for backend to start
timeout /t 3 /nobreak >nul

REM Start Frontend (if directory exists)
echo [5/5] Starting Frontend...
if exist "%~dp0ecoflow-smart-charging" (
    cd /d "%~dp0ecoflow-smart-charging"
    start "Frontend" cmd /c "npm run dev"
    echo OK - Frontend starting on http://localhost:3000
) else (
    echo SKIP - Frontend directory not found (ecoflow-smart-charging)
)
echo.

echo ============================================
echo Services are starting!
echo.
echo Backend:  http://localhost:8080
echo Frontend: http://localhost:3000 ^(if installed^)
echo.
echo Press any key to exit this window...
echo ============================================
pause >nul
