package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/r0jjames/bam-cli/internal/app"
)

// formField is one variable in the run form.
type formField struct {
	Name     string
	Value    string
	Source   string // plan, target, from #8, env LAB_TOKEN, flag
	Secret   bool
	Required bool
	Options  []string // non-empty means a cycle widget, not free text
	Touched  bool     // the user typed or cycled this field
	Err      string   // from ValidateVars, shown against this field
}

// formState is the run form: what was fetched once, and what the user has
// done to it since.
type formState struct {
	ref     app.PlanRef
	target  string // the preset's name; empty for a bare plan
	base    app.VarSet
	fields  []formField
	cursor  int
	editing bool
	input   textinput.Model
	err     error // a form-level error that ctrl-R refuses on
	loading bool
}

// buildFields turns a fetched VarSet into the form's rows, adding any name the
// target requires that the plan does not declare — otherwise a required
// variable would be invisible and impossible to satisfy.
func buildFields(base app.VarSet, ref app.PlanRef) []formField {
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

	seen := map[string]bool{}
	out := make([]formField, 0, len(base.Vars)+len(required))
	add := func(name, value, source string, secret bool) {
		if seen[name] {
			return
		}
		seen[name] = true
		// A secret is never prefilled: what is on screen is then never a
		// value the user did not type.
		if secret {
			value = ""
		}
		out = append(out, formField{
			Name: name, Value: value, Source: source, Secret: secret,
			Required: required[name], Options: options[name],
		})
	}
	for _, v := range base.Vars {
		add(v.Name, v.Value, v.Source, v.Secret)
	}
	for name := range required {
		add(name, "", "", false)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// flags is what the form hands ValidateVars, in --var shape.
//
// Only a field the user typed or cycled is sent. Every untouched value
// already sits in the base that ValidateVars is given, so resending it would
// change nothing and would raise a spurious undeclared-name warning for a
// target default the plan does not declare. An untouched secret therefore
// sends nothing, and the plan's or the environment's value stands.
func (f formState) flags() []string {
	var out []string
	for _, fl := range f.fields {
		if !fl.Touched {
			continue
		}
		out = append(out, fl.Name+"="+fl.Value)
	}
	return out
}

// changedCount is the footer's count: how many values differ from the plan's,
// because only those are sent to Bamboo.
func (f formState) changedCount() int {
	n := 0
	for _, fl := range f.fields {
		if fl.Touched {
			n++
		}
	}
	return n
}

// formView is the whole terminal: the run form in the standard panel frame.
func (m Model) formView() string {
	body := m.height - 1
	title := "Run " + m.form.ref.PlanKey
	if m.form.target != "" {
		title = "Run " + m.form.target + " · " + m.form.ref.PlanKey
	}
	if m.form.ref.Branch != "" {
		title += " · " + m.form.ref.Branch
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		panel(title, true, m.width, body, m.formBody(m.width-2, body-2)),
		m.statusBar(m.width))
}

// formBody is three columns: the name, the value, and the source or the
// constraint. A secret's value column is always ********, whatever the field
// holds, so a typed secret cannot reach the screen.
func (m Model) formBody(width, height int) string {
	if m.form.loading {
		return dimStyle.Render("loading the plan's variables…")
	}
	nameW := formNameWidth(width)
	valueW := formValueWidth(width)

	lines := []string{""}
	for i, f := range m.form.fields {
		value := f.Value
		switch {
		case f.Secret:
			value = app.MaskedDisplay
		case value == "":
			value = dimStyle.Render(strings.Repeat("_", min(valueW, 12)))
		}
		if m.form.editing && m.form.cursor == i {
			value = m.form.input.View()
		}
		line := m.cursorFor(true, m.form.cursor == i) +
			lipgloss.NewStyle().Width(nameW).Render(truncate(f.Name, nameW)) +
			lipgloss.NewStyle().Width(valueW).Render(truncate(value, valueW)) +
			m.fieldNote(f, width-nameW-valueW-4)
		lines = append(lines, strings.TrimRight(line, " "))
	}

	lines = append(lines, "", dimStyle.Render(fmt.Sprintf("%d of %d changed from the plan's values",
		m.form.changedCount(), len(m.form.fields))))
	if m.form.err != nil {
		lines = append(lines, "", errorStyle.Render(truncate(errorWhat(m.form.err), width)))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

// fieldNote is the third column: this field's own error, else its constraint,
// else where its value came from.
func (m Model) fieldNote(f formField, width int) string {
	switch {
	case f.Err != "":
		return errorStyle.Render(truncate(f.Err, width))
	case len(f.Options) > 0:
		return dimStyle.Render(truncate(strings.Join(f.Options, " | "), width))
	case f.Required && f.Value == "":
		return errorStyle.Render("required")
	case f.Required:
		return dimStyle.Render("required")
	}
	return dimStyle.Render(truncate(f.Source, width))
}

func formNameWidth(width int) int {
	w := width / 3
	switch {
	case w < 10:
		w = 10
	case w > 22:
		w = 22
	}
	return w
}

func formValueWidth(width int) int {
	w := width / 3
	if w < 10 {
		w = 10
	}
	return w
}
