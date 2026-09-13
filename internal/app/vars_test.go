package app

import (
	"testing"

	"github.com/r0jjames/bam-cli/internal/errs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withEnv(s *Service, env map[string]string) { s.Getenv = func(k string) string { return env[k] } }

func TestResolveVarsPrecedenceAndMasking(t *testing.T) {
	s := newService(t, fakeBamboo())
	withEnv(s, map[string]string{"LAB_DB_PASSWORD": "hunter2"})
	ref, err := s.ResolvePlan(bg, "provision-lab", "")
	require.NoError(t, err)

	set, err := s.ResolveVars(bg, ref, VarOptions{From: "last", Flags: []string{"compute_nodes=5"}})
	require.NoError(t, err)

	assert.Equal(t, "PROJ-PROV12-8", set.FromBuild)
	assert.True(t, set.DeclaredKnown)
	assert.Equal(t, map[string]string{
		"cluster_name":  "beta",
		"cluster_type":  "dcos",
		"compute_nodes": "5",
		"db_password":   "hunter2",
	}, set.Changed(), "global_flag is not declared and not in the target, so --from skips it")
	assert.Equal(t, map[string]bool{"db_password": true}, set.Secret())

	src := map[string]string{}
	for _, v := range set.Vars {
		src[v.Name] = v.Source
	}
	assert.Equal(t, map[string]string{"cluster_name": "from #8", "cluster_type": "from #8", "compute_nodes": "flag", "db_password": "env"}, src)

	pw, _ := set.Get("db_password")
	assert.Equal(t, MaskedDisplay, pw.Display())
	assert.Contains(t, set.Warnings[0], "db_password is masked in PROJ-PROV12-8")
	names := []string{}
	for _, v := range set.Vars {
		names = append(names, v.Name)
	}
	assert.IsIncreasing(t, names, "sorted by name")
}

func TestResolveVarsOnlySendsChanges(t *testing.T) {
	s := newService(t, fakeBamboo())
	ref, err := s.ResolvePlan(bg, "PROJ-PROV", "")
	require.NoError(t, err)
	set, err := s.ResolveVars(bg, ref, VarOptions{Flags: []string{"cluster_type=k8s"}})
	require.NoError(t, err)
	assert.Empty(t, set.Changed(), "k8s is already the plan value; the masked password is untouched")
}

func TestResolveVarsValidationOrder(t *testing.T) {
	s := newService(t, fakeBamboo())
	ref, _ := s.ResolvePlan(bg, "provision-lab", "")

	_, err := s.ResolveVars(bg, ref, VarOptions{})
	require.Error(t, err)
	assert.Equal(t, errs.KindUsage, errs.KindOf(err))
	assert.Contains(t, err.Error(), "LAB_DB_PASSWORD", "unset env reference is reported first")
	assert.Contains(t, err.Error(), "db_password")

	withEnv(s, map[string]string{"LAB_DB_PASSWORD": "x"})
	_, err = s.ResolveVars(bg, ref, VarOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster_name is required by target provision-lab")
	var e *errs.Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "--var cluster_name=VALUE", e.Try)

	_, err = s.ResolveVars(bg, ref, VarOptions{Flags: []string{"cluster_name=a", "cluster_type=nomad"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `cluster_type="nomad" is not allowed by target provision-lab`)
	assert.Contains(t, err.Error(), "allowed: k8s, dcos")
}

func TestResolveVarsWarnsOnUndeclaredFlag(t *testing.T) {
	s := newService(t, fakeBamboo())
	ref, _ := s.ResolvePlan(bg, "PROJ-PROV", "")
	set, err := s.ResolveVars(bg, ref, VarOptions{Flags: []string{"cluster_nme=x"}})
	require.NoError(t, err)
	require.Len(t, set.Warnings, 1)
	assert.Contains(t, set.Warnings[0], "cluster_nme is not a declared variable of PROJ-PROV")
	assert.Contains(t, set.Warnings[0], "did you mean cluster_name")
	assert.Equal(t, "x", set.Changed()["cluster_nme"], "still sent: it may override a global variable")
}

func TestResolveVarsBadFlag(t *testing.T) {
	s := newService(t, fakeBamboo())
	ref, _ := s.ResolvePlan(bg, "PROJ-PROV", "")
	_, err := s.ResolveVars(bg, ref, VarOptions{Flags: []string{"novalue"}})
	assert.Equal(t, errs.KindUsage, errs.KindOf(err))
}

func TestResolveVarsWithoutDeclaredList(t *testing.T) {
	p := fakeBamboo()
	p.VariablesErr = errs.Bamboof("plan variables cannot be listed").Wrap(errs.ErrUnsupported)
	s := newService(t, p)
	ref, _ := s.ResolvePlan(bg, "PROJ-PROV", "develop")
	set, err := s.ResolveVars(bg, ref, VarOptions{From: "8"})
	require.NoError(t, err)
	assert.False(t, set.DeclaredKnown)
	assert.Equal(t, "x", set.Changed()["global_flag"], "without a declared list every build variable is taken")
	_, masked := set.Changed()["db_password"]
	assert.False(t, masked, "a masked value is never sent back")
	assert.Contains(t, set.Warnings[len(set.Warnings)-1], "took every variable of PROJ-PROV12-8")
}

func TestResolveVarsFromUnsupported(t *testing.T) {
	p := fakeBamboo()
	p.BuildVarsErr = errs.Bamboof("no").Wrap(errs.ErrUnsupported)
	s := newService(t, p)
	ref, _ := s.ResolvePlan(bg, "PROJ-PROV", "develop")
	_, err := s.ResolveVars(bg, ref, VarOptions{From: "last"})
	require.Error(t, err)
	assert.Equal(t, errs.KindBamboo, errs.KindOf(err))
	assert.Contains(t, err.Error(), "cannot reuse variables of PROJ-PROV12-8")
}

func TestPlanVars(t *testing.T) {
	s := newService(t, fakeBamboo())
	ref, _ := s.ResolvePlan(bg, "PROJ-PROV", "develop")
	res, err := s.PlanVars(bg, ref, "")
	require.NoError(t, err)
	assert.Equal(t, "PROJ-PROV12-8", res.FromBuild)
	assert.Equal(t, []VarRow{
		{Name: "cluster_type", Value: "k8s", LastUsed: "dcos"},
		{Name: "compute_nodes", Value: "1", LastUsed: "3"},
		{Name: "cluster_name", Value: "", LastUsed: "beta"},
		{Name: "db_password", Value: "********", LastUsed: "********", Masked: true},
	}, res.Rows)
}
