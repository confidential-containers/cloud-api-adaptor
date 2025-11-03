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
./bin/config-extractor [-o json|table] <provider-name>
```

### Arguments

- `-o`: Output format (default: `json`)
  - `json`: Outputs flags in JSON format
  - `table`: Outputs flags in a formatted table
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
- Description

### Example Output (Table)

```
FLAG NAME          TYPE    DEFAULT      ENV VAR          DESCRIPTION
---------          ----    -------      -------          -----------
gcp-credentials    string  ""           GCP_CREDENTIALS  Google Application Credentials
gcp-project-id     string  ""           -                GCP Project ID
zone               string  ""           -                Zone
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
