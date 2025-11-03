# config-extractor

A development tool for extracting and listing all configuration parameters for
each cloud provider.

## Purpose

This tool parses both common flags (from `main.go`) and provider-specific flags
(from `manager.go`) to extract all flag definitions (command-line parameters
and environment variables). This helps developers and users understand the
complete set of available configuration options for each provider.

Documentation and templates should avoid hardcoded values, and get values from
this tool. So we can keep them consistent.

## Building

From the `cloud-providers` directory:

```bash
make config-extractor
```

## Usage

```bash
./config-extractor [-o json|table] <provider-name>
```

### Arguments

- `-o`: Output format (default: `json`)
  - `json`: Outputs flags in JSON format
  - `table`: Outputs flags in a formatted table
- `<provider-name>`: Name of the provider (e.g., `gcp`, `azure`, `aws`, `ibmcloud`)

### Examples

List GCP provider flags in table format:
```bash
./config-extractor -o table gcp
```

List Azure provider flags in JSON format:
```bash
./config-extractor -o json azure
```

## Output

The tool extracts all flags in two sections:

1. **Common flags**: Flags defined in `main.go` that apply to all providers
   (e.g., `socket`, `pause-image`, `tunnel-type`)

2. **Provider-specific flags**: Flags defined in the provider's `manager.go`
   (e.g., `gcp-credentials`, `azure-subscription-id`)

For each flag, the tool extracts:

- Flag name (command-line argument)
- Type (string, int, bool, duration, custom, etc.)
- Default value
- Environment variable name
- Description

### Example Output (Table)

```
FLAG NAME                  TYPE      DEFAULT                                   ENV VAR                     DESCRIPTION
---------                  ----      -------                                   -------                     -----------
socket                     string    ""                                        REMOTE_HYPERVISOR_ENDPOINT  Unix domain socket path of remote hypervisor service
pods-dir                   string    ""                                        PODS_DIR                    base directory for pod directories
pause-image                string    ""                                        PAUSE_IMAGE                 pause image to be used for the pods
...
gcp-credentials            string    ""                                        GCP_CREDENTIALS             Google Application Credentials
gcp-project-id             string    ""                                        GCP_PROJECT_ID              GCP Project ID
zone                       string    ""                                        GCP_ZONE                    Zone
...
```

### Example Output (JSON)

```json
{
  "provider": "gcp",
  "flags": [
    {
      "flag_name": "gcp-credentials",
      "field_name": "GcpCredentials",
      "type": "string",
      "default": "",
      "env_var": "GCP_CREDENTIALS",
      "description": "Google Application Credentials"
    },
    ...
  ]
}
```
