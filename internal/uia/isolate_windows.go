//go:build windows

package uia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/tamutamu/keymouse/internal/target"
)

const (
	WorkerScanArgument    = "__keymouse-uia-scan"
	WorkerExecuteArgument = "__keymouse-uia-execute"
)

// IsolatedDiscoverForeground executes all third-party provider calls in a
// disposable child process. CommandContext can terminate that process even
// when COM itself cannot cancel a hung provider call.
func IsolatedDiscoverForeground(timeout time.Duration) ([]target.Target, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	output, err := exec.CommandContext(ctx, exe, WorkerScanArgument).Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%w after %s", ErrDiscoveryTimeout, timeout)
	}
	if err != nil {
		return nil, fmt.Errorf("UIA scan worker: %w", err)
	}
	var targets []target.Target
	if err := json.Unmarshal(output, &targets); err != nil {
		return nil, fmt.Errorf("decode UIA scan: %w", err)
	}
	return targets, nil
}

func IsolatedExecuteTarget(want target.Target, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	data, err := json.Marshal(want)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, exe, WorkerExecuteArgument)
	cmd.Stdin = bytes.NewReader(data)
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%w after %s", ErrActionTimeout, timeout)
		}
		return fmt.Errorf("UIA action worker: %w: %s", runErr, bytes.TrimSpace(output))
	}
	return nil
}

func RunScanWorker(w io.Writer, hwnd uintptr) error {
	client, err := New()
	if err != nil {
		return err
	}
	defer client.Close()
	var targets []target.Target
	if hwnd != 0 {
		targets, err = client.ActionableTargets(hwnd)
	} else {
		targets, err = client.ForegroundActionableTargets()
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(targets)
}

func RunExecuteWorker(r io.Reader) error {
	var want target.Target
	if err := json.NewDecoder(r).Decode(&want); err != nil {
		return err
	}
	client, err := New()
	if err != nil {
		return err
	}
	defer client.Close()
	return client.ExecuteForeground(want)
}
