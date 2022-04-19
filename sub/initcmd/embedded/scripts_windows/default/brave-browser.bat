@echo off

set bin64=%ProgramFiles%\BraveSoftware\Brave-Browser\Application\brave.exe
set bin32=%ProgramFiles(x86)%\BraveSoftware\Brave-Browser\Application\brave.exe

set bin="%bin64%"
if not exist "%bin64%" (
    if not exist "%bin32%" (
        echo "The executable file [%bin32%] does not exist."
        exit /b 1
    )
    set bin="%bin32%"
)

%bin% --host-resolver-rules="MAP * ~NOTFOUND, EXCLUDE 127.0.0.1" --proxy-server="socks5://%WIRELEAP_SOCKS%" --user-data-dir="%LOCALAPPDATA%\BraveSoftware\brave-wireleap" --incognito %*
