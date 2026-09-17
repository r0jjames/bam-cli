package app

import (
	"testing"

	"github.com/r0jjames/bam-cli/internal/errs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDescribeTargets(t *testing.T) {
	cfg := testConfig(t.TempDir())
	infos, err := DescribeTargets(cfg, func(k string) string { return map[string]string{"LAB_DB_PASSWORD": "x"}[k] })
	require.NoError(t, err)
	require.Len(t, infos, 2)
	assert.Equal(t, "build", infos[0].Name)
	lab := infos[1]
	assert.Equal(t, "PROJ-PROV", lab.Plan)
	assert.Equal(t, "develop", lab.Branch)
	assert.True(t, lab.Watch)
	assert.Equal(t, []string{"cluster_name"}, lab.Required)
	assert.Equal(t, []TargetVar{
		{Name: "cluster_type", Value: "k8s"},
		{Name: "compute_nodes", Value: "2"},
		{Name: "db_password", Value: "${LAB_DB_PASSWORD}", EnvRef: "LAB_DB_PASSWORD", EnvSet: true, Secret: true},
	}, lab.Defaults)
	assert.Equal(t, "${LAB_DB_PASSWORD} (set)", lab.Defaults[2].Display())

	_, err = DescribeTarget(cfg, func(string) string { return "" }, "nope")
	assert.Equal(t, errs.KindUsage, errs.KindOf(err))
}
