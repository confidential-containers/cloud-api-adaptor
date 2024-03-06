// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package hack

// This statement forces `go mod` to recognize the packages required by update-codegen.sh
import _ "k8s.io/code-generator"

// For VS Code users, adding the following setting in .vscode/settings.json will suppress
// the error of `build constraints exclude all Go files`
//   "gopls": {
//     "build.directoryFilters": ["-hack"]
//   }
