@echo off
setlocal
set "SERVER=%~1"
if "%SERVER%"=="" set "SERVER=127.0.0.1:8080"
set "TOTAL=100"
set "PW=test123"

for /L %%i in (1,1,%TOTAL%) do (
  REM Write two‑line input using the loop var directly
  >input%%i.txt (
    echo /register user%%i %PW%
    echo /exit
  )
  start "" /B chat-client.exe %SERVER% < input%%i.txt > client%%i.log 2>&1
  ping -n 1 127.0.0.1 >nul
)

echo Launched %TOTAL% registrations in background.
pause
