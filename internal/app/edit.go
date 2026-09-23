package app

import (
	"fmt"
	"strings"

	"github.com/r0jjames/bam-cli/internal/config"
)

// The buffer of bam run --edit (run-edit spec §4). Render and parse live
// together because they are one round-trip contract: the ******** sentinel,
// the edit diff and the error block must agree. The functions return bytes
// and never touch a stream, so the terminal UI can reuse them.

const (
	errorPrefix = "# error: "
	errorMore   = "#   "
)

// editRow is one variable line as it is first shown.
type editRow struct {
	Name     string
	Shown    string // the value on the line: ******** for a secret with a value
	Secret   bool
	Required bool
	Options  []string
	V        ResolvedVar // zero when the row exists only for required/options
}

// editRows is the opened set plus the target's required and options names it
// lacks, sorted by name. It is the same rule as the run form's fields: a
// required name the plan does not declare still needs a line to be filled in.
func editRows(ref PlanRef, opened VarSet) []editRow {
	required := map[string]bool{}
	options := map[string][]string{}
	if t := ref.Target; t != nil {
		for _, n := range t.Required {
			required[n] = true
		}
		for n, vals := range t.Options {
			options[n] = vals
		}
	}
	rows := map[string]editRow{}
	for _, v := range opened.Vars {
		shown := v.Value
		if v.Secret && v.Value != "" {
			shown = MaskedDisplay
		}
		rows[v.Name] = editRow{Name: v.Name, Shown: shown, Secret: v.Secret, V: v}
	}
	for n := range required {
		if _, ok := rows[n]; !ok {
			rows[n] = editRow{Name: n, Secret: IsSecretName(n)}
		}
	}
	for n := range options {
		if _, ok := rows[n]; !ok {
			rows[n] = editRow{Name: n, Secret: IsSecretName(n)}
		}
	}
	out := make([]editRow, 0, len(rows))
	for _, n := range sortedNames(rows) {
		r := rows[n]
		r.Required, r.Options = required[n], options[n]
		out = append(out, r)
	}
	return out
}

// comment is the line above a row: source, env reference, required,
// options, secret. It never holds a secret value.
func (r editRow) comment(getenv func(string) string) string {
	var parts []string
	if src := r.V.Source; src != "" {
		if !r.Secret && r.V.Declared && src != "plan" && r.V.Value != r.V.PlanValue {
			plan := "empty"
			if r.V.PlanValue != "" {
				plan = fmt.Sprintf("%q", r.V.PlanValue)
			}
			src += " (plan: " + plan + ")"
		}
		parts = append(parts, src)
	}
	if r.V.Source == "target" {
		if name, ok := config.EnvRef(r.V.Value); ok {
			state := "unset"
			if getenv(name) != "" {
				state = "set"
			}
			parts = append(parts, "${"+name+"} "+state)
		}
	}
	if r.Required {
		parts = append(parts, "required")
	}
	if len(r.Options) > 0 {
		parts = append(parts, "options: "+strings.Join(r.Options, "|"))
	}
	if r.Secret {
		parts = append(parts, "secret")
	}
	if len(parts) == 0 {
		return ""
	}
	return "# " + strings.Join(parts, ", ") + "\n"
}

// EditBuffer renders the buffer bam run --edit opens: an error block for
// problems, a header, then one commented name=value line per variable.
func EditBuffer(ref PlanRef, opened VarSet, getenv func(string) string, server string, dryRun bool, problems []string) []byte {
	var b strings.Builder
	head := []string{"bam run " + ref.MasterKey}
	if ref.Target != nil {
		head = []string{"bam run " + ref.Target.Name, ref.MasterKey}
	}
	if ref.Branch != "" {
		head = append(head, "branch "+ref.Branch)
	} else {
		head = append(head, "default branch")
	}
	head = append(head, "server "+server)
	action := "run"
	if dryRun {
		action = "preview"
	}
	fmt.Fprintf(&b, "# %s\n", strings.Join(head, " · "))
	fmt.Fprintf(&b, "# One variable per line, name=value. Save and quit to %s.\n", action)
	b.WriteString("# " + MaskedDisplay + " keeps a secret's current value. A value of exactly ${NAME} reads the environment.\n")
	b.WriteString("# Delete a line to keep its value. Delete every line, or quit with an error (:cq), to abort.\n\n")
	for _, r := range editRows(ref, opened) {
		b.WriteString(r.comment(getenv))
		b.WriteString(r.Name + "=" + r.Shown + "\n")
	}
	return ReopenBuffer([]byte(b.String()), problems)
}

// ReopenBuffer replaces the error block at the top of text with one for
// problems, leaving everything else as the user left it.
func ReopenBuffer(text []byte, problems []string) []byte {
	lines := strings.SplitAfter(string(text), "\n")
	i := 0
	for i < len(lines) && strings.HasPrefix(lines[i], errorPrefix) {
		i++
		for i < len(lines) && strings.HasPrefix(lines[i], errorMore) {
			i++
		}
	}
	if i > 0 && i < len(lines) && strings.TrimRight(lines[i], "\r\n") == "#" {
		i++
	}
	var b strings.Builder
	for _, p := range problems {
		first, rest, more := strings.Cut(p, "\n")
		b.WriteString(errorPrefix + first + "\n")
		if more {
			for _, l := range strings.Split(rest, "\n") {
				b.WriteString(errorMore + l + "\n")
			}
		}
	}
	if len(problems) > 0 {
		b.WriteString("#\n")
	}
	b.WriteString(strings.Join(lines[i:], ""))
	return []byte(b.String())
}
