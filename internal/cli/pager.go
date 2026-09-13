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
	if len(fields) == 0 || fields[0] == "cat" {
		_, err := io.Copy(os.Stdout, r)
		return err
	}
	c := exec.Command(fields[0], fields[1:]...)
	c.Stdin, c.Stdout, c.Stderr = r, os.Stdout, os.Stderr
	return c.Run()
}
