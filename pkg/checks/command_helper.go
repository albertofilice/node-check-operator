package checks

import (
	"context"
	"os/exec"

	"github.com/albertofilice/node-check-operator/api/v1alpha1"
)

// runHostCommandWithCommand executes a command and sets the command field in the result
// This is a helper function to simplify adding command tracking to all checks
func runHostCommandWithCommand(ctx context.Context, command string, result *v1alpha1.CheckResult) ([]byte, error) {
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		// Fallback to container command
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		output, err = cmd.Output()
	}
	return output, err
}

