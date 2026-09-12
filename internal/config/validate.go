package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/r0jjames/bam-cli/internal/errs"
	"gopkg.in/yaml.v3"
)

var (
	targetNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	envRefRe     = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)
	nonAlnumRe   = regexp.MustCompile(`[^A-Za-z0-9]+`)

	// Bamboo's default list of variable-name fragments it masks.
	maskedFragments = []string{"password", "secret", "passphrase", "sshkey"}

	// Keys that must never appear under servers.<alias> in a committed file.
	credentialKeys = map[string]bool{"token": true, "password": true, "pat": true, "secret": true}
)

// EnvRef reports whether v is exactly ${NAME} and returns NAME.
func EnvRef(v string) (string, bool) {
	m := envRefRe.FindStringSubmatch(v)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// IsMaskedName reports whether Bamboo treats a variable with this name as secret.
func IsMaskedName(name string) bool {
	lower := strings.ToLower(name)
	for _, f := range maskedFragments {
		if strings.Contains(lower, f) {
			return true
		}
	}
	return false
}

// EnvNameFor suggests an environment variable name for a Bamboo variable.
func EnvNameFor(variable string) string {
	return strings.Trim(strings.ToUpper(nonAlnumRe.ReplaceAllString(variable, "_")), "_")
}

// ValidTargetName reports whether name can be a target name. Target names are
// lowercase so they never collide with uppercase plan keys.
func ValidTargetName(name string) bool { return targetNameRe.MatchString(name) }

// checkProjectCredentialKeys fails when a committed file carries a credential.
// It runs on the raw YAML so it reports the key even though strict decoding
// would also reject it.
func checkProjectCredentialKeys(data []byte, path string) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil || len(root.Content) == 0 {
		return nil // decodeStrict reports syntax errors
	}
	servers := mappingValue(root.Content[0], "servers")
	if servers == nil || servers.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(servers.Content); i += 2 {
		alias, body := servers.Content[i].Value, servers.Content[i+1]
		if body.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(body.Content); j += 2 {
			key := body.Content[j].Value
			if credentialKeys[strings.ToLower(key)] {
				return errs.Configf("credential found in %s at servers.%s.%s", path, alias, key).
					WithWhy("tokens never go in a committed file").
					WithTry(fmt.Sprintf("remove the line, revoke that token in Bamboo, then run: bam login %s", alias))
			}
		}
	}
	return nil
}

func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// validateFiles applies the rules that need only one file at a time.
func validateFiles(c *Config) error {
	if c.Project != nil {
		if err := checkTargetNames(c.ProjectPath, "targets", c.Project.Targets); err != nil {
			return err
		}
		for _, name := range sortedKeys(c.Project.Targets) {
			t := c.Project.Targets[name]
			for _, v := range sortedKeys(t.Defaults) {
				val := t.Defaults[v]
				if _, ok := EnvRef(val); ok || val == "" || !IsMaskedName(v) {
					continue
				}
				return errs.Configf("%s: targets.%s.defaults.%s holds a literal secret", c.ProjectPath, name, v).
					WithWhy("variables named like password, secret, passphrase or sshkey cannot be committed").
					WithTry(fmt.Sprintf(`use an environment reference: %s: "${%s}"`, v, EnvNameFor(v)))
			}
		}
	}
	if err := checkTargetNames(c.MachinePath, "targets", c.Machine.Targets); err != nil {
		return err
	}
	for _, root := range sortedKeys(c.Machine.Repos) {
		if err := checkTargetNames(c.MachinePath, "repos."+root+".targets", c.Machine.Repos[root].Targets); err != nil {
			return err
		}
	}
	return nil
}

func checkTargetNames(path, where string, targets map[string]Target) error {
	for _, name := range sortedKeys(targets) {
		if !ValidTargetName(name) {
			return errs.Configf("%s: %s: invalid target name %q", path, where, name).
				WithWhy("target names are lowercase letters, digits, - and _, starting with a letter").
				WithTry(fmt.Sprintf("rename it, for example %q", strings.ToLower(nonAlnumRe.ReplaceAllString(name, "-"))))
		}
	}
	return nil
}

func sortedKeys[M ~map[string]V, V any](m M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
