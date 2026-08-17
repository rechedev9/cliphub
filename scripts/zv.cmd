@echo off
REM Forwards to repo-local zv.exe. Does not stop pwsh from eating "--".
REM For workflows run from pwsh use: cmd.exe /c "scripts\zv.cmd workflows run short -- ..."
REM or: .\bin\zv.exe --% workflows run short -- ...
setlocal
"%~dp0..\bin\zv.exe" %*
exit /b %ERRORLEVEL%
