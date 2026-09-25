package composition

import (
	"strings"
	"testing"
)

func TestValidateUploadResult(t *testing.T) {
	tests := []struct {
		name    string
		result  Result
		wantErr string
	}{
		{name: "successful output", result: Result{Output: "final.mp4"}},
		{name: "failed result", result: Result{Output: "final.mp4", Error: "compose failed"}, wantErr: "composition result error: compose failed"},
		{name: "missing output", result: Result{}, wantErr: "composition result has no output"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUploadResult(tt.result)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateUploadResult error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateUploadResult error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
