// Package config loads, validates, merges and edits bam's configuration
// files. It never touches the network or the keychain.
package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// ProjectFile is .bam.yaml, committed with a repository.
type ProjectFile struct {
	Version       int               `yaml:"version"`
	Servers       map[string]Server `yaml:"servers,omitempty"`
	DefaultServer string            `yaml:"default_server,omitempty"`
	Projects      []string          `yaml:"projects,omitempty"`
	Targets       map[string]Target `yaml:"targets,omitempty"`
}

// MachineFile is the per-machine config.yaml, never committed.
type MachineFile struct {
	Version       int               `yaml:"version"`
	DefaultServer string            `yaml:"default_server,omitempty"`
	Servers       map[string]Server `yaml:"servers,omitempty"`
	Targets       map[string]Target `yaml:"targets,omitempty"`
	Repos         map[string]Repo   `yaml:"repos,omitempty"`
	Color         string            `yaml:"color,omitempty"`
	Pager         string            `yaml:"pager,omitempty"`
	Editor        string            `yaml:"editor,omitempty"`
}

type Server struct {
	URL      string   `yaml:"url,omitempty"`
	AuthEnv  string   `yaml:"auth_env,omitempty"`
	Projects []string `yaml:"projects,omitempty"`
}

// Target is a run preset.
type Target struct {
	Plan     string        `yaml:"plan,omitempty"`
	Server   string        `yaml:"server,omitempty"`
	Branch   string        `yaml:"branch,omitempty"`
	Defaults StringMap     `yaml:"defaults,omitempty"`
	Options  StringListMap `yaml:"options,omitempty"`
	Required []string      `yaml:"required,omitempty"`
	Watch    *bool         `yaml:"watch,omitempty"`
	Timeout  Duration      `yaml:"timeout,omitempty"`
}

type Repo struct {
	Server   string            `yaml:"server,omitempty"`
	Projects []string          `yaml:"projects,omitempty"`
	Targets  map[string]Target `yaml:"targets,omitempty"`
}

// StringMap decodes any YAML scalar values as strings; Bamboo variables are strings.
type StringMap map[string]string

func (m *StringMap) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: expected a mapping", n.Line)
	}
	out := StringMap{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if v.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: value of %q must be a single value", v.Line, k.Value)
		}
		out[k.Value] = v.Value
	}
	*m = out
	return nil
}

// StringListMap decodes a mapping of name to a list of scalars.
type StringListMap map[string][]string

func (m *StringListMap) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: expected a mapping", n.Line)
	}
	out := StringListMap{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if v.Kind != yaml.SequenceNode {
			return fmt.Errorf("line %d: options for %q must be a list", v.Line, k.Value)
		}
		vals := make([]string, 0, len(v.Content))
		for _, item := range v.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("line %d: options for %q must be single values", item.Line, k.Value)
			}
			vals = append(vals, item.Value)
		}
		out[k.Value] = vals
	}
	*m = out
	return nil
}

// Duration parses Go duration strings such as "45m".
type Duration time.Duration

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	v, err := time.ParseDuration(n.Value)
	if err != nil {
		return fmt.Errorf("line %d: %q is not a duration like 45m or 1h30m", n.Line, n.Value)
	}
	*d = Duration(v)
	return nil
}

func (d Duration) MarshalYAML() (any, error) { return time.Duration(d).String(), nil }
