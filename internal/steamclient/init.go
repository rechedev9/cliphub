// Package steamclient is the go-steam Game Coordinator adapter.
//
// The orchestrator also imports demoinfocs-golang, which registers the same
// protobuf extension number. internal/allowproto must be imported first so the
// second registration is ignored instead of crashing init.
package steamclient
