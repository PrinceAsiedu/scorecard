// Copyright 2025 OpenSSF Scorecard Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package memorysafe

import (
	"embed"
	"fmt"
	"go/parser"
	"go/token"
	"reflect"
	"strings"

	"github.com/ossf/scorecard/v5/checker"
	"github.com/ossf/scorecard/v5/checks/fileparser"
	"github.com/ossf/scorecard/v5/clients"
	"github.com/ossf/scorecard/v5/finding"
	"github.com/ossf/scorecard/v5/internal/dotnet/csproj"
	"github.com/ossf/scorecard/v5/internal/probes"
)

//go:embed *.yml
var fs embed.FS

const (
	Probe = "memorysafe"
)

type languageMemoryCheckConfig struct {
	Desc string

	funcPointers []func(client *checker.CheckRequest) ([]finding.Finding, error)
}

var languageMemorySafeSpecs = map[clients.LanguageName]languageMemoryCheckConfig{
	clients.Go: {
		funcPointers: []func(client *checker.CheckRequest) ([]finding.Finding, error){
			checkGoUnsafePackage,
		},
		Desc: "Check if Go code uses the unsafe package",
	},

	clients.CSharp: {
		funcPointers: []func(client *checker.CheckRequest) ([]finding.Finding, error){
			checkDotnetAllowUnsafeBlocks,
		},
		Desc: "Check if C# code uses unsafe blocks",
	},
}

func init() {
	probes.MustRegisterIndependent(Probe, Run)
}

func Run(raw *checker.CheckRequest) (found []finding.Finding, probeName string, err error) {
	prominentLangs := getRepositoryLanguageChecks(raw)
	findings := []finding.Finding{}

	for _, lang := range prominentLangs {
		for _, langFunc := range lang.funcPointers {
			if langFunc == nil {
				raw.Dlogger.Warn(&checker.LogMessage{
					Text: fmt.Sprintf("no function pointer found for language %s", lang.Desc),
				})
			}
			langFindings, err := langFunc(raw)
			if err != nil {
				return nil, Probe, fmt.Errorf("error while running function for language %s: %w", lang.Desc, err)
			}
			findings = append(findings, langFindings...)
		}
	}
	return findings, Probe, nil
}

func getRepositoryLanguageChecks(raw *checker.CheckRequest) []languageMemoryCheckConfig {
	langs, err := raw.RepoClient.ListProgrammingLanguages()
	if err != nil {
		raw.Dlogger.Warn(&checker.LogMessage{
			Text: fmt.Sprintf("RepoClient retured error for ListProgrammingLanguages: %v", err),
		})
		return nil
	}
	if len(langs) == 0 {
		return []languageMemoryCheckConfig{}
	}
	if len(langs) == 1 && langs[0].Name == clients.All {
		return getAllLanguages()
	}
	ret := []languageMemoryCheckConfig{}
	for _, language := range langs {
		if lang, ok := languageMemorySafeSpecs[clients.LanguageName(strings.ToLower(string(language.Name)))]; ok {
			ret = append(ret, lang)
		}
	}
	return ret
}

func getAllLanguages() []languageMemoryCheckConfig {
	allLanguages := make([]languageMemoryCheckConfig, 0, len(languageMemorySafeSpecs))
	for l := range languageMemorySafeSpecs {
		allLanguages = append(allLanguages, languageMemorySafeSpecs[l])
	}
	return allLanguages
}

// Golang

func checkGoUnsafePackage(client *checker.CheckRequest) ([]finding.Finding, error) {
	findings := []finding.Finding{}

	if err := fileparser.OnMatchingFileContentDo(client.RepoClient, fileparser.PathMatcher{
		Pattern:       "*.go",
		CaseSensitive: false,
	}, goCodeUsesUnsafePackage, &findings, client.Dlogger); err != nil {
		return nil, err
	}
	if len(findings) == 0 {
		found, err := finding.NewWith(fs, Probe,
			"Golang code does not use the unsafe package", nil, finding.OutcomeTrue)
		if err != nil {
			return nil, fmt.Errorf("create finding: %w", err)
		}
		findings = append(findings, *found)
	}
	return findings, nil
}

func goCodeUsesUnsafePackage(path string, content []byte, args ...interface{}) (bool, error) {
	findings, ok := args[0].(*[]finding.Finding)
	if !ok {
		// panic if it is not correct type
		panic(fmt.Sprintf("expected type findings, got %v", reflect.TypeOf(args[0])))
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", content, parser.ImportsOnly)
	if err != nil {
		dl, ok := args[1].(checker.DetailLogger)
		if !ok {
			// panic if it is not correct type
			panic(fmt.Sprintf("expected type checker.DetailLogger, got %v", reflect.TypeOf(args[1])))
		}

		dl.Warn(&checker.LogMessage{
			Text: fmt.Sprintf("malformed golang file: %v", err),
		})
		return true, nil
	}
	for _, i := range f.Imports {
		if i.Path.Value == `"unsafe"` {
			found, err := finding.NewWith(fs, Probe,
				"Golang code uses the unsafe package", &finding.Location{
					Path: path,
				}, finding.OutcomeFalse)
			if err != nil {
				return false, fmt.Errorf("create finding: %w", err)
			}
			*findings = append(*findings, *found)
		}
	}

	return true, nil
}

// CSharp

func checkDotnetAllowUnsafeBlocks(client *checker.CheckRequest) ([]finding.Finding, error) {
	findings := []finding.Finding{}

	if err := fileparser.OnMatchingFileContentDo(client.RepoClient, fileparser.PathMatcher{
		Pattern:       "*.csproj",
		CaseSensitive: false,
	}, csProjAllosUnsafeBlocks, &findings, client.Dlogger); err != nil {
		return nil, err
	}
	if len(findings) == 0 {
		found, err := finding.NewWith(fs, Probe,
			"C# code does not allow unsafe blocks", nil, finding.OutcomeTrue)
		if err != nil {
			return nil, fmt.Errorf("create finding: %w", err)
		}
		findings = append(findings, *found)
	}
	return findings, nil
}

func csProjAllosUnsafeBlocks(path string, content []byte, args ...interface{}) (bool, error) {
	findings, ok := args[0].(*[]finding.Finding)
	if !ok {
		// panic if it is not correct type
		panic(fmt.Sprintf("expected type findings, got %v", reflect.TypeOf(args[0])))
	}
	unsafe, err := csproj.IsAllowUnsafeBlocksEnabled(content)
	if err != nil {
		dl, ok := args[1].(checker.DetailLogger)
		if !ok {
			// panic if it is not correct type
			panic(fmt.Sprintf("expected type checker.DetailLogger, got %v", reflect.TypeOf(args[1])))
		}

		dl.Warn(&checker.LogMessage{
			Text: fmt.Sprintf("malformed csproj file: %v", err),
		})
		return true, nil
	}
	if unsafe {
		found, err := finding.NewWith(fs, Probe,
			"C# code allows the use of unsafe blocks", &finding.Location{
				Path: path,
			}, finding.OutcomeFalse)
		if err != nil {
			return false, fmt.Errorf("create finding: %w", err)
		}
		*findings = append(*findings, *found)
	}

	return true, nil
}
