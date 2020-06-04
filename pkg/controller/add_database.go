// +build !test

package controller

import (
	"github.com/acquia/fn-drupal-operator/pkg/controller/database"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, database.Add)
}
