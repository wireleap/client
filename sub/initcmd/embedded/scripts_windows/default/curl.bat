@echo off
set ALL_PROXY=socks5h://%WIRELEAP_SOCKS%
curl %*
