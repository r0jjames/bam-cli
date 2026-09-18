package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/r0jjames/bam-cli/internal/app"
	"github.com/r0jjames/bam-cli/internal/config"
	"github.com/r0jjames/bam-cli/internal/provider"
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

func TestROpensTheFormForAPreset(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m, _ = send(m, presetsLoadedMsg{Targets: []app.TargetInfo{{Name: "smoke", Plan: "PROJ-PROV", Branch: "develop"}}})
	m.focus = focusPresets

	m, cmd := send(m, mkKey("R"))
	require.Equal(t, screenForm, m.screen)
	require.True(t, m.form.loading)
	require.NotNil(t, cmd)

	msg, ok := cmd().(formLoadedMsg)
	require.True(t, ok, "got %T", cmd())
	require.Equal(t, "PROJ-PROV12", msg.Ref.PlanKey, "the preset's branch is resolved")
	require.Equal(t, "smoke", msg.Target)
}

func TestROnAPlanOpensAFormWithNoTargetRules(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m, _ = send(m, plansLoadedMsg{Gen: m.plansGen, Plans: []provider.Plan{{Key: "PROJ-BUILD"}}})
	m.focus = focusPlans

	_, cmd := send(m, mkKey("R"))
	require.NotNil(t, cmd)
	msg, ok := cmd().(formLoadedMsg)
	require.True(t, ok, "got %T", cmd())
	require.Equal(t, "PROJ-BUILD", msg.Ref.PlanKey)
	require.Nil(t, msg.Ref.Target)
	require.Equal(t, "", msg.Target)
}

func TestFormLoadedFillsTheFields(t *testing.T) {
	m := goldenModel(80, 24)
	m.screen, m.form.loading = screenForm, true
	m, _ = send(m, formLoadedMsg{Gen: m.formGen, Ref: sampleRef(), Target: "provision-lab", Base: sampleBase()})
	require.False(t, m.form.loading)
	require.Len(t, m.form.fields, 3)
	require.Equal(t, "provision-lab", m.form.target)
}

func TestStaleFormLoadIsDropped(t *testing.T) {
	m := formModel()
	stale := m.formGen
	m.formGen++
	m, _ = send(m, formLoadedMsg{Gen: stale, Ref: sampleRef(), Target: "other", Base: sampleBase()})
	require.Equal(t, "provision-lab", m.form.target)
}

func TestEscLeavesTheForm(t *testing.T) {
	m := formModel()
	m, _ = send(m, mkKey("esc"))
	require.Equal(t, screenColumns, m.screen)
}

// TestROnNothingDoesNothing.
func TestROnNothingDoesNothing(t *testing.T) {
	m := New(Deps{})
	m.width, m.height = 80, 24
	m, cmd := send(m, mkKey("R"))
	require.Nil(t, cmd)
	require.Equal(t, screenColumns, m.screen)
}

// TestRIsNotReachableFromTheLogScreen: the log screen is for reading.
func TestRIsNotReachableFromTheLogScreen(t *testing.T) {
	m := logModel()
	m, cmd := send(m, mkKey("R"))
	require.Nil(t, cmd)
	require.Equal(t, screenLogs, m.screen)
}

func TestTabMovesBetweenFields(t *testing.T) {
	m := formModel()
	require.Equal(t, 0, m.form.cursor)
	m, _ = send(m, mkKey("tab"))
	require.Equal(t, 1, m.form.cursor)
	m, _ = send(m, mkKey("shift+tab"))
	require.Equal(t, 0, m.form.cursor)
	m, _ = send(m, mkKey("shift+tab"))
	require.Equal(t, 2, m.form.cursor, "moving back from the first field wraps")
}

func TestJAndKMoveBetweenFieldsToo(t *testing.T) {
	m := formModel()
	m, _ = send(m, mkKey("j"))
	require.Equal(t, 1, m.form.cursor)
	m, _ = send(m, mkKey("k"))
	require.Equal(t, 0, m.form.cursor)
}

func TestEnterEditsAFreeTextFieldAndAcceptsIt(t *testing.T) {
	m := formModel()
	m, _ = send(m, mkKey("enter"))
	require.True(t, m.form.editing)
	require.Equal(t, "beta", m.form.input.Value(), "editing starts from the current value")

	for _, r := range "gamma" {
		m, _ = send(m, mkKey(string(r)))
	}
	m, _ = send(m, mkKey("enter"))
	require.False(t, m.form.editing)
	require.Equal(t, "betagamma", m.form.fields[0].Value)
	require.True(t, m.form.fields[0].Touched)
}

// TestEnterOnAnOptionsFieldCyclesRatherThanEdits: the commonest rejection
// cannot be typed at all.
func TestEnterOnAnOptionsFieldCyclesRatherThanEdits(t *testing.T) {
	m := formModel()
	m.form.cursor = 1
	m, _ = send(m, mkKey("enter"))
	require.False(t, m.form.editing)
	require.Equal(t, "dcos", m.form.fields[1].Value)
	require.True(t, m.form.fields[1].Touched)

	m, _ = send(m, mkKey("enter"))
	require.Equal(t, "k8s", m.form.fields[1].Value, "cycling wraps")
}

