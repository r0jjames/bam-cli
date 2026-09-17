// Package bamboo implements provider.Provider for Atlassian Bamboo Data
// Center 9.x+ over its REST API.
package bamboo

import (
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	urlHostRe = regexp.MustCompile(`(https?://)([^/"'\s<>]+)`)
	// hostOnlyRe matches the host after ANY scheme://, not only http(s), and
	// stops before a bracketed IPv6 literal so that is handled separately by
	// ipv6BracketRe below.
	hostOnlyRe    = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://([^/"'\s<>:\[\]]+)`)
	ipv6BracketRe = regexp.MustCompile(`\[[0-9A-Fa-f:]+\]`)
	emailRe       = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	ipv4Re        = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	placeholder   = "bamboo.example.com"
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
	// Names maps a real project key, project name or repository name to
	// the placeholder it is recorded as (for example FORGE -> LAB). The
	// repository is public, so recordings carry placeholder keys only.
	Names map[string]string
}

// Scrub rewrites every URL host to bamboo.example.com, bare mentions of the
// recorded host (case-insensitive), e-mail addresses, and the listed user names.
func (s Scrubber) Scrub(data []byte) []byte {
	out := string(data)
	out = urlHostRe.ReplaceAllString(out, "${1}"+placeholder)

	// Project, plan and repository names first, longest match first, so
	// "forge-lab" is replaced before the "FORGE" inside it.
	for _, name := range sortedByLength(mapKeys(s.Names)) {
		re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(name))
		out = re.ReplaceAllString(out, s.Names[name])
	}

	// Build list of terms to redact, in order (longest first for host, then users)
	terms := s.Terms()
	for _, term := range terms {
		if term == "" {
			continue
		}
		// Case-insensitive replacement using regex
		re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(term))
		out = re.ReplaceAllString(out, "jdoe")
	}

	out = emailRe.ReplaceAllString(out, "jdoe@example.com")
	return []byte(out)
}

// Terms returns the distinct non-empty terms that Scrub redacts: Host, host without port, and each user name.
func (s Scrubber) Terms() []string {
	terms := make(map[string]bool)

	// Add full host with port
	if s.Host != "" {
		terms[s.Host] = true
	}

	// Add host without port (if Host contains a port)
	if s.Host != "" {
		host, _, err := net.SplitHostPort(s.Host)
		if err == nil && host != "" {
			// SplitHostPort succeeded, so there was a port
			terms[host] = true
		}
	}

	// Add users (non-empty)
	for _, u := range s.Users {
		if u != "" {
			terms[u] = true
		}
	}

	// Convert to slice and sort by length (longest first)
	var result []string
	for term := range terms {
		result = append(result, term)
	}
	sort.Slice(result, func(i, j int) bool { return len(result[i]) > len(result[j]) })
	return result
}

// DenylistTerms returns everything a recording must never contain: the
// terms Scrub redacts plus the real names it replaces with placeholders.
// The recorder writes them to the private denylist so check-fixtures can
// fail on anything a future recording leaks.
func (s Scrubber) DenylistTerms() []string {
	return sortedByLength(append(s.Terms(), mapKeys(s.Names)...))
}

func mapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sortedByLength(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, v := range in {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

// FixtureHostViolations lists "file: host" for every URL host under root that
// is not in AllowedFixtureHosts, and for IPv4 addresses except 127.0.0.1.
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
		content := string(data)

		// Check for disallowed URL hosts, after any scheme, not only http(s).
		for _, m := range hostOnlyRe.FindAllStringSubmatch(content, -1) {
			if !AllowedFixtureHosts[strings.ToLower(m[1])] {
				out = append(out, path+": "+m[1])
			}
		}

		// Check for IPv6 literals in brackets, wherever they appear, except
		// the loopback address. The bracket regex is a loose candidate
		// match (it also matches non-IPv6 things like an expand range
		// "[0:50]" or a bare index "[1]"), so confirm each candidate with
		// net.ParseIP before flagging it.
		for _, m := range ipv6BracketRe.FindAllString(content, -1) {
			if m == "[::1]" {
				continue
			}
			candidate := m[1 : len(m)-1]
			if net.ParseIP(candidate) == nil {
				continue
			}
			out = append(out, path+": "+m)
		}

		// Check for IPv4 addresses (except 127.0.0.1)
		for _, ip := range ipv4Re.FindAllString(content, -1) {
			if ip != "127.0.0.1" {
				out = append(out, path+": "+ip)
			}
		}

		return nil
	})
	return out, err
}

// LoadDenylist loads terms from a file, one per line, skipping blank lines and lines starting with '#'.
// Returns nil if the file doesn't exist.
func LoadDenylist(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var terms []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			terms = append(terms, line)
		}
	}
	return terms, nil
}

// FixtureTermViolations walks files under root and reports "path: term" for every term found case-insensitively.
func FixtureTermViolations(root string, terms []string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := strings.ToLower(string(data))

		for _, term := range terms {
			if strings.Contains(content, strings.ToLower(term)) {
				out = append(out, path+": "+term)
			}
		}
		return nil
	})
	return out, err
}

// DenylistPath returns the path to the fixture denylist file.
// Uses BAM_FIXTURE_DENYLIST env var if set, otherwise returns the default path
// under the user's config directory.
func DenylistPath(getenv func(string) string) string {
	if path := getenv("BAM_FIXTURE_DENYLIST"); path != "" {
		return path
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "bam", "fixture-denylist.txt")
}
