package app

import (
	"os"
	"os/exec"
	"testing"
)

// TestPlaythroughSeed1 runs the fixed-seed playthrough in a child process (it exits on failure and owns the audio
// context) and diffs its summary against testdata. A deliberate gameplay change regenerates the file:
// go run ./src playthrough 1 > src/app/testdata/playthrough_seed1.txt
func TestPlaythroughSeed1(t *testing.T) {
	if os.Getenv("HH_PLAYTHROUGH_CHILD") != "" {
		os.Args = []string{os.Args[0], "playthrough", "1"}
		playthrough()
		os.Exit(0)
	}
	if testing.Short() {
		t.Skip("the playthrough takes a while")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPlaythroughSeed1$")
	cmd.Env = append(os.Environ(), "HH_PLAYTHROUGH_CHILD=1")
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("playthrough: %v\n%s", err, got)
	}
	want, err := os.ReadFile("src/app/testdata/playthrough_seed1.txt") // TestMain runs from the repo root
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("playthrough seed 1 drifted from testdata/playthrough_seed1.txt:\n%s", got)
	}
}
