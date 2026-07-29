package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var errMultipleJSONValues = errors.New("request body must contain exactly one JSON value")

// decodeSingleJSONBody bounds a control document and requires EOF after its
// first JSON value. Optional-body callers may treat io.EOF as an empty request.
func decodeSingleJSONBody(w http.ResponseWriter, r *http.Request, dst any, disallowUnknownFields bool) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	if disallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var trailing any
	switch err := decoder.Decode(&trailing); {
	case errors.Is(err, io.EOF):
		return nil
	case err == nil:
		return errMultipleJSONValues
	default:
		return fmt.Errorf("invalid trailing JSON data: %w", err)
	}
}
