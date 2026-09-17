package cli

import (
	"io"
	"os"
	"os/exec"
	"strings"
)

// runPager pipes r through cmd, or copies it to stdout when cmd is empty or "cat".
func runPager(cmd string, r io.Reader) error {
	fields := strings.Fields(cmd)
	// Only bare "cat" is short-circuited: "cat -n" and friends carry
	// arguments that must reach the real command.
	if len(fields) == 0 || (len(fields) == 1 && fields[0] == "cat") {
		_, err := io.Copy(os.Stdout, r)
		return err
	}
	c := exec.Command(fields[0], fields[1:]...)
	c.Stdin, c.Stdout, c.Stderr = r, os.Stdout, os.Stderr
	return c.Run()
}
