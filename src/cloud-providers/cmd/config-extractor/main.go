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

	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting executable path: %v\n", err)
		os.Exit(1)
	}
	execDir := filepath.Dir(execPath)
	managerPath := filepath.Join(execDir, "..", provider, "manager.go")

	config, err := parseManagerFile(managerPath, provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
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

func parseManagerFile(path string, provider string) (*ProviderConfig, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	config := &ProviderConfig{Provider: provider, Flags: []FlagInfo{}}
	var unknownMethods []string

	// Parse ParseCmd function to extract FlagRegistrar method calls
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "ParseCmd" {
			for _, stmt := range fn.Body.List {
				extractFromStatement(stmt, config, &unknownMethods, fset)
			}
		}
		return true
	})

	// Fail hard if we encountered unknown flag registration methods
	if len(unknownMethods) > 0 {
		return nil, fmt.Errorf("encountered unknown flag registration method(s):\n%s\n\nThe parser needs to be updated to handle these methods.",
			strings.Join(unknownMethods, "\n"))
	}

	return config, nil
}

func extractFromStatement(stmt ast.Stmt, config *ProviderConfig, unknownMethods *[]string, fset *token.FileSet) {
	if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
		if call, ok := exprStmt.X.(*ast.CallExpr); ok {
			flagInfo, unknown := extractFlagRegistrarCall(call, fset)
			if flagInfo != nil {
				config.Flags = append(config.Flags, *flagInfo)
			} else if unknown != "" {
				*unknownMethods = append(*unknownMethods, unknown)
			}
		}
	}
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

		return flagInfo, ""
	}

	return nil, ""
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

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			flag.FlagName,
			flag.Type,
			defaultVal,
			envVar,
			flag.Description,
		)
	}
	w.Flush()
}
