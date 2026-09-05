@echo off
setlocal
if not exist "%~dp0TailClip.exe" (
  echo TailClip.exe was not found. Extract the complete ZIP before starting.
  pause
  exit /b 1
)
start "" "%~dp0TailClip.exe"
