package common

import (
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	istioEnabledEnv           = "ISTIO_ENABLED"
	defaultStorageClassEnv    = "DEFAULT_STORAGE_CLASS"
	useDynamicProvisioningEnv = "USE_DYNAMIC_PROVISIONING"
	realmEnv                  = "AH_REALM"
	isAwsEc2DisabledEnv       = "AWS_EC2_METADATA_DISABLED"
)

var (
	isIstioEnabled         = false
	defaultStorageClass    = "efs"
	useDynamicProvisioning = false
	awsRegion              = ""
	realm                  = ""
)
var log = logf.Log.WithName("common")

func init() {
	parseEnvVarValues()
}

func parseEnvVarValues() {
	isIstioEnabled, _ = strconv.ParseBool(os.Getenv(istioEnabledEnv))
	useDynamicProvisioning, _ = strconv.ParseBool(os.Getenv(useDynamicProvisioningEnv))

	if sc, exists := os.LookupEnv(defaultStorageClassEnv); exists {
		defaultStorageClass = sc
	}

	if r, exists := os.LookupEnv(realmEnv); exists {
		realm = r
	}
	initAwsRegion()

}

func initAwsRegion() {
	isAwsEc2Disabled, _ := strconv.ParseBool(os.Getenv(isAwsEc2DisabledEnv))
	if isAwsEc2Disabled {
		log.Info(" EC2 Metadata checks disabled by environment variable")
		awsRegion = "local-cluster"
	} else {
		session := session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		}))
		region, err := ec2metadata.New(session).Region()
		if err != nil {
			log.Error(err, "Failed to get AWS region from EC2 metadata service")
		} else {
			awsRegion = region
			log.Info("Detected AWS Region", "region", awsRegion)
		}
	}
}

// IsIstioEnabled returns true if Istio is enabled, derived from an environment variable.
func IsIstioEnabled() bool {
	return isIstioEnabled
}

// DefaultStorageClass returns the default storage class that should be used for Drupal infrastructure, derived from an
// environment variable.
func DefaultStorageClass() string {
	return defaultStorageClass
}

// Realm returns the Realm, which could be local-cluster or production environment name
func Realm() string {
	return realm
}

//AwsRegion returns the region from AWS metadata or returns "local-cluster" if not on an EKS cluster
func AwsRegion() string {
	return awsRegion
}

// UseDynamicProvisioning returns true if Dynamic PersistentVolume provisioning should be used, derived from an
// environment variable.
func UseDynamicProvisioning() bool {
	return useDynamicProvisioning
}

func SetIsIstioEnabled_ForTestsOnly(b bool) {
	isIstioEnabled = b
}

func SetDefaultStorageClass_ForTestsOnly(s string) {
	defaultStorageClass = s
}

func SetUseDynamicProvisioning_ForTestsOnly(b bool) {
	useDynamicProvisioning = b
}

func SetAwsRegion_ForTestsOnly(s string) {
	awsRegion = s
}

func SetRealm_ForTestsOnly(s string) {
	realm = s
}

func RandPassword() (string, error) {
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	length := 16
	var b strings.Builder
	for i := 0; i < length; i++ {
		indx, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars)))) // NOTE - crypto/rand library is not FIPS 140-2 validated, and never will be: see https://github.com/golang/go/issues/11658#issuecomment-120448974
		if err != nil {
			return "", err
		}
		b.WriteRune(chars[indx.Int64()])
	}
	return b.String(), nil
}

func HashValueForLabel(s string) string {
	hash := sha1.New()
	hash.Write([]byte(s))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func HashValueOf(objs ...interface{}) string {
	hash := sha1.New()
	for i := range objs {
		hash.Write([]byte(fmt.Sprintf("%#v", objs[i])))
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// ShouldReturn is a helper function that checks a reconcile Result and an error, to determine if Reconcile() should
// return now. Returns "true" if error is not nil, or any requeueing is set on the Result.
func ShouldReturn(result reconcile.Result, err error) bool {
	return err != nil || result.Requeue || result.RequeueAfter != 0
}

func DefaultDeploymentStrategy() appsv1.DeploymentStrategy {
	twentyFivePercent := intstr.FromString("25%")

	return appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &twentyFivePercent,
			MaxSurge:       &twentyFivePercent,
		},
	}
}

func MeetsVersionConstraint(constraint, version string) bool {
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		return false
	}

	if c.Check(v) {
		return true
	}

	return false
}
