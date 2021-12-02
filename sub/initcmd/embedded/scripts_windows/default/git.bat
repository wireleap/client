@echo off
git -c http.proxy=socks5h://%WIRELEAP_SOCKS% %*
