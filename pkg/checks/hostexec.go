package checks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

const hostRootMountPath = "/host/root"

// runHostCommand executes the provided shell command inside the host namespaces
// using nsenter and the mounted host root filesystem. It returns the combined
// stdout/stderr output so callers can include detailed error messages.
func runHostCommand(ctx context.Context, command string) ([]byte, error) {
	if _, err := exec.LookPath("nsenter"); err != nil {
		return nil, fmt.Errorf("nsenter not available: %w", err)
	}

	if _, err := os.Stat(hostRootMountPath); err != nil {
		return nil, fmt.Errorf("host root not mounted at %s: %w", hostRootMountPath, err)
	}

	// Quote the command to preserve spaces/pipes safely
	quotedCommand := fmt.Sprintf("%q", command)
	fullCommand := fmt.Sprintf("nsenter -t 1 -m -p -n chroot %s /bin/sh -c %s", hostRootMountPath, quotedCommand)

	cmd := exec.CommandContext(ctx, "sh", "-c", fullCommand)
	return cmd.CombinedOutput()
}

