# config-extractor

A development tool for extracting and listing all configuration parameters for each cloud provider.

## Purpose

This tool parses the `manager.go` files for each cloud provider and extracts all flag definitions (command-line parameters and environment variables) to help developers and users understand the available configuration options.

## Building

From the `cloud-providers` directory:

```bash
make config-extractor
```

## Usage

```bash
./bin/config-extractor [-o json|table] [-no-secrets|-only-secrets] [-include-shared] <provider-name>
```

### Arguments

- `-o`: Output format (default: `json`)
  - `json`: Outputs flags in JSON format
  - `table`: Outputs flags in a formatted table
- `-no-secrets`: Exclude secret environment variables from output
- `-only-secrets`: Include only secret environment variables in output
- `-include-shared`: Include common flags shared by all providers
- `<provider-name>`: Name of the provider (e.g., `gcp`, `azure`, `aws`, `ibmcloud`)

### Examples

List GCP provider flags in table format:
```bash
./bin/config-extractor -o table gcp
```

List Azure provider flags in JSON format:
```bash
./bin/config-extractor -o json azure
```

## Output

The tool extracts the following information for each flag:
- Flag name (command-line argument)
- Type (string, int, bool, etc.)
- Default value
- Environment variable name
- Required (whether the flag is required)
- Secret (whether the flag contains sensitive data)
- Description

### Example Output (Table)

```
FLAG NAME          TYPE    DEFAULT      ENV VAR          REQUIRED  SECRET  DESCRIPTION
---------          ----    -------      -------          --------  ------  -----------
gcp-credentials    string  ""           GCP_CREDENTIALS  no        yes     Google Application Credentials
gcp-project-id     string  ""           GCP_PROJECT_ID   yes       no      GCP Project ID
zone               string  ""           GCP_ZONE         yes       no      Zone
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
      "description": "Google Application Credentials",
      "required": false,
      "secret": true
    },
    ...
  ]
}
```
