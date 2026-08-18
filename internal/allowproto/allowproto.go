// Package allowproto sets GOLANG_PROTOBUF_REGISTRATION_CONFLICT=ignore before
// any other ClipHub package registers protobuf types.
//
// go-steam and demoinfocs-golang both claim extension 50000. The process that
// imports both (the orchestrator) would otherwise panic in init. The package
// name is intentionally early in the alphabet so gofmt keeps this import in
// front of httpapi and workers.
package allowproto

import "os"

func init() {
	if os.Getenv("GOLANG_PROTOBUF_REGISTRATION_CONFLICT") == "" {
		_ = os.Setenv("GOLANG_PROTOBUF_REGISTRATION_CONFLICT", "ignore")
	}
}

// Loaded is referenced by main so the import cannot be dropped.
const Loaded = true
