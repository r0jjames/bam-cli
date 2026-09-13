package app

import (
	"time"

	"github.com/r0jjames/bam-cli/internal/config"
	"github.com/r0jjames/bam-cli/internal/errs"
)

// TargetVar is one default of a target, safe to display.
type TargetVar struct {
	Name   string
	Value  string // literal, or the ${NAME} reference as written
	EnvRef string // NAME when Value is ${NAME}
	EnvSet bool
	Secret bool
}

// Display never reveals a secret literal or an environment value.
func (v TargetVar) Display() string {
	switch {
	case v.EnvRef != "" && v.EnvSet:
		return v.Value + " (set)"
	case v.EnvRef != "":
		return v.Value + " (unset)"
	case v.Secret:
		return MaskedDisplay
	}
	return v.Value
}

// TargetInfo describes a merged target for bam target list/show.
type TargetInfo struct {
	Name      string
	Plan      string
	Server    string
	Branch    string
	Defaults  []TargetVar // sorted by name
	Options   map[string][]string
	Required  []string
	Watch     bool
	Timeout   time.Duration
	DefinedIn []string
}

// DescribeTargets returns every merged target, sorted by name.
func DescribeTargets(cfg *config.Config, getenv func(string) string) ([]TargetInfo, error) {
	all, err := cfg.Targets()
	if err != nil {
		return nil, err
	}
	out := make([]TargetInfo, 0, len(all))
	for _, name := range sortedNames(all) {
		out = append(out, describe(all[name], getenv))
	}
	return out, nil
}

// DescribeTarget returns one target.
func DescribeTarget(cfg *config.Config, getenv func(string) string, name string) (TargetInfo, error) {
	t, ok, err := cfg.Target(name)
	if err != nil {
		return TargetInfo{}, err
	}
	if !ok {
		return TargetInfo{}, errs.Usagef("unknown target %q", name).WithTry("bam target list")
	}
	return describe(t, getenv), nil
}

func describe(t config.ResolvedTarget, getenv func(string) string) TargetInfo {
	info := TargetInfo{Name: t.Name, Plan: t.Plan, Server: t.Server, Branch: t.Branch,
		Options: t.Options, Required: t.Required, Timeout: time.Duration(t.Timeout), DefinedIn: t.DefinedIn}
	if t.Watch != nil {
		info.Watch = *t.Watch
	}
	for _, name := range sortedNames(t.Defaults) {
		v := TargetVar{Name: name, Value: t.Defaults[name], Secret: config.IsMaskedName(name)}
		if ref, ok := config.EnvRef(v.Value); ok {
			v.EnvRef, v.EnvSet, v.Secret = ref, getenv(ref) != "", true
		}
		info.Defaults = append(info.Defaults, v)
	}
	return info
}
