package recording

import (
	"strings"
	"testing"
)

func TestFullDemoConsoleTransportRejectsIncompleteOrForeignEvidence(t *testing.T) {
	p := fullDemoCaptureFixture(t)
	for _, text := range []string{
		`09/06 17:58:41 ZV_FULL_DEMO:foreign:{"kind":"settings_restored","success":true}`,
		`09/06 17:58:41 player said ZV_FULL_DEMO:test:{"kind":"settings_restored","success":true}`,
		`09/06 17:58:41 ZV_FULL_DEMO:test:{"kind":"settings_restored","success":`,
		"09/06 17:58:41 ZV_FULL_DEMO:test:{\"kind\":\"settings_before\",\"values\":[\n09/06 17:58:41 ZV_FULL_DEMO:test:{\"kind\":\"settings_restored\",\"success\":true}\n",
		"ZV_FULL_DEMO:test:{\"kind\":\"settings_restored\",\"success\":true} trailing text\n",
		"ZV_FULL_DEMO:test:{" + strings.Repeat(" ", 1<<20),
	} {
		if _, err := ReadFullDemoCaptureEvidence(strings.NewReader(text), "test", p); err == nil {
			t.Fatal("accepted incomplete, forged or over-limit evidence")
		}
	}
}
