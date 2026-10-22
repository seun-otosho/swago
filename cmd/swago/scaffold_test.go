package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestScaffoldBuildsAndMigrates scaffolds a project against this checkout (via a replace
// directive, since swago is not published), then proves the output is not just
// well-formed but actually compiles, vets and migrates: go generate, go build, go vet and
// `go run . migrate` against a throwaway SQLite file.
func TestScaffoldBuildsAndMigrates(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "demo")

	if err := scaffold(dir, "demo", repoRoot); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	run := func(name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return string(out)
	}

	run("go", "generate", "./ent/...")
	run("go", "build", "./...")
	run("go", "vet", "./...")

	cmd := exec.Command("go", "run", ".", "migrate")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "DATABASE_URL=sqlite://file:"+filepath.Join(dir, "demo.db")+"?mode=rwc")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("migrate: %v\n%s", err, out)
	}
}
