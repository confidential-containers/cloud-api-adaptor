// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"testing"

	"cloud.google.com/go/auth/credentials"
)

func TestGcpCredType(t *testing.T) {
	tests := []struct {
		name    string
		creds   string
		want    credentials.CredType
		wantErr bool
	}{
		{
			name:  "service_account",
			creds: `{"type":"service_account","client_email":"sa@example.iam.gserviceaccount.com"}`,
			want:  credentials.ServiceAccount,
		},
		{
			name:  "external_account",
			creds: `{"type":"external_account","credential_source":{"file":"/var/run/secrets/tokens/gcp-ksa/token"}}`,
			want:  credentials.ExternalAccount,
		},
		{
			name:    "external_account without file source is rejected",
			creds:   `{"type":"external_account"}`,
			wantErr: true,
		},
		{
			name:    "external_account with url source is rejected",
			creds:   `{"type":"external_account","credential_source":{"url":"http://example.com/token"}}`,
			wantErr: true,
		},
		{
			name:    "external_account with executable source is rejected",
			creds:   `{"type":"external_account","credential_source":{"executable":{"command":"id"}}}`,
			wantErr: true,
		},
		{
			name:    "unsupported type is rejected",
			creds:   `{"type":"impersonated_service_account"}`,
			wantErr: true,
		},
		{
			name:    "missing type is rejected",
			creds:   `{"client_email":"sa@example.iam.gserviceaccount.com"}`,
			wantErr: true,
		},
		{
			name:    "malformed JSON is rejected",
			creds:   `{not json`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := gcpCredType(tc.creds)
			if (err != nil) != tc.wantErr {
				t.Fatalf("gcpCredType() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("gcpCredType() = %q, want %q", got, tc.want)
			}
		})
	}
}
