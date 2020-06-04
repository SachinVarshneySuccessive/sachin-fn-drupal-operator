package customercontainer

import (
	"github.com/acquia/fn-drupal-operator/pkg/common"
	goldenHelper "github.com/acquia/fn-go-utils/pkg/testhelpers"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomerECR(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		require.Equal(t, "881217801864.dkr.ecr.us-east-1.amazonaws.com", CustomerECR())
	})
}

func TestNormalizeGitPartialURL(t *testing.T) {
	t.Run("GitHub", func(t *testing.T) {
		require.Equal(t, "github.com/acquia/kpoc", NormalizeGitPartialURL("github.com/acquia/kpoc"))
	})
	t.Run("Bitbucket", func(t *testing.T) {
		require.Equal(t, "bitbucket.org/tenuc/xdfrrpf", NormalizeGitPartialURL("bitbucket.org/tenuc/xdfrrpf"))
	})
	t.Run("Acquia", func(t *testing.T) {
		require.Equal(t, "svn-2.archteam.srvs.ahdev.co/nebula", NormalizeGitPartialURL("nebula@svn-2.archteam.srvs.ahdev.co:nebula.git"))
	})
}

func TestECRRepoNameFromGitRepoURL(t *testing.T) {
	t.Run("GitHub", func(t *testing.T) {
		require.Equal(t, "customer/github.com/acquia/kpoc", ECRRepoNameFromGitRepoURL("github.com/acquia/kpoc"))
	})
	t.Run("Bitbucket", func(t *testing.T) {
		require.Equal(t, "customer/bitbucket.org/tenuc/xdfrrpf", ECRRepoNameFromGitRepoURL("bitbucket.org/tenuc/xdfrrpf"))
	})
	t.Run("Acquia", func(t *testing.T) {
		require.Equal(t, "customer/svn-2.archteam.srvs.ahdev.co/nebula", ECRRepoNameFromGitRepoURL("nebula@svn-2.archteam.srvs.ahdev.co:nebula.git"))
	})
}

func TestECRRepoURIFromGitRepoURL(t *testing.T) {
	t.Run("GitHub", func(t *testing.T) {
		require.Equal(t, "881217801864.dkr.ecr.us-east-1.amazonaws.com/customer/github.com/acquia/kpoc", ECRRepoURIFromGitRepoURL("github.com/acquia/kpoc"))
	})
	t.Run("Bitbucket", func(t *testing.T) {
		require.Equal(t, "881217801864.dkr.ecr.us-east-1.amazonaws.com/customer/bitbucket.org/tenuc/xdfrrpf", ECRRepoURIFromGitRepoURL("bitbucket.org/tenuc/xdfrrpf"))
	})
	t.Run("Acquia", func(t *testing.T) {
		require.Equal(t, "881217801864.dkr.ecr.us-east-1.amazonaws.com/customer/svn-2.archteam.srvs.ahdev.co/nebula", ECRRepoURIFromGitRepoURL("nebula@svn-2.archteam.srvs.ahdev.co:nebula.git"))
	})
}

func TestIsCustomerECRRepo(t *testing.T) {
	t.Run("Is customer", func(t *testing.T) {
		require.True(t, IsCustomerECRRepo("customer/github.com/acquia/kpoc"))
	})
	t.Run("Not customer", func(t *testing.T) {
		require.False(t, IsCustomerECRRepo("foobar/bitbucket.org/tenuc/xdfrrpf"))
	})
}

func TestGitPartialURLFromECRRepoName(t *testing.T) {
	t.Run("GitHub", func(t *testing.T) {
		require.Equal(t, "github.com/acquia/kpoc", GitPartialURLFromECRRepoName("customer/github.com/acquia/kpoc"))
	})
	t.Run("Bitbucket", func(t *testing.T) {
		require.Equal(t, "bitbucket.org/tenuc/xdfrrpf", GitPartialURLFromECRRepoName("customer/bitbucket.org/tenuc/xdfrrpf"))
	})
	t.Run("Acquia", func(t *testing.T) {
		require.Equal(t, "svn-2.archteam.srvs.ahdev.co/nebula", GitPartialURLFromECRRepoName("customer/svn-2.archteam.srvs.ahdev.co/nebula"))
	})
}

func TestTemplate(t *testing.T) {
	common.SetRealm_ForTestsOnly("TestRealm")
	common.SetAwsRegion_ForTestsOnly("TestRegion")

	t.Run("Template with ImageRepo", func(t *testing.T) {
		testCustomerTemplate := Template(drupalApplicationWithID, drupalEnvironmentWithID)
		require.True(t, goldenHelper.GoldenSpec(t, "TemplateImageRepo", testCustomerTemplate))
	})
}
