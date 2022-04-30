// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cloudinit

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// https://cloudinit.readthedocs.io/en/latest/topics/format.html#cloud-config-data

type CloudConfig struct {
	WriteFiles []WriteFile `yaml:"write_files"`
}

// https://cloudinit.readthedocs.io/en/latest/topics/modules.html#write-files
type WriteFile struct {
	Path        string `yaml:"path"`
	Content     string `yaml:"content,omitempty"`
	Owner       string `yaml:"owner,omitempty"`
	Permissions string `yaml:"permissions,omitempty"`
	Encoding    string `yaml:"encoding,omitempty"`
	Append      string `yaml:"append,omitempty"`
}

const cloudInitText = `{{/* Template for cloud-config */ -}}
#cloud-config
{{- if .WriteFiles }}

write_files:
{{- range .WriteFiles }}
  - path: {{ .Path }}
{{- if .Owner }}
    owner: {{ .Owner }}
{{- end }}
{{- if .Permissions }}
    permissions: {{ .Permissions }}
{{- end }}
{{- if .Encoding }}
    encoding: {{ .Encoding }}
{{- end }}
{{- if .Append }}
    append: {{ .Append }}
{{- end }}
{{- if .Content }}
    content: |
{{- range splitLines .Content }}
      {{ . }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
`

var templateFuncMap = template.FuncMap{
	"splitLines": splitLines,
}

func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil
	}
	if lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}

func (config *CloudConfig) Generate() (string, error) {

	tpl, err := template.New("base").Funcs(templateFuncMap).Parse(cloudInitText)
	if err != nil {
		return "", fmt.Errorf("Error initializing a template for cloudinit userdata: %w", err)
	}

	var buf bytes.Buffer

	if err := tpl.Execute(&buf, config); err != nil {
		return "", fmt.Errorf("Error executing a template for cloudinit userdata: %w", err)
	}

	return buf.String(), nil
}
