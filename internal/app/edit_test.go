package app

import (
	"strings"
	"testing"

	"github.com/r0jjames/bam-cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func editRef() PlanRef {
	tgt := config.ResolvedTarget{Name: "provision-lab", Target: config.Target{Plan: "PROJ-PROV", Branch: "develop",
		Defaults: config.StringMap{"cluster_type": "k8s", "db_password": "${LAB_DB_PASSWORD}"},
		Options:  config.StringListMap{"cluster_type": {"k8s", "dcos"}, "region": {"eu", "us"}},
		Required: []string{"cluster_name", "ticket"},
	}}
	return PlanRef{PlanKey: "PROJ-PROV12", MasterKey: "PROJ-PROV", Branch: "develop", Target: &tgt}
}

// editOpened is what ApplyVars would give for editRef with --from 8 and
// --var cluster_name=alpha.
func editOpened() VarSet {
	return VarSet{DeclaredKnown: true,
		Declared: map[string]bool{"api_token": true, "cluster_name": true, "cluster_type": true, "compute_nodes": true},
		Vars: []ResolvedVar{
			{Name: "api_token", Value: "********", PlanValue: "********", Source: "plan", Declared: true, Secret: true},
			{Name: "cluster_name", Value: "alpha", Source: "flag", Declared: true},
			{Name: "cluster_type", Value: "k8s", PlanValue: "k8s", Source: "target", Declared: true},
			{Name: "compute_nodes", Value: "3", PlanValue: "1", Source: "from #8", Declared: true},
			{Name: "db_password", Value: "${LAB_DB_PASSWORD}", Source: "target", Secret: true},
		}}
}

const editHeader = "" +
	"# bam run provision-lab · PROJ-PROV · branch develop · server work\n" +
	"# One variable per line, name=value. Save and quit to run.\n" +
	"# ******** keeps a secret's current value. A value of exactly ${NAME} reads the environment.\n" +
	"# Delete a line to keep its value. Delete every line, or quit with an error (:cq), to abort.\n" +
	"\n"

const editRowsText = "" +
	"# plan, secret\n" +
	"api_token=********\n" +
	"# flag (plan: empty), required\n" +
	"cluster_name=alpha\n" +
	"# target, options: k8s|dcos\n" +
	"cluster_type=k8s\n" +
	"# from #8 (plan: \"1\")\n" +
	"compute_nodes=3\n" +
	"# target, ${LAB_DB_PASSWORD} unset, secret\n" +
	"db_password=********\n" +
	"# options: eu|us\n" +
	"region=\n" +
	"# required\n" +
	"ticket=\n"

func TestEditBufferRendersEveryRow(t *testing.T) {
	got := EditBuffer(editRef(), editOpened(), emptyEnv, "work", false, nil)
	assert.Equal(t, editHeader+editRowsText, string(got))
}

func TestEditBufferNeverHoldsASecret(t *testing.T) {
	env := func(k string) string {
		if k == "LAB_DB_PASSWORD" {
			return "hunter2"
		}
		return ""
	}
	opened := editOpened()
	opened.Vars[0].Value = "tok-123" // a secret the user passed with --var
	got := string(EditBuffer(editRef(), opened, env, "work", false, nil))
	assert.NotContains(t, got, "hunter2")
	assert.NotContains(t, got, "tok-123")
	assert.Contains(t, got, "# target, ${LAB_DB_PASSWORD} set, secret\ndb_password=********\n")
}

func TestEditBufferShowsAnEmptySecretAsEmpty(t *testing.T) {
	opened := editOpened()
	opened.Vars[0].Value = ""
	got := string(EditBuffer(editRef(), opened, emptyEnv, "work", false, nil))
	assert.Contains(t, got, "\napi_token=\n")
}

func TestEditBufferForAPlanOnTheDefaultBranch(t *testing.T) {
	ref := PlanRef{PlanKey: "PROJ-BUILD", MasterKey: "PROJ-BUILD"}
	got := string(EditBuffer(ref, VarSet{}, emptyEnv, "work", true, nil))
	assert.True(t, strings.HasPrefix(got,
		"# bam run PROJ-BUILD · default branch · server work\n"+
			"# One variable per line, name=value. Save and quit to preview.\n"), got)
}

func TestEditBufferPutsProblemsFirst(t *testing.T) {
	got := string(EditBuffer(editRef(), editOpened(), emptyEnv, "work", false,
		[]string{"cluster_type=\"nomad\" is not allowed by target provision-lab\nallowed: k8s, dcos", "line 3 is not name=value"}))
	assert.Equal(t, ""+
		"# error: cluster_type=\"nomad\" is not allowed by target provision-lab\n"+
		"#   allowed: k8s, dcos\n"+
		"# error: line 3 is not name=value\n"+
		"#\n"+
		editHeader+editRowsText, got)
}

func TestReopenBufferReplacesTheBlock(t *testing.T) {
	user := "# my note\ncluster_name=beta\n"
	once := ReopenBuffer([]byte(user), []string{"first\nwhy"})
	assert.Equal(t, "# error: first\n#   why\n#\n"+user, string(once))

	twice := ReopenBuffer(once, []string{"second"})
	assert.Equal(t, "# error: second\n#\n"+user, string(twice), "blocks never stack")

	assert.Equal(t, user, string(ReopenBuffer(twice, nil)), "no problems, no block")
	assert.Equal(t, user, string(ReopenBuffer([]byte(user), nil)), "text without a block is untouched")
}
