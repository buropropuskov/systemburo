@echo off
rem ASCII only - cmd.exe reads .bat in OEM codepage and mangles Cyrillic.
setlocal

echo.
echo === Building "Bastion-2 batch requests" ===
echo.

where python >nul 2>&1
if errorlevel 1 (
    echo Python not found.
    echo Install it from python.org and tick "Add python.exe to PATH",
    echo then close this window, open a new one and run the build again.
    echo.
    pause
    exit /b 1
)

echo [1/3] Checking PyInstaller...
python -m PyInstaller --version >nul 2>&1
if errorlevel 1 (
    echo       not installed, installing...
    python -m pip install --upgrade pyinstaller
    if errorlevel 1 goto :failed
)

echo [2/3] Building...
python -m PyInstaller --onefile --windowed --clean --noconfirm --name BastionZayavki bastion_zayavki.py
if errorlevel 1 goto :failed

echo [3/3] Cleaning up...
if exist build rmdir /s /q build
if exist BastionZayavki.spec del /q BastionZayavki.spec

echo.
echo Done. The program is here:
echo     %CD%\dist\BastionZayavki.exe
echo.
echo Copy it to any Windows machine - Python is not needed there.
echo.
pause
exit /b 0

:failed
echo.
echo Build failed - see the message above.
echo.
pause
exit /b 1
