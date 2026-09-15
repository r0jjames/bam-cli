package config

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/r0jjames/bam-cli/internal/errs"
)

// EnvServerAlias names the ad-hoc server defined by BAM_URL.
const EnvServerAlias = "env"

// ResolvedServer is one server alias after merging both layers.
type ResolvedServer struct {
	Alias     string
	URL       string
	AuthEnv   string
	Token     string // only for the BAM_URL server
	Projects  []string
	DefinedIn []string // "project", "machine", "env"
}

// Servers returns every configured alias, merged field by field with the
// machine layer winning, plus the BAM_URL server when set.
func (c *Config) Servers() map[string]ResolvedServer {
	out := map[string]ResolvedServer{}
	add := func(layer string, servers map[string]Server) {
		for alias, s := range servers {
			r := out[alias]
			r.Alias = alias
			if s.URL != "" {
				r.URL = s.URL
			}
			// A layer's auth_env is applied only when that same layer's
			// entry also sets url. Otherwise a bare "servers.X: {auth_env}"
			// (e.g. a machine entry with no url of its own) could end up
			// paired with a url written by a different layer -- such as a
			// project file -- and send that env var's secret to a host its
			// owner never approved (controller ruling F-R1).
			if s.AuthEnv != "" && s.URL != "" {
				r.AuthEnv = s.AuthEnv
			}
			if len(s.Projects) > 0 {
				r.Projects = s.Projects
			}
			r.DefinedIn = append(r.DefinedIn, layer)
			out[alias] = r
		}
	}
	if c.Project != nil {
		add("project", c.Project.Servers)
	}
	add("machine", c.Machine.Servers)
	if c.Env.URL != "" {
		out[EnvServerAlias] = ResolvedServer{Alias: EnvServerAlias, URL: c.Env.URL, Token: c.Env.Token, DefinedIn: []string{"env"}}
	}
	return out
}

// RepoEntry returns the machine file's repos entry for this repository.
func (c *Config) RepoEntry() (Repo, bool) {
	for key, r := range c.Machine.Repos {
		if c.expand(key) == filepath.Clean(c.RepoRoot) {
			return r, true
		}
	}
	return Repo{}, false
}

func (c *Config) expand(path string) string {
	if strings.HasPrefix(path, "~/") && c.Home != "" {
		path = filepath.Join(c.Home, path[2:])
	}
	return filepath.Clean(path)
}

// SelectServer applies spec §3.5: flag, BAM_SERVER, BAM_URL, target server,
// machine repos entry, project default, machine default, the only server.
func (c *Config) SelectServer(flag, targetServer string) (ResolvedServer, error) {
	servers := c.Servers()
	pick := func(alias string) (ResolvedServer, error) {
		s, ok := servers[alias]
		if !ok || s.URL == "" {
			return ResolvedServer{}, errs.Configf("no server alias %q", alias).
				WithTry(fmt.Sprintf("bam server add %s --url URL", alias))
		}
		return s, nil
	}
	repo, _ := c.RepoEntry()
	projectDefault := ""
	if c.Project != nil {
		projectDefault = c.Project.DefaultServer
	}
	switch {
	case flag != "":
		return pick(flag)
	case c.Env.Server != "":
		return pick(c.Env.Server)
	case c.Env.URL != "":
		return pick(EnvServerAlias)
	case targetServer != "":
		return pick(targetServer)
	case repo.Server != "":
		return pick(repo.Server)
	case projectDefault != "":
		return pick(projectDefault)
	case c.Machine.DefaultServer != "":
		return pick(c.Machine.DefaultServer)
	}
	aliases := sortedKeys(servers)
	switch len(aliases) {
	case 1:
		return servers[aliases[0]], nil
	case 0:
		return ResolvedServer{}, errs.Configf("no Bamboo server configured").
			WithTry("bam server add <alias> --url URL")
	}
	return ResolvedServer{}, errs.Configf("no server selected").
		WithWhy("configured aliases: " + strings.Join(aliases, ", ")).
		WithTry("pass --server ALIAS, or set default_server in .bam.yaml")
}

