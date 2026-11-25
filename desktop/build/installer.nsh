!include "getProcessInfo.nsh"
Var pid
Var installLock

# Silent `--updated` without `--force-run` (Studio through 2.4.30) skips the finish-page Run checkbox.
# Do not !insertmacro StartApp: that redeclares Var startAppArgs when installSection expands doStartApp.
!macro customInstall
  System::Call 'kernel32::CloseHandle(p $installLock)'
  ${if} ${isUpdated}
  ${andIf} ${Silent}
    ${ifNot} ${isForceRun}
      HideWindow
      ${StdUtils.ExecShellAsUser} $0 "$launchLink" "open" "--updated"
    ${endIf}
  ${endIf}
!macroend

!macro customCheckAppRunning
  ReadEnvStr $R8 APPDATA
  CreateDirectory "$R8\${APP_PACKAGE_NAME}"
  System::Call 'kernel32::CreateFileW(w "$R8\${APP_PACKAGE_NAME}\update-installing.lock", i 0x40000000, i 0, p 0, i 2, i 0x04000000, p 0) p .s'
  Pop $installLock
  !insertmacro IS_POWERSHELL_AVAILABLE
  !insertmacro _CHECK_APP_RUNNING
!macroend
