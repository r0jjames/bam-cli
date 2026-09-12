// Package bamboo implements provider.Provider for Atlassian Bamboo Data
// Center 9.x+ over its REST API.
package bamboo

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	urlHostRe   = regexp.MustCompile(`(https?://)([^/"'\s<>]+)`)
	hostOnlyRe  = regexp.MustCompile(`https?://([^/"'\s<>:]+)`)
	emailRe     = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	placeholder = "bamboo.example.com"
)

// AllowedFixtureHosts are the only hosts that may appear in testdata.
var AllowedFixtureHosts = map[string]bool{
	"bamboo.example.com": true,
	"example.com":        true,
	"localhost":          true,
	"127.0.0.1":          true,
}

// Scrubber removes identifying values from recorded Bamboo responses.
type Scrubber struct {
	Host  string   // host[:port] of the recorded server
	Users []string // user names and full names to replace with jdoe
}

// Scrub rewrites every URL host to bamboo.example.com, bare mentions of the
// recorded host, e-mail addresses, and the listed user names.
func (s Scrubber) Scrub(data []byte) []byte {
	out := string(data)
	out = urlHostRe.ReplaceAllString(out, "${1}"+placeholder)
	if s.Host != "" {
		out = strings.ReplaceAll(out, s.Host, placeholder)
	}
	out = emailRe.ReplaceAllString(out, "jdoe@example.com")
	users := append([]string(nil), s.Users...)
	sort.Slice(users, func(i, j int) bool { return len(users[i]) > len(users[j]) }) // longest first
	for _, u := range users {
		if u != "" {
			out = strings.ReplaceAll(out, u, "jdoe")
		}
	}
	return []byte(out)
}

// FixtureHostViolations lists "file: host" for every URL host under root that
// is not in AllowedFixtureHosts.
func FixtureHostViolations(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range hostOnlyRe.FindAllStringSubmatch(string(data), -1) {
			if !AllowedFixtureHosts[strings.ToLower(m[1])] {
				out = append(out, path+": "+m[1])
			}
		}
		return nil
	})
	return out, err
}
