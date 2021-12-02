@echo off
set bin=%ProgramFiles%\Mozilla Firefox\firefox.exe

if not exist "%bin%" (
    echo "The executable file [%bin%] does not exist."
    exit /b 1
)

set profile=%LOCALAPPDATA%/Mozilla/Firefox/wireleap
md "%profile%"

> "%profile%/prefs.js" (
    echo.user_pref("network.proxy.socks", "%WIRELEAP_SOCKS_HOST%"^);
    echo.user_pref("network.proxy.socks_port", %WIRELEAP_SOCKS_PORT%^);
    echo.user_pref("network.proxy.socks_remote_dns", true^);
    echo.user_pref("network.proxy.type", 1^);
)

"%bin%" --profile "%profile%" --new-instance --private-window %*
