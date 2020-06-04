include fn-targets.mk

NAME = fn-drupal-operator
TAG := $(shell whoami)
ECR := 881217801864.dkr.ecr.us-east-1.amazonaws.com
FULL_IMAGE_NAME := ${ECR}/${NAME}:${TAG}
BUFFER := $(shell mktemp)

OPERATOR_BINARY = build/_output/bin/fn-drupal-operator
OPERATOR_FILES = $(shell find pkg -type f)
TYPES = $(shell find pkg/apis/fnresources/ -name *_types.go)
CRDS = $(shell ls deploy/crds/*_crd.yaml)

DEPLOY = $(shell find deploy/ -type f )
HELM_FILES = $(shell ls helm/fn-drupal-operator/*.yaml)
CHART = helm/fn-drupal-operator/templates

# This is a pattern for letting make know that it needs to build a docker image
# even though the files that image depends on did not change.  In other words,
# it's a way to trigger rebuilding the image based on the image and tag
# parameters.  Targets for each of these can be found at the bottom of the file.
#
# We look for a file with the image and tag of the current build.  If it
# exists, then the image and tag are the same as the last build.  So we only
# need to rebuild the image if the files it depends on have changed.  If the
# file does not exist, then the image we're being asked to build has not
# immediately been generated, so we must make it again even if none of the
# files it depends on have changed.  i.e. we want to build with a new tag
LAST = $(subst :,,.last_${ECR}${NAME}${TAG}.makefiledata)

export GO111MODULE = auto
export GOPRIVATE = github.com/acquia

.PHONY: all
all: build helm

# Convenience target
.PHONY: build-images
build-images: ${OPERATOR_BINARY}

.PHONY: publish-images
publish-images: build-images ${OPERATOR_BINARY}
	docker push ${FULL_IMAGE_NAME}
ifeq ($(LATEST),1)
	docker tag ${FULL_IMAGE_NAME} ${ECR}/${NAME}:latest
	docker push ${ECR}/${NAME}:latest
endif

.PHONY: helm
helm: ${CHART}

.PHONY: lint
lint:
	gofmt -l . | tee $(BUFFER)
	@! test -s $(BUFFER)
	cd helm \
	  && ./package.sh \
	  && cd fn-drupal-operator \
	  && helm lint

.PHONY: lint-fix
lint-fix:
	gofmt -w .

.PHONY: test
test:
	go test ./pkg/... -coverprofile=coverage.out -tags test -failfast

.PHONY: test-regolden
test-regolden:
	$(info ***)
	$(info *** Updating golden (testdata) files. Be sure to manually verify them via 'git diff'!)
	$(info ***)
	find pkg/ -name '*.golden' -delete
	go test ./pkg/controller/... ./pkg/customercontainer -update -failfast

	@echo "***"
	@echo "*** Verifying new Golden files match"
	@echo "***"
	make test

.PHONY: test-integration
test-integration:
	go test ./integration/ -v -count=1

.PHONY: test-clean
test-clean:
	kubectl get ns -o name | grep "namespace/test" | xargs -r -n1 kubectl delete --wait=false
	kubectl get ns

.PHONY: coverage
coverage: test
	go tool cover -html=coverage.out


${OPERATOR_BINARY}: cmd/manager/ ${OPERATOR_FILES} ${LAST} ${CRDS}
	operator-sdk build ${FULL_IMAGE_NAME}

${CHART}: ${DEPLOY} ${HELM_FILES} ./helm/package.sh
	cd helm && \
	./package.sh

${CRDS}: ${TYPES}
	operator-sdk generate k8s
	operator-sdk generate crds

${LAST}:
	@rm .last_*.makefiledata 2> /dev/null || true
	@touch ${LAST}


#####################
#  Release Targets  #
#####################

.PHONY: build-image-${NAME}
build-image-${NAME}: build

.PHONY: build-image
build-image: build-image-${NAME}

.PHONY: publish-images
publish-image: build-image-${NAME} push
