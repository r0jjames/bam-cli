package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/r0jjames/bam-cli/internal/config"
	"github.com/r0jjames/bam-cli/internal/errs"
)

// MaskedDisplay replaces every secret value in output.
const MaskedDisplay = "********"

// ResolvedVar is one variable after every source has been applied.
type ResolvedVar struct {
	Name      string
	Value     string // never print directly; use Display
	Source    string // plan | target | from #N | flag | env
	Secret    bool
	Declared  bool
	PlanValue string
}

// Display is the value as it may be shown.
func (v ResolvedVar) Display() string {
	if v.Secret {
		return MaskedDisplay
	}
	return v.Value
}

// VarSet is the resolved variables of one run.
type VarSet struct {
	Vars          []ResolvedVar // sorted by name
	FromBuild     string
	Warnings      []string
	DeclaredKnown bool
}

// Changed returns the variables to send: undeclared ones and those whose
// value differs from the plan's.
func (v VarSet) Changed() map[string]string {
	out := map[string]string{}
	for _, r := range v.Vars {
		if !r.Declared || r.Value != r.PlanValue {
			out[r.Name] = r.Value
		}
	}
	return out
}

// Secret returns the names whose values must be redacted.
func (v VarSet) Secret() map[string]bool {
	out := map[string]bool{}
	for _, r := range v.Vars {
		if r.Secret {
			out[r.Name] = true
		}
	}
	return out
}

// Get returns one variable by name.
func (v VarSet) Get(name string) (ResolvedVar, bool) {
	for _, r := range v.Vars {
		if r.Name == name {
			return r, true
		}
	}
	return ResolvedVar{}, false
}

// VarOptions are the command-line inputs to variable resolution.
type VarOptions struct {
	From  string   // --from
	Flags []string // --var name=value, in order
}

// ResolveVars applies, lowest to highest: plan values, target defaults,
// --from, --var. Then it validates env references, required names and allowed
// values (spec §2.3).
func (s *Service) ResolveVars(ctx context.Context, ref PlanRef, o VarOptions) (VarSet, error) {
	var set VarSet
	vars := map[string]*ResolvedVar{}
	declaredNames := map[string]bool{}

	declared, err := s.P.ListVariables(ctx, ref.MasterKey)
	switch {
	case err == nil:
		set.DeclaredKnown = true
	case errors.Is(err, errs.ErrUnsupported):
	default:
		return VarSet{}, err
	}
	for _, d := range declared {
		declaredNames[d.Name] = true
		vars[d.Name] = &ResolvedVar{Name: d.Name, Value: d.Value, PlanValue: d.Value, Source: "plan",
			Declared: true, Secret: d.Masked || config.IsMaskedName(d.Name)}
	}
	get := func(name string) *ResolvedVar {
		v, ok := vars[name]
		if !ok {
			v = &ResolvedVar{Name: name, Secret: config.IsMaskedName(name)}
			vars[name] = v
		}
		return v
	}
	t := ref.Target
	inTarget := func(name string) bool {
		if t == nil {
			return false
		}
		_, ok := t.Defaults[name]
		return ok
	}

	if t != nil {
		for name, val := range t.Defaults {
			v := get(name)
			v.Value, v.Source = val, "target"
		}
	}

	if o.From != "" {
		key, err := s.ResolveFrom(ctx, ref, o.From)
		if err != nil {
			return VarSet{}, err
		}
		bv, err := s.P.BuildVariables(ctx, key)
		if errors.Is(err, errs.ErrUnsupported) {
			return VarSet{}, errs.Bamboof("cannot reuse variables of %s", key).
				WithWhy("this Bamboo server does not return the variables a build used").
				WithTry("pass the values with --var")
		}
		if err != nil {
			return VarSet{}, err
		}
		set.FromBuild = key
		label := "from #" + key[strings.LastIndex(key, "-")+1:]
		for _, name := range sortedNames(bv) {
			val := bv[name]
			if set.DeclaredKnown && !declaredNames[name] && !inTarget(name) {
				continue
			}
			if val == MaskedDisplay {
				keep := "leaving it unset"
				if v, ok := vars[name]; ok {
					keep = "keeping the " + v.Source + " value"
				}
				set.Warnings = append(set.Warnings, fmt.Sprintf("%s is masked in %s; %s", name, key, keep))
				continue
			}
			v := get(name)
			v.Value, v.Source = val, label
		}
		if !set.DeclaredKnown {
			set.Warnings = append(set.Warnings, fmt.Sprintf("plan variables of %s cannot be listed; took every variable of %s", ref.MasterKey, key))
		}
	}

	for _, f := range o.Flags {
		name, val, ok := strings.Cut(f, "=")
		if !ok || name == "" {
			return VarSet{}, errs.Usagef("--var takes name=value, not %q", f)
		}
		if set.DeclaredKnown && !declaredNames[name] && !inTarget(name) {
			w := fmt.Sprintf("%s is not a declared variable of %s", name, ref.MasterKey)
			if c := Closest(name, sortedNames(declaredNames), 1); len(c) > 0 {
				w += "; did you mean " + c[0] + "?"
			}
			set.Warnings = append(set.Warnings, w)
		}
		v := get(name)
		v.Value, v.Source = val, "flag"
	}

	names := sortedNames(vars)
	for _, name := range names {
		v := vars[name]
		if v.Source != "target" {
			continue
		}
		if envName, ok := config.EnvRef(v.Value); ok {
			val := s.Getenv(envName)
			if val == "" {
				return VarSet{}, errs.Usagef("variable %s needs environment variable %s, which is not set", name, envName).
					WithTry(fmt.Sprintf("export %s=..., or pass --var %s=VALUE", envName, name))
			}
			v.Value, v.Source, v.Secret = val, "env", true
		}
	}
	if t != nil {
		for _, name := range t.Required {
			if v, ok := vars[name]; !ok || v.Value == "" {
				return VarSet{}, errs.Usagef("%s is required by target %s", name, t.Name).
					WithTry(fmt.Sprintf("--var %s=VALUE", name))
			}
		}
		for _, name := range sortedNames(t.Options) {
			v, ok := vars[name]
			if !ok || v.Value == "" {
				continue
			}
			if !slices.Contains(t.Options[name], v.Value) {
				return VarSet{}, errs.Usagef("%s=%q is not allowed by target %s", name, v.Value, t.Name).
					WithWhy("allowed: " + strings.Join(t.Options[name], ", "))
			}
		}
	}

	for _, name := range names {
		set.Vars = append(set.Vars, *vars[name])
	}
	return set, nil
}

