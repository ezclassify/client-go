package ezclassify

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The vendor-readonly workflow is the only thing standing between spec/ and a
// hand edit, and a workflow has no unit test of its own. A gate can print
// "inspecting ONLY the pushed head commit" and then report on, and pass, a
// commit that was not in that push: given a pathspec, `git log -1 <sha> --
// 'spec/'` WALKS to the nearest ancestor touching that path.
//
// Two tests below, and they check different things on purpose. The shape test
// pins the specific constructs whose absence reintroduces a known defect. The
// behavior test extracts the workflow's literal shell body and runs it against
// real git history, so a rewrite that keeps the words but loses the property
// still fails.

const vendorWorkflowPath = ".github/workflows/vendor-readonly.yml"

// workflowRunBody extracts the single `run: |` block scalar from the
// vendor-readonly workflow and returns it dedented, exactly as the runner would
// hand it to bash.
//
// Deliberately strict: if the workflow ever grows a second `run:` step this
// fails rather than silently testing whichever one it found first, because a
// test that inspects the wrong half of a gate is worse than no test.
func workflowRunBody(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(vendorWorkflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", vendorWorkflowPath, err)
	}
	lines := strings.Split(string(raw), "\n")

	var bodies []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "run: |" {
			continue
		}
		keyIndent := len(line) - len(strings.TrimLeft(line, " "))
		var block []string
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "" {
				block = append(block, "")
				continue
			}
			indent := len(next) - len(strings.TrimLeft(next, " "))
			if indent <= keyIndent {
				break
			}
			block = append(block, next)
		}
		// Dedent by the block's own minimum indent.
		minIndent := -1
		for _, blockLine := range block {
			if blockLine == "" {
				continue
			}
			indent := len(blockLine) - len(strings.TrimLeft(blockLine, " "))
			if minIndent < 0 || indent < minIndent {
				minIndent = indent
			}
		}
		for j, blockLine := range block {
			if len(blockLine) >= minIndent {
				block[j] = blockLine[minIndent:]
			}
		}
		bodies = append(bodies, strings.Join(block, "\n"))
	}

	if len(bodies) != 1 {
		t.Fatalf("expected exactly 1 `run: |` block in %s, found %d", vendorWorkflowPath, len(bodies))
	}
	if !strings.Contains(bodies[0], "set -euo pipefail") {
		t.Fatalf("extracted run body does not look like the gate script:\n%s", bodies[0])
	}
	return bodies[0]
}

func TestVendorGateShape(t *testing.T) {
	raw, err := os.ReadFile(vendorWorkflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", vendorWorkflowPath, err)
	}
	workflow := string(raw)
	body := workflowRunBody(t)

	t.Run("fires on push as well as pull_request", func(t *testing.T) {
		// Spec commits usually land by direct push. With `pull_request` alone
		// the gate would rarely observe a real spec commit.
		for _, trigger := range []string{"pull_request", "push"} {
			pattern := regexp.MustCompile(`(?m)^ {2}` + trigger + `:\n(?: {4}.*\n| *\n)* {4}paths:\n(?: {6}- .*\n)+`)
			block := pattern.FindString(workflow)
			if block == "" {
				t.Fatalf("no `%s:` trigger with a paths list in %s", trigger, vendorWorkflowPath)
			}
			if !strings.Contains(block, "- 'spec/**'") {
				t.Errorf("`%s:` trigger does not watch spec/**:\n%s", trigger, block)
			}
		}
	})

	t.Run("reads the head commit with diff-tree, never a walking log", func(t *testing.T) {
		// `git log -1 <sha> -- <paths>` walks past <sha> to the nearest
		// ancestor that touched <paths>. `diff-tree` asks only what this one
		// commit changed.
		if !strings.Contains(body, `git diff-tree --no-commit-id --name-only -r "$PUSH_AFTER" -- 'spec/'`) {
			t.Error("single-commit lookup must use `git diff-tree --no-commit-id --name-only -r`")
		}
		// Comment lines are excluded: the workflow explains the trap in prose
		// directly above the fix, and matching that would make this assertion
		// fire on its own documentation.
		walking := regexp.MustCompile(`git log -1[^\n]*\s--\s`)
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			if match := walking.FindString(line); match != "" {
				t.Errorf("`git log -1` with a pathspec walks to an ancestor; found: %q", match)
			}
		}
	})

	t.Run("guards both base resolutions with --is-ancestor", func(t *testing.T) {
		// A `!=` string test lets `origin/<default>..HEAD` resolve EMPTY
		// whenever HEAD is already contained in the default branch, and the
		// gate then passes having inspected nothing.
		guards := []string{
			`git merge-base --is-ancestor "$PUSH_BEFORE" "$PUSH_AFTER"`,
			`! git merge-base --is-ancestor "$PUSH_AFTER" "$DEFAULT_REF"`,
		}
		for _, guard := range guards {
			if !strings.Contains(body, guard) {
				t.Errorf("missing base-resolution guard: %s", guard)
			}
		}
	})

	t.Run("refuses rather than passing on an empty commit set", func(t *testing.T) {
		// Isolate the block that handles "no commits touch spec/" and require
		// that it can exit 1. Asserting on the whole file would pass on any
		// `exit 1` anywhere.
		start := strings.Index(body, `if [ -z "$VENDOR_COMMITS" ]; then`)
		if start < 0 {
			t.Fatal("no empty-commit-set branch in the gate body")
		}
		end := strings.Index(body[start:], "\nfi\n")
		if end < 0 {
			t.Fatal("empty-commit-set branch is not terminated")
		}
		block := body[start : start+end]
		if !strings.Contains(block, "exit 1") {
			t.Errorf("empty commit set exits 0 unconditionally:\n%s", block)
		}
	})

	t.Run("refusals name the spec bot and the contracts source", func(t *testing.T) {
		// A blocked contributor needs to know who writes the mirror and where
		// the spec comes from.
		for _, want := range []string{"the EzClassify spec bot", "ezclassify/contracts"} {
			if !strings.Contains(workflow, want) {
				t.Errorf("workflow does not name %q", want)
			}
		}
	})
}

