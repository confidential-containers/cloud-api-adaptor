package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
)

type FlagInfo struct {
	FlagName    string `json:"flag_name"`
	FieldName   string `json:"field_name"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	EnvVar      string `json:"env_var"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
}

type ProviderConfig struct {
	Provider string     `json:"provider"`
	Flags    []FlagInfo `json:"flags"`
}

func main() {
	outputFormat := flag.String("o", "json", "Output format: json or table")
	noSecrets := flag.Bool("no-secrets", false, "Exclude secret env vars from output")
	secretsOnly := flag.Bool("only-secrets", false, "Include only secret env vars in output")
	includeAll := flag.Bool("include-shared", false, "Include common flags (shared by all providers)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [-o json|table] [-no-secrets|-only-secrets] [-include-shared] <provider-name>\n", os.Args[0])
		os.Exit(1)
	}

	provider := flag.Arg(0)

	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting executable path: %v\n", err)
		os.Exit(1)
	}
	execDir := filepath.Dir(execPath)

	config := &ProviderConfig{Provider: provider, Flags: []FlagInfo{}}

	// Parse common flags if -include-shared is set
	if *includeAll {
		commonPath := filepath.Join(execDir, "..", "..", "cloud-api-adaptor", "cmd", "cloud-api-adaptor", "main.go")
		commonFlags, err := parseFile(commonPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not parse common flags: %v\n", err)
		} else {
			config.Flags = append(config.Flags, commonFlags...)
		}
	}

	// Parse provider-specific flags from manager.go
	managerPath := filepath.Join(execDir, "..", provider, "manager.go")
	providerFlags, err := parseFile(managerPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	config.Flags = append(config.Flags, providerFlags...)

	// Filter secrets
	if *noSecrets {
		config.Flags = filterFlags(config.Flags, func(f FlagInfo) bool { return !f.Secret })
	} else if *secretsOnly {
		config.Flags = filterFlags(config.Flags, func(f FlagInfo) bool { return f.Secret })
	}

	// Sort flags alphabetically by env_var
	sort.Slice(config.Flags, func(i, j int) bool {
		return config.Flags[i].EnvVar < config.Flags[j].EnvVar
	})

	switch *outputFormat {
	case "json":
		output, _ := json.MarshalIndent(config, "", "  ")
		fmt.Println(string(output))
	case "table":
		printTable(config)
	default:
		fmt.Fprintf(os.Stderr, "Invalid output format: %s (use json or table)\n", *outputFormat)
		os.Exit(1)
	}
}

func parseFile(path string) ([]FlagInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var flags []FlagInfo

	// Find all reg.XxxWithEnv calls anywhere in the file
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if flagInfo, _ := extractFlagRegistrarCall(call, fset); flagInfo != nil {
				flags = append(flags, *flagInfo)
			}
		}
		return true
	})

	return flags, nil
}

func filterFlags(flags []FlagInfo, predicate func(FlagInfo) bool) []FlagInfo {
	var filtered []FlagInfo
	for _, f := range flags {
		if predicate(f) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func extractFlagRegistrarCall(call *ast.CallExpr, fset *token.FileSet) (*FlagInfo, string) {
	// Look for calls like: reg.StringWithEnv(...), reg.IntWithEnv(...), etc.
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, ""
	}

	// Check if this is a call on the 'reg' variable (FlagRegistrar)
	if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "reg" {
		methodName := sel.Sel.Name

		// Check if this is a known FlagRegistrar method
		var flagType string
		switch methodName {
		case "StringWithEnv":
			flagType = "string"
		case "IntWithEnv":
			flagType = "int"
		case "UintWithEnv":
			flagType = "uint"
		case "Float64WithEnv":
			flagType = "float64"
		case "BoolWithEnv":
			flagType = "bool"
		case "DurationWithEnv":
			flagType = "duration"
		case "CustomTypeWithEnv":
			flagType = "custom"
		default:
			// Unknown method on reg - record it
			pos := fset.Position(call.Pos())
			return nil, fmt.Sprintf("  - %s (line %d)", methodName, pos.Line)
		}

		if len(call.Args) < 5 {
			return nil, ""
		}

		flagInfo := &FlagInfo{Type: flagType}

		// Extract field name from arg[0]: &config.FieldName
		if unary, ok := call.Args[0].(*ast.UnaryExpr); ok {
			if sel, ok := unary.X.(*ast.SelectorExpr); ok {
				flagInfo.FieldName = sel.Sel.Name
			}
		}

		// Extract flag name from arg[1]: "flag-name"
		if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			flagInfo.FlagName = strings.Trim(lit.Value, `"`)
		}

		// Extract default value from arg[2]
		flagInfo.Default = exprToString(call.Args[2])

		// Extract env var from arg[3]: "ENV_VAR"
		if lit, ok := call.Args[3].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			flagInfo.EnvVar = strings.Trim(lit.Value, `"`)
		}

		// Extract description from arg[4]: "description"
		if lit, ok := call.Args[4].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			flagInfo.Description = strings.Trim(lit.Value, `"`)
		}

		// Extract FlagOption calls from variadic args (arg[5], arg[6], ...)
		// These are function calls like Required(), Secret(), provider.Required(), or provider.Secret()
		flagInfo.Required = false
		flagInfo.Secret = false
		for i := 5; i < len(call.Args); i++ {
			if optCall, ok := call.Args[i].(*ast.CallExpr); ok {
				optName := getFunctionName(optCall.Fun)
				switch optName {
				case "Required":
					flagInfo.Required = true
				case "Secret":
					flagInfo.Secret = true
				}
			}
		}

		return flagInfo, ""
	}

	return nil, ""
}

// getFunctionName extracts the function name from a call expression.
// Handles both simple identifiers (Required) and selector expressions (provider.Required).
func getFunctionName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return strings.Trim(e.Value, `"`)
	case *ast.Ident:
		return e.Name
	case *ast.UnaryExpr:
		// Handle negative numbers
		if e.Op == token.SUB {
			return "-" + exprToString(e.X)
		}
	}
	return ""
}

func printTable(config *ProviderConfig) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "FLAG NAME\tTYPE\tDEFAULT\tENV VAR\tREQUIRED\tSECRET\tDESCRIPTION\n")
	fmt.Fprintf(w, "---------\t----\t-------\t-------\t--------\t------\t-----------\n")

	for _, flag := range config.Flags {
		envVar := flag.EnvVar
		if envVar == "" {
			envVar = "-"
		}

		defaultVal := flag.Default
		if defaultVal == "" {
			defaultVal = `""`
		}

		required := "no"
		if flag.Required {
			required = "yes"
		}

		secret := "no"
		if flag.Secret {
			secret = "yes"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			flag.FlagName,
			flag.Type,
			defaultVal,
			envVar,
			required,
			secret,
			flag.Description,
		)
	}
	w.Flush()
}