// TestEscWhileEditingLeavesTheFieldNotTheForm.
func TestEscWhileEditingLeavesTheFieldNotTheForm(t *testing.T) {
	m := formModel()
	m, _ = send(m, mkKey("enter"))
	m, _ = send(m, mkKey("esc"))
	require.False(t, m.form.editing)
	require.Equal(t, screenForm, m.screen)

	m, _ = send(m, mkKey("esc"))
	require.Equal(t, screenColumns, m.screen)
}

// TestEscWhileEditingKeepsTheOldValue.
func TestEscWhileEditingKeepsTheOldValue(t *testing.T) {
	m := formModel()
	m, _ = send(m, mkKey("enter"))
	for _, r := range "zzz" {
		m, _ = send(m, mkKey(string(r)))
	}
	m, _ = send(m, mkKey("esc"))
	require.Equal(t, "beta", m.form.fields[0].Value)
}

// TestTypingQWhileEditingDoesNotQuit.
func TestTypingQWhileEditingDoesNotQuit(t *testing.T) {
	m := formModel()
	m, _ = send(m, mkKey("enter"))
	m, _ = send(m, mkKey("q"))
	require.Equal(t, screenForm, m.screen)
	require.True(t, m.form.editing)
	require.Contains(t, m.form.input.Value(), "q")
}

// TestASecretFieldNeverEchoesWhatIsTyped.
func TestASecretFieldNeverEchoesWhatIsTyped(t *testing.T) {
	m := formModel()
	m.form.cursor = 2
	m, _ = send(m, mkKey("enter"))
	for _, r := range "s3cret" {
		m, _ = send(m, mkKey(string(r)))
	}
	require.NotContains(t, m.View(), "s3cret")

	m, _ = send(m, mkKey("enter"))
	require.Equal(t, "s3cret", m.form.fields[2].Value)
	require.True(t, m.form.fields[2].Touched)
	require.NotContains(t, m.View(), "s3cret")
}

func TestAnEmptyRequiredFieldIsMarkedAndBlocksRun(t *testing.T) {
	m := formModel()
	m.form.fields[0].Value, m.form.fields[0].Touched = "", true
	m.revalidate()
	require.Contains(t, m.form.fields[0].Err, "required")
	require.False(t, m.canRun())
}

func TestFillingTheFieldClearsTheMark(t *testing.T) {
	m := formModel()
	m.form.fields[0].Value, m.form.fields[0].Touched = "", true
	m.revalidate()
	require.NotEmpty(t, m.form.fields[0].Err)

	m.form.fields[0].Value = "beta"
	m.revalidate()
	require.Empty(t, m.form.fields[0].Err)
	require.True(t, m.canRun())
}

// TestAnUnsetEnvReferenceNamesTheVariableAndBlocksRun.
func TestAnUnsetEnvReferenceNamesTheVariableAndBlocksRun(t *testing.T) {
	base := sampleBase()
	base.Vars = append(base.Vars, app.ResolvedVar{Name: "token", Value: "${LAB_TOKEN}", Source: "target"})
	ref := sampleRef()
	ref.Target.Defaults = config.StringMap{"cluster_type": "k8s", "token": "${LAB_TOKEN}"}

	m := formModel()
	m.deps.Getenv = func(string) string { return "" }
	m.form.base, m.form.ref = base, ref
	m.form.fields = buildFields(base, ref)
	m.revalidate()

	// The message names the variable, so it lands on that field's row rather
	// than only in the footer.
	var note string
	for _, f := range m.form.fields {
		if f.Name == "token" {
			note = f.Err
		}
	}
	require.Equal(t, "unset ${ENV}", note)
	require.False(t, m.canRun())
}

// TestWarningsDoNotBlock, exactly as they do not block bam run.
func TestWarningsDoNotBlock(t *testing.T) {
	m := formModel()
	m.form.fields = append(m.form.fields, formField{Name: "typo_name", Value: "x", Touched: true})
	m.revalidate()
	require.True(t, m.canRun())
	require.NoError(t, m.form.err)
}

// TestCyclingAnOptionRevalidates: the feedback follows the keystroke.
func TestCyclingAnOptionRevalidates(t *testing.T) {
	m := formModel()
	m.form.fields[0].Value, m.form.fields[0].Touched = "", true
	m.form.cursor = 1
	m, _ = send(m, mkKey("enter"))
	require.Contains(t, m.form.fields[0].Err, "required", "the required mark survives a cycle elsewhere")
}

// TestAcceptingAnEditRevalidates.
func TestAcceptingAnEditRevalidates(t *testing.T) {
	m := formModel()
	m, _ = send(m, mkKey("enter"))
	for i := 0; i < len("beta"); i++ {
		m, _ = send(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	m, _ = send(m, mkKey("enter"))
	require.Contains(t, m.form.fields[0].Err, "required")
}
