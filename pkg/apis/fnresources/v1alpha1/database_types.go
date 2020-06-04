package v1alpha1

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/acquia/fn-go-utils/pkg/operatorutils"
)

// Important: Run "operator-sdk generate k8s && operator-sdk generate crds" to regenerate code after modifying this file
// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

// DatabaseSpec defines the desired state of Database
// +k8s:openapi-gen=true
type DatabaseSpec struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	SchemaName  string `json:"schemaName"`
	User        string `json:"user"`
	AdminSecret string `json:"adminSecret,omitempty"` // +optional
	UserSecret  string `json:"userSecret"`
}

// DatabaseStatus defines the observed state of Database
// +k8s:openapi-gen=true
type DatabaseStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Database is the Schema for the databases API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseSpec   `json:"spec,omitempty"`
	Status DatabaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DatabaseList contains a list of Database
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Database{}, &DatabaseList{})
}

// DBChildLabels is map of database child labels
var DBChildLabels = []string{
	ApplicationIdLabel,
	SiteIdLabel,
	EnvironmentIdLabel,
	DatabaseIdLabel,
}

var _ operatorutils.ResourceWithId = &Database{}

func (d *Database) NewList() runtime.Object {
	return &DatabaseList{}
}

func (d *Database) IdLabel() string {
	return DatabaseIdLabel
}

func (d *Database) Id() string {
	return d.GetLabels()[DatabaseIdLabel]
}

func (d *Database) SetId(value string) {
	if d.GetLabels() == nil {
		d.SetLabels(map[string]string{})
	}
	d.ObjectMeta.Labels[DatabaseIdLabel] = value
}

// ChildLabels returns map of database labels
func (d Database) ChildLabels() map[string]string {
	dbLabels := d.GetLabels()
	if dbLabels == nil {
		return nil
	}

	ls := make(map[string]string, len(DBChildLabels))
	for _, val := range DBChildLabels {
		ls[val] = dbLabels[val]
	}

	return ls
}

// DatabaseName returns name of the database schema
func (d Database) DatabaseName() string {
	return d.Spec.SchemaName
}

// ConnectionConfig intended to replace pkg/common/Database
type ConnectionConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Name     string `json:"database"`
	User     string `json:"user"`
	Password string `json:"pass"`
}

// GetConnectionConfig returns a database connecton config
func (d *Database) GetConnectionConfig(c client.Client) (ConnectionConfig, error) {
	pwdSecret, err := d.getUserSecret(c)
	if err != nil {
		return ConnectionConfig{}, err
	}

	passwd := string(pwdSecret.Data["password"])

	return ConnectionConfig{
		Host:     d.Spec.Host,
		Port:     d.Spec.Port,
		Name:     d.DatabaseName(),
		User:     d.Spec.User,
		Password: passwd,
	}, nil
}

// GetUser returns database user from user secret
func (d Database) GetUser(c client.Client) (string, error) {
	config, err := d.GetConnectionConfig(c)
	if err != nil {
		return "", err
	}
	return config.User, nil
}

// GetPassword returns the database password string. This is used by site controller to populate env-config secret with
// database creds used by customer application (Drupal)
func (d *Database) GetPassword(c client.Client) (string, error) {
	config, err := d.GetConnectionConfig(c)
	if err != nil {
		return "", err
	}
	return config.Password, nil
}

// GetUserSecret returns database user credentials secret object
func (d *Database) getUserSecret(c client.Client) (*corev1.Secret, error) {
	pwdSecret := &corev1.Secret{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: d.Spec.UserSecret, Namespace: d.Namespace}, pwdSecret); err != nil {
		return nil, err
	}
	return pwdSecret, nil
}

// getAdminUser returns mysql admin user from admin secret
func (d Database) getAdminUser(c client.Client) (string, error) {
	config, err := d.GetAdminConnectionConfig(c)
	if err != nil {
		return "", err
	}
	return config.User, nil
}

// GetAdminPassword returns the database password string. This is used by site controller to populate env-config secret
// with database creds used by customer application (Drupal)
func (d *Database) GetAdminPassword(c client.Client) (string, error) {
	config, err := d.GetAdminConnectionConfig(c)
	if err != nil {
		return "", err
	}
	return config.Password, nil
}

// GetAdminConnectionConfig returns admin database of database CR
func (d *Database) GetAdminConnectionConfig(c client.Client) (ConnectionConfig, error) {
	dbAdminSecret, err := d.GetAdminSecret(c)
	if err != nil {
		return ConnectionConfig{}, err
	}

	db := ConnectionConfig{}

	data := dbAdminSecret.Data

	db.User = string(data["username"])
	db.Password = string(data["password"])
	db.Host = d.Spec.Host
	db.Port = d.Spec.Port

	return db, nil
}

// GetAdminSecret returns database user credentials secret object
func (d *Database) GetAdminSecret(c client.Client) (*corev1.Secret, error) {
	pwdSecret := &corev1.Secret{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: d.Spec.AdminSecret, Namespace: d.Namespace}, pwdSecret); err != nil {
		return pwdSecret, err
	}
	return pwdSecret, nil
}

// GetConnectionFromConfig returns a mysql connection
func (db ConnectionConfig) GetConnectionFromConfig() (*sql.DB, error) {
	config := mysql.NewConfig()
	config.User = db.User
	config.Passwd = db.Password
	config.DBName = db.Name
	config.Net = "tcp"
	portInt := strconv.Itoa(db.Port)
	config.Addr = net.JoinHostPort(db.Host, portInt)
	config.Timeout = time.Second * 5
	conn, err := sql.Open("mysql", config.FormatDSN())

	if err != nil {
		return nil, err
	}
	conn.SetConnMaxLifetime(time.Second * 10)
	return conn, err
}

// GetAdminConnection returns connection to admin mysql
func (d Database) GetAdminConnection(c client.Client) (*sql.DB, error) {
	db, err := d.GetAdminConnectionConfig(c)
	if err != nil {
		return nil, err
	}
	return db.GetConnectionFromConfig()
}

////////////////
//  Webhooks  //
////////////////

var _ webhook.Validator = &Database{}
var _ webhook.Defaulter = &Database{}

func (d *Database) ValidateCreate() error {
	log := logf.Log.WithName("databasevalidator").WithValues("operation", "create")
	userLength := len(d.Spec.User)
	if userLength > 16 {
		err := fmt.Errorf("user '%s' too many characters (%d > 16)", d.Spec.User, userLength)
		log.Info(err.Error())
		return err
	}
	return nil
}

func (d *Database) ValidateUpdate(old runtime.Object) error {
	log := logf.Log.WithName("databasevalidator").WithValues("operation", "update")
	oldd, ok := old.(*Database)
	if !ok {
		return fmt.Errorf("invalid old object passed.")
	}

	if d.Spec.User != oldd.Spec.User {
		err := fmt.Errorf("user field is immutable")
		log.Info(err.Error())
		return err
	}
	if d.Spec.SchemaName != oldd.Spec.SchemaName {
		err := fmt.Errorf("schemaName field is immutable")
		log.Info(err.Error())
		return err
	}
	return nil
}

func (d *Database) ValidateDelete() error {
	return nil
}

func (d *Database) Default() {
	log := logf.Log.WithName("databasedefaulter")
	if d.Spec.Port == 0 {
		log.Info("Defaulting port to 3306")
		d.Spec.Port = 3306
	}
}

/////////////////////
//  Migration code  //
/////////////////////

var _ versionedType = &Database{}

func (d *Database) SpecVersion() string {
	return "2"
}

func (d *Database) migrationNeeded() bool {
	return false
}

func (d *Database) doMigrate() {}
