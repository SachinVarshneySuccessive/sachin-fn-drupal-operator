package common

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestMain(m *testing.M) {
	// Disable attempts to read EC2 metadata endpoint, since it won't exist for automated tests.
	if err := os.Setenv(isAwsEc2DisabledEnv, "true"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestCommon_EnvVars(t *testing.T) {
	val, wasSet := os.LookupEnv(istioEnabledEnv)

	logf.SetLogger(logf.ZapLogger(true))

	values := []string{"2", "", "false", "FALSE", "False"}

	for _, v := range values {
		_ = os.Setenv(istioEnabledEnv, v)
		parseEnvVarValues()
		require.False(t, IsIstioEnabled())
	}

	values = []string{"true", "TRUE", "True"}

	for _, v := range values {
		_ = os.Setenv(istioEnabledEnv, v)
		parseEnvVarValues()
		require.True(t, IsIstioEnabled())
	}

	_ = os.Unsetenv(istioEnabledEnv)
	parseEnvVarValues()
	require.False(t, IsIstioEnabled())

	if wasSet {
		os.Setenv(istioEnabledEnv, val)
	} else {
		os.Unsetenv(istioEnabledEnv)
	}
	parseEnvVarValues()
}

func TestMergeLabelsAndAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		base     Labels
		overlay  Labels
		expected Labels
	}{
		{
			"simple",
			Labels{"baseonly": "foo", "dup": "bar"},
			Labels{"overlayonly": "baz", "dup": "bip"},
			Labels{"baseonly": "foo", "overlayonly": "baz", "dup": "bip"},
		},
		{
			"nil-base",
			nil,
			Labels{"overlayonly": "baz", "dup": "bip"},
			Labels{"overlayonly": "baz", "dup": "bip"},
		},
		{
			"nil-overlay",
			Labels{"baseonly": "foo", "dup": "bar"},
			nil,
			Labels{"baseonly": "foo", "dup": "bar"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, MergeLabels(test.base, test.overlay))
			require.Equal(t, test.expected, MergeAnnotations(test.base, test.overlay))
		})
	}
}

func TestMeetsVersionConstraint(t *testing.T) {
	t.Run("should be valid conditions", func(t *testing.T) {
		require.True(t, MeetsVersionConstraint(">=7.3", "7.3"))
		require.True(t, MeetsVersionConstraint(">=7.3", "7.3.2"))
		require.True(t, MeetsVersionConstraint(">=7.3", "7.4"))
	})

	t.Run("should be invalid conditions", func(t *testing.T) {
		require.False(t, MeetsVersionConstraint(">=7.3", "7.2"))
		require.False(t, MeetsVersionConstraint(">=7.3", "7.2.9"))
		require.False(t, MeetsVersionConstraint(">=7.3", "custom"))
	})
}
