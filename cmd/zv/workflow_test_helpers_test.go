package main

import "strings"

func documentedWorkflowCommand(command string) string {
	fields, ok := splitCommandFields(command)
	if !ok || len(fields) == 0 || fields[0] != "zv" {
		return ""
	}
	if len(fields) >= 2 && fields[1] == "short" {
		return "./bin/zv short"
	}
	if len(fields) >= 3 && fields[1] == "flows" && fields[2] == "run" {
		return "./bin/zv flows run"
	}
	out := []string{"./bin/zv"}
	for _, field := range fields[1:] {
		if strings.HasPrefix(field, "--") || strings.HasPrefix(field, "<") {
			break
		}
		out = append(out, field)
	}
	return strings.Join(out, " ")
}