// gitRepo is a throwaway repository the gate script is driven against.
type gitRepo struct {
	t   *testing.T
	dir string
}

// git runs a git command in the scratch repository and returns its trimmed
// stdout.
func (r *gitRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// commit writes content to a path and commits it under the given author.
func (r *gitRepo) commit(name, email, subject, path, content string) string {
	r.t.Helper()
	full := filepath.Join(r.dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
	r.git("add", path)
	r.git(
		"-c", "user.name="+name,
		"-c", "user.email="+email,
		"commit", "-q", "-m", subject,
	)
	return r.git("rev-parse", "HEAD")
}

const (
	botName  = "ezclassify-spec-bot[bot]"
	botEmail = "ezclassify-spec-bot[bot]@users.noreply.github.com"
	zeroSHA  = "0000000000000000000000000000000000000000"
)

func TestVendorGateBehavior(t *testing.T) {
	// bash and git are already required to work in this repo (the Makefile
	// shells out to scripts/regen.sh), so a missing one is a broken checkout,
	// not a reason to skip. Skipping is how a gate goes quietly inert.
	for _, tool := range []string{"bash", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("%s is required to exercise the vendor gate: %v", tool, err)
		}
	}

	script := filepath.Join(t.TempDir(), "gate.sh")
	if err := os.WriteFile(script, []byte(workflowRunBody(t)), 0o755); err != nil {
		t.Fatal(err)
	}

	repo := &gitRepo{t: t, dir: t.TempDir()}
	repo.git("init", "-q", "-b", "main")

	base := repo.commit("Someone", "someone@example.com", "chore: seed", "README.md", "seed\n")
	botSpec := repo.commit(botName, botEmail,
		"chore(spec): vendor spec bundle from contracts@aaaaaaaa",
		"spec/SPEC.md", "one\n")
	nonSpec := repo.commit("Someone", "someone@example.com",
		"Widen the push gate", "client.go", "package x\n")
	// The default branch the fallback resolves against.
	repo.git("update-ref", "refs/remotes/origin/main", nonSpec)

	humanSpec := repo.commit("Someone", "someone@example.com",
		"Cut the removed license surface from the vendored spec",
		"spec/SPEC.md", "hand edited\n")
	badSubject := repo.commit(botName, botEmail,
		"feat: smuggle content through the spec channel",
		"spec/SPEC.md", "smuggled\n")
	forgedRevert := repo.commit("Someone", "someone@example.com",
		`Revert "chore(spec): vendor spec bundle from contracts@aaaaaaaa"`,
		"spec/SPEC.md", "novel content wearing a revert title\n")
	// A true rollback: restores the exact spec/ tree the bot commit produced.
	realRollback := repo.commit("Someone", "someone@example.com",
		`Revert "chore(spec): vendor spec bundle from contracts@aaaaaaaa"`,
		"spec/SPEC.md", "one\n")

	// A bot commit on a branch that origin/main does not contain, which is
	// exactly what the spec bot pushes.
	repo.git("checkout", "-q", "-b", "chore/spec-update", nonSpec)
	botOnBranch := repo.commit(botName, botEmail,
		"chore(spec): vendor spec bundle from contracts@bbbbbbbb",
		"spec/SPEC.md", "two\n")

	cases := []struct {
		name        string
		event       string
		before      string
		after       string
		prBase      string
		prHead      string
		prAuthor    string
		wantExit    int
		wantOutput  []string
		denyOutput  []string
		explanation string
	}{
		{
			name: "bot commit with a usable push base passes", event: "push",
			before: base, after: botSpec, wantExit: 0,
			wantOutput: []string{botSpec, "pass identity + subject gates"},
		},
		{
			name: "bot commit passes when the push base is all zeros", event: "push",
			before: zeroSHA, after: botSpec, wantExit: 0,
			wantOutput: []string{"inspecting ONLY the pushed head commit", botSpec},
			explanation: "branch creation must not hard-fail; the spec bot " +
				"creates a branch and pushes it",
		},
		{
			name:  "does not walk to an ancestor when the head touches no spec path",
			event: "push", before: zeroSHA, after: nonSpec, wantExit: 1,
			wantOutput: []string{"Refusing to pass"},
			denyOutput: []string{botSpec},
			explanation: "`git log -1 <head> -- spec/` would return the " +
				"ancestor botSpec here and the gate would pass on it",
		},
		{
			name:  "refuses a rollback force-push whose base is not an ancestor",
			event: "push", before: botOnBranch, after: nonSpec, wantExit: 1,
			wantOutput: []string{"Refusing to pass"},
			denyOutput: []string{botSpec},
		},
		{
			name: "refuses a human hand edit of spec/", event: "push",
			before: nonSpec, after: humanSpec, wantExit: 1,
			wantOutput: []string{"not authored by the spec bot", humanSpec},
		},
		{
			name: "refuses a bot-authored commit with a foreign subject", event: "push",
			before: humanSpec, after: badSubject, wantExit: 1,
			wantOutput: []string{"disallowed subject", badSubject},
		},
		{
			name: "refuses a forged revert title carrying novel content", event: "push",
			before: badSubject, after: forgedRevert, wantExit: 1,
			wantOutput: []string{"not authored by the spec bot", forgedRevert},
			explanation: "the rollback allowance keys on tree identity, so a " +
				"revert-shaped subject alone buys nothing",
		},
		{
			name: "allows a rollback that restores a bot-authored spec tree", event: "push",
			before: forgedRevert, after: realRollback, wantExit: 0,
			wantOutput: []string{"rollback allowed"},
		},
		{
			name:  "passes a branch-creation push of a bot commit via the fallback range",
			event: "push", before: zeroSHA, after: botOnBranch, wantExit: 0,
			wantOutput: []string{"Falling back to", botOnBranch},
		},
		{
			name: "refuses a pull request that a human opened", event: "pull_request",
			prBase: nonSpec, prHead: humanSpec, prAuthor: "some-contributor", wantExit: 1,
			wantOutput: []string{"not 'ezclassify-spec-bot[bot]'"},
		},
		{
			name: "passes a range with no spec commits at all", event: "push",
			before: botSpec, after: nonSpec, wantExit: 0,
			wantOutput: []string{"nothing to check"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cmd := exec.Command("bash", script)
			cmd.Dir = repo.dir
			cmd.Env = append(os.Environ(),
				"GIT_CONFIG_GLOBAL=/dev/null",
				"GIT_CONFIG_SYSTEM=/dev/null",
				"EVENT_NAME="+testCase.event,
				"PUSH_BEFORE="+testCase.before,
				"PUSH_AFTER="+testCase.after,
				"BASE_SHA="+testCase.prBase,
				"HEAD_SHA="+testCase.prHead,
				"PR_AUTHOR="+testCase.prAuthor,
				"PR_LABELS=",
				"DEFAULT_BRANCH=main",
				"ROLLBACK_SEARCH_DEPTH=50",
				"ALLOWED_BOT_LOGIN="+botName,
				"ALLOWED_BOT_EMAIL="+botEmail,
				"OVERRIDE_LABEL=spec-mirror-override",
			)
			combined, runErr := cmd.CombinedOutput()
			exitCode := 0
			if runErr != nil {
				exitError, ok := runErr.(*exec.ExitError)
				if !ok {
					t.Fatalf("run gate: %v\n%s", runErr, combined)
				}
				exitCode = exitError.ExitCode()
			}
			output := string(combined)

			context := ""
			if testCase.explanation != "" {
				context = fmt.Sprintf("\n(%s)", testCase.explanation)
			}
			if exitCode != testCase.wantExit {
				t.Fatalf("exit = %d, want %d%s\n%s", exitCode, testCase.wantExit, context, output)
			}
			for _, want := range testCase.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q%s\n%s", want, context, output)
				}
			}
			for _, deny := range testCase.denyOutput {
				if strings.Contains(output, deny) {
					t.Errorf("output names %q, which was not in this push%s\n%s", deny, context, output)
				}
			}
		})
	}
}
