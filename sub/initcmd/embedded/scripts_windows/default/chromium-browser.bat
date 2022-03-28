@echo off

set bin64=%ProgramFiles%\Chromium\Application\chrome.exe
set bin32=%ProgramFiles(x86)%\Chromium\Application\chrome.exe

set bin="%bin64%"
if not exist "%bin64%" (
    if not exist "%bin32%" (
        echo "The executable file [%bin32%] does not exist."
        exit /b 1
    )
    set bin="%bin32%"
)

%bin% --proxy-server="socks5://%WIRELEAP_SOCKS%" --user-data-dir="%LOCALAPPDATA%\Chromium\chromium-wireleap" --incognito %*
