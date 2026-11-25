!include "getProcessInfo.nsh"
Var pid

!ifndef BUILD_UNINSTALLER
  Var installLock

  !macro openInstallLock FILE
    System::Call 'kernel32::CreateFileW(w "${FILE}", i 0x40000000, i 0, p 0, i 2, i 0x04000000, p 0) p .s'
    Pop $installLock
  !macroend

  !macro takeInstallLock FILE
    ${if} $installLock == ""
    ${orIf} $installLock == -1
      !insertmacro openInstallLock "${FILE}"
      ${if} $installLock == -1
        SetFileAttributes "${FILE}" NORMAL
        Delete "${FILE}"
        !insertmacro openInstallLock "${FILE}"
      ${endIf}
    ${endIf}
  !macroend

  !macro customUnInstallCheck
    ${if} ${Errors}
      DetailPrint `Uninstall was not successful. Not able to launch uninstaller!`
    ${elseIf} $R0 != 0
      MessageBox MB_OK|MB_ICONEXCLAMATION "$(uninstallFailed): $R0"
      DetailPrint `Uninstall was not successful. Uninstaller error code: $R0.`
      SetErrorLevel 2
      Quit
    ${endIf}
    !insertmacro takeInstallLock "$INSTDIR.update-installing.lock"
    ${if} $installLock == -1
      CreateDirectory "$INSTDIR"
      !insertmacro takeInstallLock "$INSTDIR\update-installing.lock"
    ${endIf}
    ${if} $installLock == -1
      DetailPrint `Install lock not taken: Studio is not held back while files are replaced.`
    ${endIf}
  !macroend
!endif

# Silent `--updated` without `--force-run` (Studio through 2.4.30) skips the finish-page Run checkbox.
# Do not !insertmacro StartApp: that redeclares Var startAppArgs when installSection expands doStartApp.
!macro customInstall
  ${if} $installLock != ""
  ${andIf} $installLock != -1
    System::Call 'kernel32::CloseHandle(p $installLock)'
    StrCpy $installLock ""
  ${endIf}
  ${if} ${isUpdated}
  ${andIf} ${Silent}
    ${ifNot} ${isForceRun}
      HideWindow
      ${StdUtils.ExecShellAsUser} $0 "$launchLink" "open" "--updated"
    ${endIf}
  ${endIf}
!macroend

!macro customCheckAppRunning
  !ifndef BUILD_UNINSTALLER
    !insertmacro takeInstallLock "$INSTDIR.update-installing.lock"
  !endif
  !insertmacro IS_POWERSHELL_AVAILABLE
  !insertmacro _CHECK_APP_RUNNING
!macroend
