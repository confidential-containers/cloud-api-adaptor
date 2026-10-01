package podvm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

func retryUntil(ctx context.Context, description string, operation func(context.Context) error) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := operation(attemptCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for %s: %w (last error: %v)", description, ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}
func waitForSystemdService(ctx context.Context, client *ssh.Client, unit string) error {
	err := retryUntil(ctx, unit, func(ctx context.Context) error {
		command := fmt.Sprintf("systemctl is-active --quiet %s && systemctl show %s --property=InvocationID --value", unit, unit)
		firstInvocation, err := runSSHCommand(ctx, client, command)
		if err != nil {
			return err
		}

		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}

		lastInvocation, err := runSSHCommand(ctx, client, command)
		if err != nil {
			return err
		}
		if string(firstInvocation) != string(lastInvocation) {
			return fmt.Errorf("%s restarted during the stability check", unit)
		}
		return nil
	})
	if err == nil {
		return nil
	}

	debug := os.Getenv("TEST_PODVM_DEBUG")
	diagnostics := []struct {
		name    string
		command string
	}{
		{
			name: "systemd state",
			command: "systemctl show " + unit +
				" --property=ActiveState,SubState,Result,ExecMainCode,ExecMainStatus,NRestarts",
		},
	}
	if debug == "1" || debug == "true" {
		diagnostics = append(diagnostics, struct {
			name    string
			command string
		}{
			name: "latest invocation journal",
			command: "invocation_id=$(systemctl show " + unit + " --property=InvocationID --value); " +
				"if [ -n \"$invocation_id\" ]; then " +
				"journalctl --no-pager --output=short-precise _SYSTEMD_INVOCATION_ID=\"$invocation_id\"; " +
				"else journalctl --boot --unit " + unit + " --no-pager --output=short-precise --lines=30; fi",
		})
	}

	message := fmt.Sprintf("%v", err)
	for _, diagnostic := range diagnostics {
		diagnosticCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		output, diagnosticErr := runSSHCommand(diagnosticCtx, client, diagnostic.command)
		cancel()
		message += fmt.Sprintf("\n\n==> %s <==\n%s", diagnostic.name, output)
		if diagnosticErr != nil && len(output) == 0 {
			message += fmt.Sprintf("failed to collect diagnostics: %v\n", diagnosticErr)
		}
	}
	return errors.New(message)
}
