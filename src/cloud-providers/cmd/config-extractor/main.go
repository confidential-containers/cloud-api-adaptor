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
	IsCommon    bool   `json:"is_common"`
}

type ProviderConfig struct {
	Provider string     `json:"provider"`
	Flags    []FlagInfo `json:"flags"`
}

func main() {
	outputFormat := flag.String("o", "json", "Output format: json or table")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [-o json|table] <provider-name>\n", os.Args[0])
		os.Exit(1)
	}

	provider := flag.Arg(0)

	// Find repository base path intelligently
	repoBase, err := findRepoBase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding repository base: %v\n", err)
		os.Exit(1)
	}

	// Extract common flags from main.go
	mainPath := filepath.Join(repoBase, "src", "cloud-api-adaptor", "cmd", "cloud-api-adaptor", "main.go")
	commonConfig, err := parseMainFile(mainPath, "common")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing main.go: %v\n", err)
		os.Exit(1)
	}

	// Mark common flags
	for i := range commonConfig.Flags {
		commonConfig.Flags[i].IsCommon = true
	}

	// Extract provider-specific flags from manager.go
	managerPath := filepath.Join(repoBase, "src", "cloud-providers", provider, "manager.go")
	providerConfig, err := parseManagerFile(managerPath, provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing %s/manager.go: %v\n", provider, err)
		os.Exit(1)
	}

	// Mark provider-specific flags
	for i := range providerConfig.Flags {
		providerConfig.Flags[i].IsCommon = false
	}

	// Combine both configs (common first, then provider-specific)
	config := &ProviderConfig{
		Provider: provider,
		Flags:    append(commonConfig.Flags, providerConfig.Flags...),
	}

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

func findRepoBase() (string, error) {
	// Start from the executable's directory and walk up to find the repo root
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(execPath)

	// Walk up the directory tree looking for a marker that indicates repo root
	// We'll look for "src/cloud-providers" and "src/cloud-api-adaptor" directories
	for i := 0; i < 10; i++ { // Limit to 10 levels up
		// Check if we're at repo root by looking for expected structure
		cloudProvidersPath := filepath.Join(dir, "src", "cloud-providers")
		cloudAdaptorPath := filepath.Join(dir, "src", "cloud-api-adaptor")

		if _, err := os.Stat(cloudProvidersPath); err == nil {
			if _, err := os.Stat(cloudAdaptorPath); err == nil {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find repository root (expected to find src/cloud-providers and src/cloud-api-adaptor)")
}

func parseManagerFile(path string, provider string) (*ProviderConfig, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	config := &ProviderConfig{Provider: provider, Flags: []FlagInfo{}}

	// Parse ParseCmd function to extract FlagRegistrar method calls
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "ParseCmd" {
			for _, stmt := range fn.Body.List {
				extractFromStatement(stmt, config)
			}
		}
		return true
	})

	return config, nil
}

func parseMainFile(path string, name string) (*ProviderConfig, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	config := &ProviderConfig{Provider: name, Flags: []FlagInfo{}}

	// Parse Setup method to extract FlagRegistrar method calls
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "Setup" {
			for _, stmt := range fn.Body.List {
				extractFromStatementRecursive(stmt, config)
			}
		}
		return true
	})

	return config, nil
}

func extractFromStatementRecursive(stmt ast.Stmt, config *ProviderConfig) {
	// Handle expression statements
	if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
		if call, ok := exprStmt.X.(*ast.CallExpr); ok {
			if flagInfo := extractFlagRegistrarCall(call); flagInfo != nil {
				config.Flags = append(config.Flags, *flagInfo)
			}

			// Look for cmd.Parse call - the function literal is at index 2
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Parse" && len(call.Args) >= 3 {
					if fnLit, ok := call.Args[2].(*ast.FuncLit); ok {
						// Recurse into the function body
						for _, stmt := range fnLit.Body.List {
							extractFromStatementRecursive(stmt, config)
						}
					}
				}
			}
		}
	}
}

func extractFromStatement(stmt ast.Stmt, config *ProviderConfig) {
	if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
		if call, ok := exprStmt.X.(*ast.CallExpr); ok {
			if flagInfo := extractFlagRegistrarCall(call); flagInfo != nil {
				config.Flags = append(config.Flags, *flagInfo)
			}
		}
	}
}

func extractFlagRegistrarCall(call *ast.CallExpr) *FlagInfo {
	// Look for calls like: reg.StringWithEnv(...), reg.IntWithEnv(...), etc.
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	methodName := sel.Sel.Name

	// Check if this is a FlagRegistrar method
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
		return nil
	}

	if len(call.Args) < 5 {
		return nil
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

	return flagInfo
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
	fmt.Fprintf(w, "FLAG NAME\tTYPE\tDEFAULT\tENV VAR\tDESCRIPTION\n")
	fmt.Fprintf(w, "---------\t----\t-------\t-------\t-----------\n")

	for _, flag := range config.Flags {
		envVar := flag.EnvVar
		if envVar == "" {
			envVar = "-"
		}

		defaultVal := flag.Default
		if defaultVal == "" {
			defaultVal = `""`
		}

		// Add asterisk prefix for common flags
		prefix := ""
		if flag.IsCommon {
			prefix = "* "
		}

		fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\n",
			prefix,
			flag.FlagName,
			flag.Type,
			defaultVal,
			envVar,
			flag.Description,
		)
	}
	w.Flush()

	// Print legend at the bottom
	fmt.Println()
	fmt.Println("* = Common flags (available for all providers)")
}
