package main

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/gordonklaus/ineffassign/pkg/ineffassign"
	"github.com/timakin/bodyclose/passes/bodyclose"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"
)

func main() {
	analyzers := []*analysis.Analyzer{
		ineffassign.Analyzer,
		bodyclose.Analyzer,
	}

	for _, analyzer := range staticcheck.Analyzers {
		analyzers = append(analyzers, analyzer.Analyzer)
	}

	for _, analyzer := range stylecheck.Analyzers {
		analyzers = append(analyzers, analyzer.Analyzer)
	}

	multichecker.Main(analyzers...)
}
