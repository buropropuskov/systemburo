@echo off
rem ASCII only - cmd.exe reads .bat in OEM codepage and mangles Cyrillic.
setlocal

echo.
echo === Building "Bastion-2 batch requests" ===
echo.

rem "py" first: the bare "python" name may be hijacked by the Microsoft Store
rem execution alias, which passes "where python" but fails on any real run.
set "PY="

py -3 -c "import sys" >nul 2>&1
if not errorlevel 1 set "PY=py -3"
if defined PY goto :found

python -c "import sys" >nul 2>&1
if not errorlevel 1 set "PY=python"
if defined PY goto :found

python3 -c "import sys" >nul 2>&1
if not errorlevel 1 set "PY=python3"
if defined PY goto :found

goto :nopython

:found
echo [1/4] Interpreter: %PY%
for /f "delims=" %%v in ('%PY% -c "import sys; print(sys.version.split()[0])"') do echo       version %%v

echo [2/4] Checking PyInstaller...
%PY% -m PyInstaller --version >nul 2>&1
if errorlevel 1 (
    echo       not installed, installing...
    %PY% -m pip install --upgrade pyinstaller
    if errorlevel 1 goto :failed
)

echo [3/4] Building...
%PY% -m PyInstaller --onefile --windowed --clean --noconfirm --name BastionZayavki bastion_zayavki.py
if errorlevel 1 goto :failed

echo [4/4] Cleaning up...
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

:nopython
echo No working Python found.
echo.
echo If Python IS installed, Windows is most likely intercepting the name
echo with a Microsoft Store stub. Turn it off here:
echo     Settings ^> Apps ^> Advanced app settings ^> App execution aliases
echo     switch OFF both "python.exe" and "python3.exe"
echo.
echo Otherwise install Python from python.org and tick
echo "Add python.exe to PATH" during setup.
echo.
pause
exit /b 1

:failed
echo.
echo Build failed - see the message above.
echo.
pause
exit /b 1
