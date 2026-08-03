@echo off
chcp 65001 >nul
rem Сборка bastion_zayavki.py в один exe. Запускать в папке со скриптом.

echo.
echo === Сборка "Бастион-2 - пакетное создание заявок" ===
echo.

where python >nul 2>&1
if errorlevel 1 (
    echo Python не найден. Установи с python.org и поставь галочку
    echo "Add python.exe to PATH", затем запусти сборку заново.
    pause
    exit /b 1
)

echo [1/3] Проверяю PyInstaller...
python -m PyInstaller --version >nul 2>&1
if errorlevel 1 (
    echo      не установлен, ставлю...
    python -m pip install --upgrade pyinstaller || goto :oshibka
)

echo [2/3] Собираю...
python -m PyInstaller ^
    --onefile ^
    --windowed ^
    --name "Заявки Бастион" ^
    --clean ^
    --noconfirm ^
    bastion_zayavki.py || goto :oshibka

echo [3/3] Убираю временные файлы...
if exist build rmdir /s /q build
if exist "Заявки Бастион.spec" del /q "Заявки Бастион.spec"

echo.
echo Готово. Файл лежит здесь:
echo     %CD%\dist\Заявки Бастион.exe
echo.
echo Его можно копировать на любой компьютер с Windows,
echo Python там уже не нужен.
echo.
pause
exit /b 0

:oshibka
echo.
echo Сборка не удалась - смотри сообщение выше.
pause
exit /b 1
