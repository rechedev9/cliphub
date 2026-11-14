//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func applicationErrorEvent(app, pid, module, code string) string {
	return `<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'><System><EventID>1000</EventID></System><EventData>` +
		`<Data Name='AppName'>` + app + `</Data><Data Name='ModuleName'>` + module + `</Data>` +
		`<Data Name='ModuleVersion'>1.2.3.4</Data><Data Name='ExceptionCode'>` + code + `</Data>` +
		`<Data Name='FaultingOffset'>0000000000197a79</Data><Data Name='ProcessId'>` + pid + `</Data>` +
		`<Data Name='AppPath'>C:\Users\kéko\cs2.exe</Data></EventData></Event>`
}

func TestParseCS2CrashPicksTheEventOfTheCapturedProcess(t *testing.T) {
	events := applicationErrorEvent("NisSrv.exe", "0x6e0", "NisSrv.exe", "c0000409") + "\r\n" +
		applicationErrorEvent("cs2.exe", "0x1a2b", "engine2.dll", "c0000005") + "\r\n" +
		applicationErrorEvent("cs2.exe", "0x6e0", "nvwgf2umx.dll", "c0000409")
	// wevtutil can emit a non-UTF-8 code page; the decoder must still read every event.
	raw := []byte(strings.Replace(events, "é", "\xe9", -1))

	crash, ok := parseCS2Crash(raw, 0x6e0)
	if !ok || crash.module != "nvwgf2umx.dll" || crash.exception != "c0000409" {
		t.Fatalf("crash = %+v, %v; want nvwgf2umx.dll c0000409", crash, ok)
	}
	if _, ok := parseCS2Crash(raw, 0x999); ok {
		t.Fatal("an event of another cs2.exe process was attributed to this capture")
	}
	if crash, ok := parseCS2Crash(raw, 0); !ok || crash.module != "engine2.dll" {
		t.Fatalf("without a known pid the newest cs2.exe event wins, got %+v, %v", crash, ok)
	}
}

func TestCS2ExitSummaryNamesTheCrash(t *testing.T) {
	exit := cs2ExitInfo{known: true, code: 0xC0000005, abnormal: true, lifetime: 10400 * time.Millisecond}
	crash := cs2Crash{module: "engine2.dll", version: "1.0", exception: "c0000005", offset: "0000000000197a79"}
	got := cs2ExitSummary(exit, crash, true)
	want := "cs2.exe exit code 0xC0000005 (access violation) after 10.4s; Windows crash report: exception 0xC0000005 in engine2.dll 1.0 at offset 0x197A79"
	if got != want {
		t.Fatalf("summary =\n%s\nwant\n%s", got, want)
	}
	if got := cs2ExitSummary(cs2ExitInfo{}, cs2Crash{}, false); got != "cs2.exe exit code unknown; no Windows crash report" {
		t.Fatalf("unknown exit summary = %q", got)
	}
}
