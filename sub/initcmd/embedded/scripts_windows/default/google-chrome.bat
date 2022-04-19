@echo off

set bin64=%ProgramFiles%\Google\Chrome\Application\chrome.exe
set bin32=%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe

set bin="%bin64%"
if not exist "%bin64%" (
    if not exist "%bin32%" (
        echo "The executable file [%bin%] does not exist."
        exit /b 1
    )
    set bin="%bin32%"
)

%bin% --proxy-server="socks5://%WIRELEAP_SOCKS%" --host-resolver-rules="MAP * ~NOTFOUND, EXCLUDE 127.0.0.1" --user-data-dir="%LOCALAPPDATA%\Google\Chrome\chrome-wireleap" --incognito %*
