package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/rechedev9/cliphub/internal/winproc"
)

// hookErrorWindowTitlePattern matches the MessageBox titles of advancedfx's hook modules when a signature
// scan fails, almost always because a CS2 update landed after the installed HLAE build.
var hookErrorWindowTitlePattern = regexp.MustCompile(`^Error - Afx`)

// isHookErrorWindowTitle reports whether title is a native HLAE hook crash
// dialog, e.g. "Error - AfxHookSource2".
func isHookErrorWindowTitle(title string) bool {
	return hookErrorWindowTitlePattern.MatchString(title)
}

// hookIncompatibleError reports that HLAE's injected hook crashed with a native error dialog,
// which means the installed HLAE build does not match the installed CS2.
type hookIncompatibleError struct {
	windowTitle string
}

func (e *hookIncompatibleError) Error() string {
	return fmt.Sprintf(
		"HLAE hook crashed with a native error dialog (%q) instead of capturing: "+
			"the installed HLAE/AfxHookSource2 build is likely incompatible with the current CS2 version "+
			"(CS2 updates regularly break AfxHookSource2's signature scan until advancedfx ships a new build); "+
			"check https://github.com/advancedfx/advancedfx/releases for a newer HLAE build",
		e.windowTitle,
	)
}

func processWindowTitle(image string) (running bool, title string, err error) {
	running, titles, err := winproc.WindowTitles(image)
	if err != nil {
		return running, "", err
	}
	return running, judgedWindowTitle(titles), nil
}

// judgedWindowTitle returns the title of a hook-crash or launcher error dialog when one is open.
func judgedWindowTitle(titles []string) string {
	for _, candidate := range titles {
		if isErrorDialogTitle(candidate) {
			return candidate
		}
	}
	return ""
}

func isErrorDialogTitle(title string) bool {
	return isHookErrorWindowTitle(title) || title == launcherErrorWindowTitle
}

// launcherErrorWindowTitle is the MessageBox CS2's launcher opens when it cannot start,
// for example when the Steamworks SDK cannot reach the signed-in Steam client.
const launcherErrorWindowTitle = "Launcher Error"

// steamNotRunningPrefix is the stable reason the orchestrator keeps when CS2 cannot reach Steam.
const steamNotRunningPrefix = "steam_not_running:"

// readDialogText reads a dialog's body; tests replace it.
var readDialogText = func(image, title string) string {
	text, err := winproc.DialogText(image, title)
	if err != nil || text == "" {
		log.Printf("read %q dialog of %s: text %q, err %v", title, image, text, err)
	}
	return text
}

// launcherDialogError reports a CS2 that never started because its launcher showed an error.
type launcherDialogError struct {
	text string
}

func (e *launcherDialogError) Error() string {
	detail := fmt.Sprintf("CS2 showed %q before starting the demo: %s", launcherErrorWindowTitle, e.text)
	if namesSteamClient(e.text) {
		return steamNotRunningPrefix + " " + detail
	}
	return "cs2_launcher_error: " + detail
}

// namesSteamClient matches the launcher's Steam messages, not a module path under Steam\steamapps.
func namesSteamClient(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "steamworks") || strings.Contains(lower, "steam client") ||
		strings.Contains(lower, "steam must be running") || strings.Contains(lower, "connecttoglobaluser")
}

// dialogFailure turns an open error dialog of image into the capture's failure, or nil.
func dialogFailure(image, title string) error {
	if isHookErrorWindowTitle(title) {
		return &hookIncompatibleError{windowTitle: title}
	}
	if title == launcherErrorWindowTitle {
		return &launcherDialogError{text: readDialogText(image, title)}
	}
	return nil
}

// requireSteamClient refuses to launch CS2 without Steam, which would only end in a Steamworks error.
func requireSteamClient() error {
	running, err := processRunning("steam.exe")
	if err != nil || running {
		return nil
	}
	return fmt.Errorf("%s steam.exe is not running; open Steam and sign in before capturing", steamNotRunningPrefix)
}
