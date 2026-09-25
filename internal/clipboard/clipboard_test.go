package clipboard

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func helper(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
}

func TestClipboardHelpers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("clipboard helper fixtures require a POSIX shell")
	}
	for _, tc := range []struct {
		name, platform, wayland, args string
	}{
		{"pbcopy", "darwin", "", ""},
		{"wl-copy", "linux", "wayland-0", ""},
		{"xclip", "linux", "", "-selection\nclipboard\n"},
		{"xsel", "linux", "", "--clipboard\n--input\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PATH", dir)
			t.Setenv("COPY_OUTPUT", filepath.Join(dir, "content"))
			t.Setenv("COPY_ARGS", filepath.Join(dir, "args"))
			helper(t, dir, tc.name, `/bin/cat > "$COPY_OUTPUT"
for arg in "$@"; do printf '%s\n' "$arg"; done > "$COPY_ARGS"`)
			text := "az aks create \\\n  --name 'a; $(not-executed)' \\\n  --subscription \"team\"\n"
			if err := write(text, tc.platform, tc.wayland); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(os.Getenv("COPY_OUTPUT"))
			if err != nil || string(content) != text {
				t.Fatalf("clipboard bytes changed: %q, %v", content, err)
			}
			args, err := os.ReadFile(os.Getenv("COPY_ARGS"))
			if err != nil || string(args) != tc.args {
				t.Fatalf("clipboard helper arguments: %q, %v", args, err)
			}
		})
	}
}

func TestClipboardErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	for _, platform := range []string{"darwin", "linux", "unsupported"} {
		if err := write("data", platform, ""); err == nil {
			t.Fatalf("no clipboard backend on %s returned success", platform)
		}
	}
	helper(t, dir, "pbcopy", "echo 'clipboard unavailable' >&2\nexit 1")
	if err := write("data", "darwin", ""); err == nil || !strings.Contains(err.Error(), "clipboard unavailable") {
		t.Fatalf("helper error not surfaced: %v", err)
	}
}

func TestClipboardTimeout(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	helper(t, dir, "pbcopy", "exec /bin/sleep 10")
	if err := write("data", "darwin", ""); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("blocked clipboard helper did not time out: %v", err)
	}
}
