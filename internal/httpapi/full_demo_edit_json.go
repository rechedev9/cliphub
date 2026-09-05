package httpapi

import (
	"bytes"
	"encoding/json"

	"github.com/rechedev9/cliphub/internal/renderplan"
)

func (r *renderEditRequest) UnmarshalJSON(data []byte) error {
	if _, err := renderplan.HasFullDemoJSON(data); err != nil {
		return err
	}
	type plain renderEditRequest
	var decoded plain
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&decoded); err != nil {
		return err
	}
	*r = renderEditRequest(decoded)
	return nil
}
