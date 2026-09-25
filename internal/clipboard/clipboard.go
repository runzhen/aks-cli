package clipboard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func Write(text string) error {
	return write(text, runtime.GOOS, os.Getenv("WAYLAND_DISPLAY"))
}

func write(text, platform, waylandDisplay string) error {
	var candidates [][]string
	switch platform {
	case "darwin":
		candidates = [][]string{{"pbcopy"}}
	case "linux":
		if waylandDisplay != "" {
			candidates = append(candidates, []string{"wl-copy"})
		}
		candidates = append(candidates, []string{"xclip", "-selection", "clipboard"}, []string{"xsel", "--clipboard", "--input"})
	default:
		return fmt.Errorf("system clipboard is not supported on %s", platform)
	}
	for _, args := range candidates {
		path, err := exec.LookPath(args[0])
		if errors.Is(err, exec.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("find %s: %w", args[0], err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, path, args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		// Clipboard helpers may fork a selection owner that inherits stderr.
		cmd.WaitDelay = 100 * time.Millisecond
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("%s timed out: %w", args[0], ctx.Err())
			}
			return fmt.Errorf("%s failed: %w (%s)", args[0], err, strings.TrimSpace(stderr.String()))
		}
		return nil
	}
	if platform == "darwin" {
		return fmt.Errorf("pbcopy not found; system clipboard requires macOS pbcopy")
	}
	return fmt.Errorf("no clipboard helper found; install wl-clipboard for Wayland or xclip/xsel for X11")
}
