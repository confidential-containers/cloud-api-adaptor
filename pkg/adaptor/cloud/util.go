// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloud

import "os"

func DefaultToEnv(field *string, env, fallback string) {

	if *field != "" {
		return
	}

	val := os.Getenv(env)
	if val == "" {
		val = fallback
	}

	*field = val
}
