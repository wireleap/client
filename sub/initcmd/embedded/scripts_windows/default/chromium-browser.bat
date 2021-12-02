@echo off
set bin=%ProgramFiles%\Chromium\Application\chrome.exe

if not exist "%bin%" (
    echo "The executable file [%bin%] does not exist."
    exit /b 1
)

"%bin%" --proxy-server="socks5://%WIRELEAP_SOCKS%" --user-data-dir="%LOCALAPPDATA%\Chromium\chromium-wireleap" --incognito %*
