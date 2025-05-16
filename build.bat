@echo off
echo Building TCP Chat Application with Go modules...

cd server
go build -o chat-server.exe
if %ERRORLEVEL% neq 0 (
    echo Error building server!
    exit /b %ERRORLEVEL%
)
echo Server build successful!

cd ..\client
go build -o chat-client.exe
if %ERRORLEVEL% neq 0 (
    echo Error building client!
    exit /b %ERRORLEVEL%
)
echo Client build successful!

echo Build process complete! You can now run:
echo   server\chat-server.exe - to start the server
echo   client\chat-client.exe - to start the client
