@echo off

set bin64=%ProgramFiles%\Google\Chrome\Application\chrome.exe
set bin32=%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe

if not exist "%bin64%" (
    if not exist "%bin32%" (
        echo "The executable file [%bin%] does not exist."
        exit /b 1
    )
    set bin="%bin32%"
)
set bin="%bin64%"

%bin% --proxy-server="socks5://%WIRELEAP_SOCKS%" --user-data-dir="%LOCALAPPDATA%\Google\Chrome\chrome-wireleap" --incognito %*
