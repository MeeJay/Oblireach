@echo off
setlocal EnableDelayedExpansion
for /F "tokens=1,2 delims=#" %%a in ('"prompt #$H#$E# & echo on & for %%b in (1) do rem"') do (set "ESC=%%b")
set "VIOL=!ESC![38;5;129m"
set "RST=!ESC![0m"
cd /d "%~dp0"
git config --local user.name "alexandreaj"
git config --local user.email "5599410+alexandreaj@users.noreply.github.com"

:: Read current version
for /f "delims=" %%v in (server\VERSION) do set RELAY_VER=%%v

cls
echo.
echo  =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
echo      Build  -  Oblireach Relay
echo  =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
echo   Relay  : v!RELAY_VER!
echo  -------------------------------------------------
echo.

:: ─────────────────────────────────────────────────────────────────────────────
::  PHASE 1 - BRANCH + BUMP
:: ─────────────────────────────────────────────────────────────────────────────

:: Branch
echo    [1]  dev
echo    [2]  main
echo.
set /p _T=  Branch (1/2):
if "!_T!"=="1" set BRANCH=dev
if "!_T!"=="2" set BRANCH=main
if not defined BRANCH ( echo. & echo  Choix invalide. & endlocal & pause & exit /b 1 )
set BRANCH=!BRANCH: =!
echo.

:: Bump?
echo  -------------------------------------------------
echo  [V] Bump Relay  (current: v!RELAY_VER!)
echo.
set /p _Q=  Bump? (Y/n):
if /i "!_Q!"=="n" ( set RELAY_NEW=!RELAY_VER! & goto :CONFIRM )
echo.
echo    [1]  Patch   +0.0.1  (defaut)
echo    [2]  Minor   +0.1.0
echo    [3]  Major   +1.0.0
echo.
set _B=
set /p _B=  Type (1/2/3, Entree=patch):
if "!_B!"==""  set _B=1
if "!_B!"=="1" set _B=patch
if "!_B!"=="2" set _B=minor
if "!_B!"=="3" set _B=major
if not "!_B!"=="patch" if not "!_B!"=="minor" if not "!_B!"=="major" (
  echo  Choix invalide, patch applique par defaut.
  set _B=patch
)
for /f "delims=" %%n in ('node -e "const v='!RELAY_VER!'.split('.').map(Number),b='!_B!';let [M,m,p]=v;if(b==='major'){M++;m=0;p=0;}else if(b==='minor'){m++;p=0;}else{p++;}process.stdout.write([M,m,p].join('.'))"') do set RELAY_NEW=%%n
echo.
echo    !RELAY_VER!  ->  !RELAY_NEW!

:CONFIRM
echo.
echo  -------------------------------------------------
echo  Recapitulatif [!BRANCH!] :
echo.
echo    Relay  : v!RELAY_VER!  ->  v!RELAY_NEW!   [Windows + Linux + macOS]
echo.
set /p _Q=  Continuer ? (Y/n):
if /i "!_Q!"=="n" ( echo. & echo  Annule. & endlocal & pause & exit /b 0 )

:: ─────────────────────────────────────────────────────────────────────────────
::  PHASE 2 - BUMP VERSION FILE
:: ─────────────────────────────────────────────────────────────────────────────
echo.
node -e "require('fs').writeFileSync('./server/VERSION','!RELAY_NEW!\n')"
set RELAY_VER=!RELAY_NEW!
echo  Version: !RELAY_VER!  OK

:: ─────────────────────────────────────────────────────────────────────────────
::  PHASE 3 - BUILDS  (pure Go = cross-compile directement, pas de Docker)
:: ─────────────────────────────────────────────────────────────────────────────
echo.
echo  -------------------------------------------------
echo  Builds  (v!RELAY_VER!)
echo  -------------------------------------------------

pushd server
if not exist dist\ mkdir dist

echo.
echo  [1/5] go mod tidy...
call go mod tidy
if !ERRORLEVEL! neq 0 ( popd & echo !VIOL! ECHEC : go mod tidy.!RST! & endlocal & pause & exit /b 1 )

set LDFLAGS=-s -w -X main.relayVersion=!RELAY_VER!

echo  [2/5] Windows amd64...
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
call go build -ldflags="!LDFLAGS!" -o dist\oblireach-relay.exe .
if !ERRORLEVEL! neq 0 ( popd & echo !VIOL! ECHEC : build Windows.!RST! & endlocal & pause & exit /b 1 )
echo    OK: dist\oblireach-relay.exe

echo  [3/5] Linux amd64...
set GOOS=linux
set GOARCH=amd64
call go build -ldflags="!LDFLAGS!" -o dist\oblireach-relay-linux-amd64 .
if !ERRORLEVEL! neq 0 ( popd & echo !VIOL! ECHEC : build Linux amd64.!RST! & endlocal & pause & exit /b 1 )
echo    OK: dist\oblireach-relay-linux-amd64

echo  [4/5] macOS arm64...
set GOOS=darwin
set GOARCH=arm64
call go build -ldflags="!LDFLAGS!" -o dist\oblireach-relay-darwin-arm64 .
if !ERRORLEVEL! neq 0 ( popd & echo !VIOL! ECHEC : build macOS arm64.!RST! & endlocal & pause & exit /b 1 )
echo    OK: dist\oblireach-relay-darwin-arm64

echo  [5/5] macOS amd64...
set GOOS=darwin
set GOARCH=amd64
call go build -ldflags="!LDFLAGS!" -o dist\oblireach-relay-darwin-amd64 .
if !ERRORLEVEL! neq 0 ( popd & echo !VIOL! ECHEC : build macOS amd64.!RST! & endlocal & pause & exit /b 1 )
echo    OK: dist\oblireach-relay-darwin-amd64

popd

:: ─────────────────────────────────────────────────────────────────────────────
::  PHASE 4 - GIT COMMIT + PUSH
:: ─────────────────────────────────────────────────────────────────────────────
echo.
echo  -------------------------------------------------
echo  [GIT] Commit + push -> origin/!BRANCH!
echo  -------------------------------------------------
git add -A
git commit -m "Relay v!RELAY_VER! [!BRANCH!]"
if !ERRORLEVEL! neq 0 (
  echo  Rien de nouveau a committer.
) else (
  git push origin HEAD:!BRANCH!
  if !ERRORLEVEL! neq 0 ( echo !VIOL! ECHEC : git push.!RST! )
)

echo.
echo  =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
echo   Done  -  Relay v!RELAY_VER!  [!BRANCH!]
echo  =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
echo.
endlocal
pause
exit /b 0
