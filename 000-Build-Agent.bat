@echo off
setlocal EnableDelayedExpansion
for /F "tokens=1,2 delims=#" %%a in ('"prompt #$H#$E# & echo on & for %%b in (1) do rem"') do (set "ESC=%%b")
set "VIOL=!ESC![38;5;129m"
set "RST=!ESC![0m"
cd /d "%~dp0"
git config --local user.name "alexandreaj"
git config --local user.email "5599410+alexandreaj@users.noreply.github.com"

:: Read current version
for /f "delims=" %%v in (agent\VERSION) do set AGENT_VER=%%v

cls
echo.
echo  =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
echo      Build  -  Oblireach Agent
echo  =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
echo   Agent  : v!AGENT_VER!
echo  -------------------------------------------------
echo.
echo  NOTE: CGO required. Must run on Windows with MinGW-w64.
echo        macOS agent must be built on macOS.
echo.

:: ─────────────────────────────────────────────────────────────────────────────
::  PHASE 1 - BRANCH + BUMP
:: ─────────────────────────────────────────────────────────────────────────────

echo    [1]  dev
echo    [2]  main
echo.
set /p _T=  Branch (1/2):
if "!_T!"=="1" set BRANCH=dev
if "!_T!"=="2" set BRANCH=main
if not defined BRANCH ( echo. & echo  Choix invalide. & endlocal & pause & exit /b 1 )
set BRANCH=!BRANCH: =!
echo.

echo  -------------------------------------------------
echo  [V] Bump Agent  (current: v!AGENT_VER!)
echo.
set /p _Q=  Bump? (Y/n):
if /i "!_Q!"=="n" ( set AGENT_NEW=!AGENT_VER! & goto :CONFIRM )
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
for /f "delims=" %%n in ('node -e "const v='!AGENT_VER!'.split('.').map(Number),b='!_B!';let [M,m,p]=v;if(b==='major'){M++;m=0;p=0;}else if(b==='minor'){m++;p=0;}else{p++;}process.stdout.write([M,m,p].join('.'))"') do set AGENT_NEW=%%n
echo.
echo    !AGENT_VER!  ->  !AGENT_NEW!

:CONFIRM
echo.
echo  -------------------------------------------------
echo  Recapitulatif [!BRANCH!] :
echo.
echo    Agent  : v!AGENT_VER!  ->  v!AGENT_NEW!   [Windows amd64]
echo.
set /p _Q=  Continuer ? (Y/n):
if /i "!_Q!"=="n" ( echo. & echo  Annule. & endlocal & pause & exit /b 0 )

:: ─────────────────────────────────────────────────────────────────────────────
::  PHASE 2 - BUMP VERSION FILE
:: ─────────────────────────────────────────────────────────────────────────────
echo.
node -e "require('fs').writeFileSync('./agent/VERSION','!AGENT_NEW!\n')"
set AGENT_VER=!AGENT_NEW!
echo  Version: !AGENT_VER!  OK

:: ─────────────────────────────────────────────────────────────────────────────
::  PHASE 3 - BUILD  (CGO required, Windows only)
:: ─────────────────────────────────────────────────────────────────────────────
echo.
echo  -------------------------------------------------
echo  Builds  (v!AGENT_VER!)
echo  -------------------------------------------------

pushd agent
if not exist dist\ mkdir dist

echo.
echo  [1/2] go mod tidy...
call go mod tidy
if !ERRORLEVEL! neq 0 ( popd & echo !VIOL! ECHEC : go mod tidy.!RST! & endlocal & pause & exit /b 1 )

set LDFLAGS=-s -w -X main.agentVersion=!AGENT_VER!

echo  [2/2] Windows amd64 (CGO)...
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64
call go build -ldflags="!LDFLAGS!" -o dist\oblireach-agent.exe .
if !ERRORLEVEL! neq 0 ( popd & echo !VIOL! ECHEC : build Windows.!RST! & endlocal & pause & exit /b 1 )
echo    OK: dist\oblireach-agent.exe

popd

:: ─────────────────────────────────────────────────────────────────────────────
::  PHASE 3b - COPY TO OBLIANCE DIST
:: ─────────────────────────────────────────────────────────────────────────────
echo.
echo  -------------------------------------------------
echo  Copie vers Obliance agent\dist\
echo  -------------------------------------------------
set OBLIANCE_DIST=..\Obliance\agent\dist

if not exist "!OBLIANCE_DIST!\" (
  echo  AVERTISSEMENT: !OBLIANCE_DIST! introuvable - copie ignoree.
) else (
  copy /Y "agent\dist\oblireach-agent.exe" "!OBLIANCE_DIST!\oblireach-agent.exe" >nul
  if !ERRORLEVEL! neq 0 ( echo !VIOL! ECHEC : copie .exe vers Obliance.!RST! ) else (
    echo    OK: !OBLIANCE_DIST!\oblireach-agent.exe
  )
)

echo.
echo  NOTE: macOS agent must be built on macOS:
echo    cd agent ^&^& CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.agentVersion=!AGENT_VER!" -o dist/oblireach-agent-darwin-arm64 .
echo    cd agent ^&^& CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.agentVersion=!AGENT_VER!" -o dist/oblireach-agent-darwin-amd64 .

:: ─────────────────────────────────────────────────────────────────────────────
::  PHASE 4 - GIT COMMIT + PUSH
:: ─────────────────────────────────────────────────────────────────────────────
echo.
echo  -------------------------------------------------
echo  [GIT] Commit + push -> origin/!BRANCH!
echo  -------------------------------------------------
git add -A
git commit -m "Agent v!AGENT_VER! [!BRANCH!]"
if !ERRORLEVEL! neq 0 (
  echo  Rien de nouveau a committer.
) else (
  git push origin HEAD:!BRANCH!
  if !ERRORLEVEL! neq 0 ( echo !VIOL! ECHEC : git push.!RST! )
)

echo.
echo  =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
echo   Done  -  Agent v!AGENT_VER!  [!BRANCH!]
echo  =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=
echo.
endlocal
pause
exit /b 0
