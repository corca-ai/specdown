package core

import "regexp"

// VariableRefExpr is the raw regex fragment matching a variable name
// (including dot-path access like "foo.bar.baz").
const VariableRefExpr = `[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*`

// VariablePattern matches ${name} references with optional backslash escaping.
// Group 1: optional backslash prefix; Group 2: variable name (possibly dotted).
var VariablePattern = regexp.MustCompile(`(\\?)\$\{(` + VariableRefExpr + `)\}`)
