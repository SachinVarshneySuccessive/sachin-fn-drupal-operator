package integration

import (
	"context"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/google/uuid"
	. "github.com/smartystreets/goconvey/convey"

	fn "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
)

func TestDatabaseValidatingWebhook(t *testing.T) {
	Convey("Given a client to an active cluster", t, func() {
		c, err := NewRealClient()
		So(err, ShouldBeNil)

		name := strings.ToLower(t.Name())

		ns := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		// throw away error in case this ns already exists
		// if the error was something else and it wasn't created,
		// the following tests will fail with an error that will
		// inform us of that.
		_ = c.Create(context.TODO(), ns)

		Convey("Creating a Database with a User longer than 16 chars should fail", func() {
			err := c.Create(context.TODO(), &fn.Database{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: name,
					Namespace:    ns.Name,
				},
				Spec: fn.DatabaseSpec{
					User: "thisislongerthansixteencharacters",
				},
			})
			So(err, ShouldBeError)
			So(err.Error(), ShouldEqual, "admission webhook \"databases.fnresources.acquia.io\" denied the request: user 'thisislongerthansixteencharacters' too many characters (33 > 16)")
		})

		Convey("Creating a Database with a User less than 16 chars should succeed", func() {
			dbname := uuid.New().String() // need unique predictable name
			db := &fn.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dbname,
					Namespace: ns.Name,
				},
				Spec: fn.DatabaseSpec{
					User:       "shortname",
					SchemaName: "validschema",
				},
			}
			err := c.Create(context.TODO(), db)
			So(err, ShouldBeNil)
			// refresh obj
			So(c.Get(context.TODO(), types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, db), ShouldBeNil)

			Convey("Updating user field is not allowed", func() {
				err := update(c, db, func() {
					db.Spec.User = "newname"
				})
				So(err, ShouldBeError)
				So(err.Error(), ShouldEqual, "admission webhook \"databases.fnresources.acquia.io\" denied the request: user field is immutable")
			})

			Convey("Updating schemaName field is not allowed", func() {
				err := update(c, db, func() {
					db.Spec.SchemaName = "newname"
				})
				So(err, ShouldBeError)
				So(err.Error(), ShouldEqual, "admission webhook \"databases.fnresources.acquia.io\" denied the request: schemaName field is immutable")
			})
		})
	})
}

func TestDatabaseDefaultingWebhook(t *testing.T) {
	Convey("Given a client to an active cluster", t, func() {
		c, err := NewRealClient()
		So(err, ShouldBeNil)

		name := strings.ToLower(t.Name())

		ns := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		// throw away error in case this ns already exists
		// if the error was something else and it wasn't created,
		// the following tests will fail with an error that will
		// inform us of that.
		_ = c.Create(context.TODO(), ns)

		Convey("An empty Database should have defaults applied", func() {
			db := &fn.Database{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: name,
					Namespace:    ns.Name,
				},
			}
			err := c.Create(context.TODO(), db)
			So(err, ShouldBeNil)

			err = c.Get(context.TODO(), types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, db)
			So(err, ShouldBeNil)
			So(db.Spec.Port, ShouldEqual, 3306)
		})
	})
}

func update(c client.Client, obj runtime.Object, change func()) error {
	for {
		change()
		err := c.Update(context.TODO(), obj)
		if !errors.IsConflict(err) {
			return err
		}
		key, err := client.ObjectKeyFromObject(obj)
		if err != nil {
			return err
		}
		err = c.Get(context.TODO(), key, obj)
		if err != nil {
			return err
		}
	}
}
