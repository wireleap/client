@echo off
for %%f in (%WIRELEAP_HOME%\scripts\*.bat %WIRELEAP_HOME%\scripts\default\*.bat) do echo %%~nf | sort /unique
