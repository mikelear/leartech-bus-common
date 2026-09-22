// Package ci holds tests about how this repository is BUILT, rather than what
// it does at runtime. There is no production code here.
//
// These exist because this repo went a long time with no Go lint and no
// comment gate, and the cost was invisible: pkg/maestro/audience.go carries a
// dated measurement that is now wrong, and nothing could have caught it.
// Wiring the gates in fixes today. These tests are what stops them being
// quietly unwired again.
package ci

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

const (
	makefile     = "../Makefile"
	golangciYAML = "../.golangci.yml"
	triggersYAML = "../.lighthouse/jenkins-x/triggers.yaml"
)

// A bare `golangci-lint run` resolves config by golangci's own discovery. The
// pipeline runs the yq-merged estate base instead, so the local target has to
// delegate to the same golden mk or the two drift apart.
func TestLintDelegatesToTheGoldenMakefile(t *testing.T) {
	src := read(t, makefile)

	m := regexp.MustCompile(`(?m)^lint:.*\n((?:\t.*\n)+)`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("no lint target found in the Makefile; this test reads a target that " +
			"has moved, so a pass would mean nothing")
	}
	recipe := m[1]

	if !strings.Contains(recipe, "$(LEARTECH_GO_MK)") {
		t.Errorf("the lint target no longer delegates to the golden mk:\n%s", recipe)
	}
	if regexp.MustCompile(`(?m)^\tgolangci-lint run\s*$`).MatchString(recipe) {
		t.Errorf("lint is a bare `golangci-lint run` again, which does not use the " +
			"merged estate config the pipeline builds")
	}
}

// comment-gate is the gate this repo was missing. verify is the one command a
// developer is told to run, so comment-gate has to be reachable from it.
func TestVerifyReachesTheCommentGate(t *testing.T) {
	src := read(t, makefile)

	m := regexp.MustCompile(`(?m)^verify:([^\n#]*)`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("no verify target in the Makefile")
	}
	if !strings.Contains(m[1], "comment-gate") {
		t.Errorf("verify does not depend on comment-gate (deps: %q). A local gate that "+
			"omits the check CI runs teaches people the local gate cannot be trusted.", m[1])
	}
}

// require-committed exists because comment-gate diffs COMMITTED work: against
// a dirty tree it reports +0/+0 and passes having read nothing.
func TestCommentGateRefusesToRunAgainstADirtyTree(t *testing.T) {
	src := read(t, makefile)
	m := regexp.MustCompile(`(?m)^comment-gate:([^\n#]*)`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("no comment-gate target in the Makefile")
	}
	if !strings.Contains(m[1], "require-committed") {
		t.Errorf("comment-gate does not depend on require-committed (deps: %q), so it can "+
			"run against uncommitted work and pass having examined nothing", m[1])
	}
}

// The revive stutter exclusions are a concession to a published API, not a
// decision to stop checking. Scoped to the two packages carrying the legacy
// names, a NEW stuttering export anywhere else still fails.
func TestStutterExclusionsStayScopedToTheLegacyPackages(t *testing.T) {
	src := read(t, golangciYAML)

	rules := regexp.MustCompile(`(?m)^\s*-\s*path:\s*'([^']+)'\n\s*text:\s*'stutters'`).FindAllStringSubmatch(src, -1)
	if len(rules) == 0 {
		t.Fatal("no scoped stutter exclusion found in .golangci.yml; this test reads a " +
			"rule shape that has moved, so a pass would mean nothing")
	}

	allowed := map[string]bool{"pkg/mongo/": true, "pkg/redis/": true}
	for _, r := range rules {
		if !allowed[r[1]] {
			t.Errorf("stutter exclusion widened to %q. Renaming an exported type in a "+
				"shared library breaks consumers, which is why pkg/mongo and pkg/redis are "+
				"excused — that reason does not extend to new code.", r[1])
		}
	}

	if regexp.MustCompile(`(?m)^\s*-\s*text:\s*'stutters'\s*\n\s*linters:`).MatchString(src) {
		t.Error("there is an unscoped 'stutters' exclusion, which switches the rule off " +
			"for the whole repository rather than for the published names")
	}
}

// A presubmit registered as optional reports its result and blocks nothing.
func TestLintIsARequiredPresubmit(t *testing.T) {
	src := read(t, triggersYAML)

	m := regexp.MustCompile(`(?s)- name: lint\n(.*?)(?:\n  - name:|\n  postsubmits:)`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("no lint presubmit registered in triggers.yaml, so the lint pipeline " +
			"never runs on a PR no matter what lint.yaml contains")
	}
	block := m[1]

	if !strings.Contains(block, "always_run: true") {
		t.Errorf("the lint presubmit is not always_run:\n%s", block)
	}
	if strings.Contains(block, "optional: true") {
		t.Errorf("the lint presubmit is optional, so it reports a result and blocks "+
			"nothing:\n%s", block)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unable to read %s: %v", path, err)
	}
	return string(b)
}