// VarRow is one line of bam plan vars.
type VarRow struct {
	Name     string
	Value    string // plan value
	LastUsed string // value in the reference build, if readable
	Masked   bool
}

// PlanVarsResult is what bam plan vars shows.
type PlanVarsResult struct {
	Rows      []VarRow
	FromBuild string
	Warnings  []string
}

// PlanVars lists declared variables with their values in a reference build
// (from, default "last").
func (s *Service) PlanVars(ctx context.Context, ref PlanRef, from string) (PlanVarsResult, error) {
	var res PlanVarsResult
	if from == "" {
		from = "last"
	}
	var used map[string]string
	if key, err := s.ResolveFrom(ctx, ref, from); err == nil {
		bv, err := s.P.BuildVariables(ctx, key)
		switch {
		case err == nil:
			used, res.FromBuild = bv, key
		case errors.Is(err, errs.ErrUnsupported):
			res.Warnings = append(res.Warnings, "this server does not return the variables a build used")
		default:
			return res, err
		}
	} else if errs.KindOf(err) != errs.KindUsage || from != "last" {
		return res, err
	}

	declared, err := s.P.ListVariables(ctx, ref.MasterKey)
	switch {
	case err == nil:
		for _, d := range declared {
			res.Rows = append(res.Rows, VarRow{Name: d.Name, Value: d.Value, LastUsed: used[d.Name],
				Masked: d.Masked || config.IsMaskedName(d.Name)})
		}
	case errors.Is(err, errs.ErrUnsupported):
		res.Warnings = append(res.Warnings, fmt.Sprintf("plan variables of %s cannot be listed; showing the variables of %s", ref.MasterKey, res.FromBuild))
		for _, name := range sortedNames(used) {
			res.Rows = append(res.Rows, VarRow{Name: name, LastUsed: used[name], Masked: config.IsMaskedName(name) || used[name] == MaskedDisplay})
		}
	default:
		return res, err
	}
	return res, nil
}

func sortedNames[M ~map[string]V, V any](m M) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