// ProjectKeys returns the projects that scope navigation for server s.
func (c *Config) ProjectKeys(s ResolvedServer) []string {
	if repo, ok := c.RepoEntry(); ok && len(repo.Projects) > 0 {
		return repo.Projects
	}
	if c.Project != nil && len(c.Project.Projects) > 0 {
		return c.Project.Projects
	}
	return s.Projects
}

// ResolvedTarget is a target after merging every layer that defines it.
type ResolvedTarget struct {
	Name string
	Target
	DefinedIn []string // file paths, lowest precedence first
}

// Targets merges targets by name: project, then machine top-level, then the
// machine repos entry for this repository. It validates the merged result.
func (c *Config) Targets() (map[string]ResolvedTarget, error) {
	out := map[string]ResolvedTarget{}
	layer := func(path string, targets map[string]Target) {
		for name, t := range targets {
			r := out[name]
			r.Name = name
			r.Target = mergeTarget(r.Target, t)
			r.DefinedIn = append(r.DefinedIn, path)
			out[name] = r
		}
	}
	if c.Project != nil {
		layer(c.ProjectPath, c.Project.Targets)
	}
	layer(c.MachinePath, c.Machine.Targets)
	if repo, ok := c.RepoEntry(); ok {
		layer(c.MachinePath+" (repos)", repo.Targets)
	}
	servers := c.Servers()
	for _, name := range sortedKeys(out) {
		if err := validateTarget(out[name], servers); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Target returns one merged target.
func (c *Config) Target(name string) (ResolvedTarget, bool, error) {
	all, err := c.Targets()
	if err != nil {
		return ResolvedTarget{}, false, err
	}
	t, ok := all[name]
	return t, ok, nil
}

func mergeTarget(base, over Target) Target {
	if over.Plan != "" {
		base.Plan = over.Plan
	}
	if over.Server != "" {
		base.Server = over.Server
	}
	if over.Branch != "" {
		base.Branch = over.Branch
	}
	if len(over.Defaults) > 0 {
		merged := StringMap{}
		for k, v := range base.Defaults {
			merged[k] = v
		}
		for k, v := range over.Defaults {
			merged[k] = v
		}
		base.Defaults = merged
	}
	if len(over.Options) > 0 {
		merged := StringListMap{}
		for k, v := range base.Options {
			merged[k] = v
		}
		for k, v := range over.Options {
			merged[k] = v
		}
		base.Options = merged
	}
	if over.Required != nil {
		base.Required = over.Required
	}
	if over.Watch != nil {
		base.Watch = over.Watch
	}
	if over.Timeout != 0 {
		base.Timeout = over.Timeout
	}
	return base
}

func validateTarget(t ResolvedTarget, servers map[string]ResolvedServer) error {
	where := strings.Join(t.DefinedIn, ", ")
	if t.Plan == "" {
		return errs.Configf("target %q has no plan", t.Name).
			WithWhy("defined in " + where).
			WithTry(fmt.Sprintf("add plan: PROJ-PLAN under targets.%s", t.Name))
	}
	if t.Server != "" {
		if _, ok := servers[t.Server]; !ok {
			return errs.Configf("target %q uses unknown server %q", t.Name, t.Server).
				WithTry(fmt.Sprintf("bam server add %s --url URL", t.Server))
		}
	}
	for _, name := range sortedKeys(t.Options) {
		val, ok := t.Defaults[name]
		if !ok || val == "" {
			continue
		}
		if _, isRef := EnvRef(val); isRef {
			continue
		}
		if !slices.Contains(t.Options[name], val) {
			display := val
			if IsMaskedName(name) {
				display = "********"
			}
			return errs.Configf("target %q: default %s=%q is not one of %s", t.Name, name, display, strings.Join(t.Options[name], ", ")).
				WithWhy("defined in " + where)
		}
	}
	return nil
}
