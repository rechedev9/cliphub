package workers

import "log"

func logProcessTree(tool string, ended int, err error) {
	switch {
	case err != nil:
		log.Printf("worker: ended %s and %d processes it started, with problems: %v", tool, ended, err)
	case ended > 0:
		log.Printf("worker: ended %s and %d processes it started", tool, ended)
	}
}

// EndLeftoverTools ends every running process whose executable is one of the
// given files, with everything those processes started, and reports how many
// it ended. The cloud worker calls it at start for its recorder and editor:
// an orchestrator that died left them running, and a recorder still holds
// CS2 open, so the machine could not capture again until they were gone.
func EndLeftoverTools(executables ...string) (int, error) {
	return endProcessesOf(executables)
}
