package steamresolve

import (
	"os"
	"strings"
)

// Session is a short-lived Steam logon. It is never persisted: callers pass it
// per request or the process reads it from the environment.
type Session struct {
	Username string
	Password string
	Guard    string
}

// Complete reports whether every logon field is non-empty after trim.
func (s Session) Complete() bool {
	return s.Username != "" && s.Password != "" && s.Guard != ""
}

// SessionFromEnv reads ZV_STEAM_USERNAME / PASSWORD / GUARD. Empty values are
// discarded so a blank env var is "not set", not "set to empty".
func SessionFromEnv() Session {
	return Session{
		Username: strings.TrimSpace(os.Getenv(EnvUsername)),
		Password: strings.TrimSpace(os.Getenv(EnvPassword)),
		Guard:    strings.TrimSpace(os.Getenv(EnvGuard)),
	}
}

// Configured reports whether the environment holds a complete Steam session.
func Configured() bool {
	return SessionFromEnv().Complete()
}
