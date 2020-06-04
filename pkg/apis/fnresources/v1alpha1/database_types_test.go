package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCreate(t *testing.T) {
	d := &Database{
		Spec: DatabaseSpec{
			User: "thisislongerthan16characters",
		},
	}
	require.EqualError(t, d.ValidateCreate(), "user 'thisislongerthan16characters' too many characters (28 > 16)")

	d = &Database{
		Spec: DatabaseSpec{
			User: "shortenough",
		},
	}
	require.NoError(t, d.ValidateCreate())
}

func TestValidateUpdate(t *testing.T) {
	d := &Database{
		Spec: DatabaseSpec{
			User: "orig",
		},
	}
	oldd := &Database{
		Spec: DatabaseSpec{
			User: "new",
		},
	}
	require.EqualError(t, d.ValidateUpdate(oldd), "user field is immutable")

	d = &Database{
		Spec: DatabaseSpec{
			SchemaName: "orig",
		},
	}
	oldd = &Database{
		Spec: DatabaseSpec{
			SchemaName: "new",
		},
	}
	require.EqualError(t, d.ValidateUpdate(oldd), "schemaName field is immutable")
}

func TestValidateDelete(t *testing.T) {
	d := &Database{}
	require.NoError(t, d.ValidateDelete())
}

func TestDefault(t *testing.T) {
	d := &Database{}
	d.Default()
	require.Equal(t, 3306, d.Spec.Port)
}
