package renderplan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// HasFullDemoJSON distinguishes a legacy omission from an explicitly malformed
// new profile. In particular null must never silently resume with legacy defaults.
func HasFullDemoJSON(data []byte) (bool, error) {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return false, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') {
		return false, fmt.Errorf("edit must be a JSON object")
	}
	found := false
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return false, err
		}
		name, ok := token.(string)
		if !ok {
			return false, fmt.Errorf("invalid edit field name")
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return false, err
		}
		if strings.EqualFold(name, "full_demo") {
			if found || name != "full_demo" || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return false, fmt.Errorf("full_demo must be one explicitly named non-null approval snapshot")
			}
			found = true
		}
	}
	return found, nil
}

func (r *EditRequest) UnmarshalJSON(data []byte) error {
	full, err := HasFullDemoJSON(data)
	if err != nil {
		return err
	}
	type plain EditRequest
	var decoded plain
	dec := json.NewDecoder(bytes.NewReader(data))
	if full {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(&decoded); err != nil {
		return err
	}
	*r = EditRequest(decoded)
	return nil
}
