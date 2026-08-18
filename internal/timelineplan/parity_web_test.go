package timelineplan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const webParityFixture = `web/lib/editor/testdata/parity-plans.json`

type webParityCase struct {
	Name          string          `json:"name"`
	Doc           json.RawMessage `json:"doc"`
	ValidateError *string         `json:"validateError"`
	RenderError   *string         `json:"renderError"`
}

func TestDefaultDocumentParity(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(DefaultDocument())
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode(DefaultDocument) = %v, want nil", err)
	}
	assertParityError(t, "Validate", doc.Validate(), nil)
	wantRender := "timeline has no items"
	assertParityError(t, "ValidateForRender", doc.ValidateForRender(), &wantRender)
}

func TestWebParityPlans(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", filepath.FromSlash(webParityFixture))
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("shared web parity fixture missing: %s (another agent writes it)", path)
		}
		t.Fatalf("read %s: %v", path, err)
	}
	var cases []webParityCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	for i, tc := range cases {
		name := tc.Name
		if name == "" {
			name = filepath.Base(path) + "#" + strconv.Itoa(i)
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			doc, err := Decode(tc.Doc)
			if err != nil {
				t.Fatalf("Decode() = %v, want a timelineplan.Document", err)
			}
			assertParityError(t, "Validate", doc.Validate(), tc.ValidateError)
			assertParityError(t, "ValidateForRender", doc.ValidateForRender(), tc.RenderError)
		})
	}
}

func assertParityError(t *testing.T, op string, err error, want *string) {
	t.Helper()
	if want == nil {
		if err != nil {
			t.Fatalf("%s() = %v, want nil", op, err)
		}
		return
	}
	if err == nil || err.Error() != *want {
		t.Fatalf("%s() = %v, want %q", op, err, *want)
	}
}
