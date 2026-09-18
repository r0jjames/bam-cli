package tui

import (
	"testing"

	"github.com/r0jjames/bam-cli/internal/app"
	"github.com/r0jjames/bam-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func sampleTarget() *config.ResolvedTarget {
	return &config.ResolvedTarget{Name: "provision-lab", Target: config.Target{
		Plan:     "PROJ-PROV",
		Branch:   "develop",
		Defaults: config.StringMap{"cluster_type": "k8s"},
		Options:  config.StringListMap{"cluster_type": {"k8s", "dcos"}},
		Required: []string{"cluster_name"},
	}}
}

func sampleRef() app.PlanRef {
	return app.PlanRef{PlanKey: "PROJ-PROV12", MasterKey: "PROJ-PROV", Branch: "develop", Target: sampleTarget()}
}

func sampleBase() app.VarSet {
	return app.VarSet{
		DeclaredKnown: true,
		Declared:      map[string]bool{"cluster_name": true, "cluster_type": true, "ssh_key": true},
		Vars: []app.ResolvedVar{
			{Name: "cluster_name", Declared: true, Source: "plan"},
			{Name: "cluster_type", Value: "k8s", PlanValue: "k8s", Source: "target", Declared: true},
			{Name: "ssh_key", Value: app.MaskedDisplay, Source: "plan", Declared: true, Secret: true},
		},
	}
}

func TestBuildFieldsCarriesSourceRequiredAndOptions(t *testing.T) {
	fs := buildFields(sampleBase(), sampleRef())
	require.Len(t, fs, 3)

	require.Equal(t, "cluster_name", fs[0].Name)
	require.True(t, fs[0].Required)
	require.Empty(t, fs[0].Options)

	require.Equal(t, "cluster_type", fs[1].Name)
	require.Equal(t, []string{"k8s", "dcos"}, fs[1].Options)
	require.False(t, fs[1].Required)
	require.Equal(t, "target", fs[1].Source)

	require.Equal(t, "ssh_key", fs[2].Name)
	require.True(t, fs[2].Secret)
}

// TestBuildFieldsAddsARequiredNameThePlanDoesNotDeclare, so it can be filled
// in rather than being invisible and impossible to satisfy.
func TestBuildFieldsAddsARequiredNameThePlanDoesNotDeclare(t *testing.T) {
	ref := sampleRef()
	ref.Target.Required = append(ref.Target.Required, "ticket")
	names := fieldNames(buildFields(sampleBase(), ref))
	require.Contains(t, names, "ticket")
}

// TestSecretFieldsStartEmpty: a secret is never prefilled, so what is on
// screen is never a value the user did not type.
func TestSecretFieldsStartEmpty(t *testing.T) {
	for _, f := range buildFields(sampleBase(), sampleRef()) {
		if f.Secret {
			require.Equal(t, "", f.Value)
			require.False(t, f.Touched)
		}
	}
}

// TestFlagsSendsOnlyWhatWasTyped. An untouched field's value already sits in
// the base that ValidateVars is given, so resending it would only produce a
// spurious undeclared-name warning; an untouched secret therefore sends
// nothing, and the plan's or the environment's value stands.
func TestFlagsSendsOnlyWhatWasTyped(t *testing.T) {
	f := formState{fields: buildFields(sampleBase(), sampleRef())}
	require.Empty(t, f.flags())

	f.fields[0].Value, f.fields[0].Touched = "beta", true
	require.Equal(t, []string{"cluster_name=beta"}, f.flags())

	f.fields[2].Value, f.fields[2].Touched = "typed", true
	require.Contains(t, f.flags(), "ssh_key=typed")
}

// TestFlagsKeepsAnEmptyTypedValue: clearing a field on purpose is a value.
func TestFlagsKeepsAnEmptyTypedValue(t *testing.T) {
	f := formState{fields: buildFields(sampleBase(), sampleRef())}
	f.fields[1].Value, f.fields[1].Touched = "", true
	require.Equal(t, []string{"cluster_type="}, f.flags())
}

func fieldNames(fs []formField) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Name)
	}
	return out
}

func formModel() Model {
	m := goldenModel(80, 24)
	m.svc = testService()
	m.screen = screenForm
	m.form = formState{ref: sampleRef(), target: "provision-lab", base: sampleBase(),
		fields: buildFields(sampleBase(), sampleRef())}
	m.form.fields[0].Value, m.form.fields[0].Touched = "beta", true
	return m
}

func TestFormHeaderNamesTheTargetPlanAndBranch(t *testing.T) {
	v := formModel().View()
	require.Contains(t, v, "provision-lab")
	require.Contains(t, v, "PROJ-PROV12")
	require.Contains(t, v, "develop")
}

func TestFormShowsSourcesAndConstraints(t *testing.T) {
	v := formModel().View()
	require.Contains(t, v, "required")
	require.Contains(t, v, "k8s | dcos")
}

// TestFormNeverShowsASecretValue.
func TestFormNeverShowsASecretValue(t *testing.T) {
	m := formModel()
	m.form.fields[2].Value, m.form.fields[2].Touched = "s3cret", true
	v := m.View()
	require.NotContains(t, v, "s3cret")
	require.Contains(t, v, app.MaskedDisplay)
}

func TestFormHidesTheLeftColumn(t *testing.T) {
	require.NotContains(t, formModel().View(), "1 Plans")
}

func TestFormCountsWhatChanged(t *testing.T) {
	require.Contains(t, formModel().View(), "1 of 3 changed")
}

func TestFormWhileLoadingSaysSo(t *testing.T) {
	m := formModel()
	m.form.loading, m.form.fields = true, nil
	require.Contains(t, m.View(), "loading")
}

func TestFormGolden(t *testing.T) { requireGolden(t, "form-80x24", formModel().View()) }
