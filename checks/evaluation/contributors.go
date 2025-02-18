// Copyright 2021 OpenSSF Scorecard Authors
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

package evaluation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ossf/scorecard/v5/checker"
	sce "github.com/ossf/scorecard/v5/errors"
	"github.com/ossf/scorecard/v5/finding"
	"github.com/ossf/scorecard/v5/probes/contributorsFromOrgOrCompany"
)

const (
	numberCompaniesForTopScore = 3
)

// Contributors applies the score policy for the Contributors check.
func Contributors(name string,
	findings []finding.Finding,
	dl checker.DetailLogger,
) checker.CheckResult {
	expectedProbes := []string{
		contributorsFromOrgOrCompany.Probe,
	}

	if !finding.UniqueProbesEqual(findings, expectedProbes) {
		e := sce.WithMessage(sce.ErrScorecardInternal, "invalid probe results")
		return checker.CreateRuntimeErrorResult(name, e)
	}

	numberOfTrue := getNumberOfTrue(findings)
	reason := fmt.Sprintf("project has %d contributing companies or organizations", numberOfTrue)

	if numberOfTrue > 0 {
		logFindings(findings, dl)
	}
	if numberOfTrue > numberCompaniesForTopScore {
		return checker.CreateMaxScoreResult(name, reason)
	}

	return checker.CreateProportionalScoreResult(name, reason, numberOfTrue, numberCompaniesForTopScore)
}

func getNumberOfTrue(findings []finding.Finding) int {
	var numberOfTrue int
	for i := range findings {
		f := &findings[i]
		if f.Outcome == finding.OutcomeTrue {
			if f.Probe == contributorsFromOrgOrCompany.Probe {
				numberOfTrue++
			}
		}
	}
	return numberOfTrue
}

func logFindings(findings []finding.Finding, dl checker.DetailLogger) {
	var orgs []string
	const suffix = " contributor org/company found"
	for i := range findings {
		f := &findings[i]
		if f.Outcome == finding.OutcomeTrue {
			org := strings.TrimSuffix(f.Message, suffix)
			orgs = append(orgs, org)
		}
	}
	slices.Sort(orgs)
	dl.Info(&checker.LogMessage{
		Text: "found contributions from: " + strings.Join(orgs, ", "),
	})
}
