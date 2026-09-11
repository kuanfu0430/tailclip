@echo off
setlocal
if not exist "%~dp0TailBlink.exe" (
  echo TailBlink.exe was not found. Extract the complete ZIP before starting.
  pause
  exit /b 1
)
start "" "%~dp0TailBlink.exe"
