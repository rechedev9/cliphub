# Silent `--updated` without `--force-run` (Studio through 2.4.30) skips the finish-page Run checkbox.
# Relaunch here unless the running app already passed `--force-run`.
!macro customInstall
  ${if} ${isUpdated}
  ${andIf} ${Silent}
    ${ifNot} ${isForceRun}
      HideWindow
      !insertmacro StartApp
    ${endIf}
  ${endIf}
!macroend
