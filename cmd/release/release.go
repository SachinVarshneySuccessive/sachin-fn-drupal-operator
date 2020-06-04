package main

import (
	"github.com/acquia/fn-go-utils/pkg/release"
)

func main() {
	release.ExecuteRelease(release.Project{
		Name:              "fn-drupal-operator",
		GitRepo:           "git@github.com:acquia/fn-drupal-operator.git",
		HelmImageTagPaths: [][2]string{{"image", "tag"}},
	})
}
