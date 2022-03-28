@echo off

set bin64=%ProgramFiles%\Mozilla Firefox\firefox.exe
set bin32=%ProgramFiles(x86)%\Mozilla Firefox\firefox.exe

set bin="%bin64%"
if not exist "%bin64%" (
    if not exist "%bin32%" (
        echo "The executable file [%bin32%] does not exist."
        exit /b 1
    )
    set bin="%bin32%"
)

set profile=%LOCALAPPDATA%/Mozilla/Firefox/wireleap
md "%profile%"

> "%profile%/prefs.js" (
    echo.user_pref("network.proxy.socks", "%WIRELEAP_SOCKS_HOST%"^);
    echo.user_pref("network.proxy.socks_port", %WIRELEAP_SOCKS_PORT%^);
    echo.user_pref("network.proxy.socks_remote_dns", true^);
    echo.user_pref("network.proxy.type", 1^);
)

%bin% --profile "%profile%" --new-instance --private-window %*
