# Silent `--updated` without `--force-run` (Studio through 2.4.30) skips the finish-page Run checkbox.
# Do not !insertmacro StartApp: that redeclares Var startAppArgs when installSection expands doStartApp.
!macro customInstall
  ${if} ${isUpdated}
  ${andIf} ${Silent}
    ${ifNot} ${isForceRun}
      HideWindow
      ${StdUtils.ExecShellAsUser} $0 "$launchLink" "open" "--updated"
    ${endIf}
  ${endIf}
!macroend
