package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// Only a boot whose file holds a fatal error crashed; a killed boot leaves an empty file that the next start drops.
func TestCrashLogTellsACrashedBootFromAKilledOne(t *testing.T) {
	c := crashLog{dir: t.TempDir()}
	crashedBoot, killedBoot := uuid.NewString(), uuid.NewString()
	if err := os.WriteFile(filepath.Join(c.dir, crashedBoot+".txt"), []byte("fatal error: runtime: out of memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.dir, killedBoot+".txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !c.crashed(crashedBoot) || c.crashed(killedBoot) || c.crashed(uuid.NewString()) || c.crashed("../"+crashedBoot) {
		t.Fatal("crashed answers true only for the boot with a fatal error on file")
	}
}
