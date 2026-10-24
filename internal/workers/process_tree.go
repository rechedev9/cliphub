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
