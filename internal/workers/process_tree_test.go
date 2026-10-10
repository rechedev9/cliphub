package workers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const (
	// treeHelperDirEnv turns the test binary into a stand-in for a media
	// tool: it writes its pid into that directory and starts one more copy
	// of itself per remaining generation, like an editor that starts ffmpeg.
	treeHelperDirEnv   = "ZV_WORKERS_TREE_HELPER_DIR"
	treeHelperDepthEnv = "ZV_WORKERS_TREE_HELPER_DEPTH"
	// treeHelperExitEnv makes every generation but the last exit once it has
	// started the next, like a tool that finishes and leaves a viewer open.
	treeHelperExitEnv = "ZV_WORKERS_TREE_HELPER_EXIT"
	// treeHelperLifetime keeps a helper from outliving a failed test run.
	treeHelperLifetime = 90 * time.Second
)

func runTreeHelper(dir string) {
	depth, _ := strconv.Atoi(os.Getenv(treeHelperDepthEnv))
	if depth > 0 {
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(), treeHelperDepthEnv+"="+strconv.Itoa(depth-1))
		if err := child.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "tree helper: start child:", err)
			os.Exit(2)
		}
	}
	pidFile := filepath.Join(dir, fmt.Sprintf("generation-%d.pid", depth))
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "tree helper: write pid:", err)
		os.Exit(2)
	}
	if depth > 0 && os.Getenv(treeHelperExitEnv) != "" {
		return
	}
	time.Sleep(treeHelperLifetime)
}

// treeHelperPIDs waits until every generation of a helper tree has written
// its pid and returns them, the tool itself last.
func treeHelperPIDs(t *testing.T, dir string, generations int) []int {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		pids := []int{}
		for depth := range generations {
			raw, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("generation-%d.pid", depth)))
			if err != nil {
				break
			}
			pid, err := strconv.Atoi(string(raw))
			if err != nil {
				break
			}
			pids = append(pids, pid)
		}
		if len(pids) == generations {
			return pids
		}
		if time.Now().After(deadline) {
			t.Fatalf("the helper tree did not start: %d of %d generations wrote a pid", len(pids), generations)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
