!include "getProcessInfo.nsh"
Var pid

!ifndef BUILD_UNINSTALLER
  Var installLock

  !macro openInstallLock
    System::Call 'kernel32::CreateFileW(w "$INSTDIR.update-installing.lock", i 0x40000000, i 0, p 0, i 2, i 0x04000000, p 0) p .s'
    Pop $installLock
  !macroend

  !macro takeInstallLock
    ${if} $installLock == ""
    ${orIf} $installLock == -1
      !insertmacro openInstallLock
      ${if} $installLock == -1
        SetFileAttributes "$INSTDIR.update-installing.lock" NORMAL
        Delete "$INSTDIR.update-installing.lock"
        !insertmacro openInstallLock
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
    !insertmacro takeInstallLock
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
    !insertmacro takeInstallLock
  !endif
  !insertmacro IS_POWERSHELL_AVAILABLE
  !insertmacro _CHECK_APP_RUNNING
!macroend
