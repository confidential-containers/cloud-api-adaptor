// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

//go:build linux && amd64

package podvm

import (
	"context"
	"fmt"

	"golang.org/x/crypto/ssh"
)

type sshCommandResult struct {
	output []byte
	err    error
}

func runSSHCommandWithClient(ctx context.Context, client *ssh.Client, command string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("run SSH command: %w", err)
	}

	result := make(chan sshCommandResult, 1)
	go func() {
		session, err := client.NewSession()
		if err != nil {
			result <- sshCommandResult{err: err}
			return
		}
		defer session.Close()

		output, err := session.CombinedOutput(command)
		result <- sshCommandResult{output: output, err: err}
	}()

	select {
	case result := <-result:
		return result.output, result.err
	case <-ctx.Done():
		_ = client.Close()
		return nil, fmt.Errorf("run SSH command: %w", ctx.Err())
	}
}
