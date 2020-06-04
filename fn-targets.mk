# Shared make targets for fn-* / NW projects.
# https://github.com/acquia/fn-go-utils

# Sane defaults, can be overridden easliy.
export GOPRIVATE = github.com/acquia

ifndef ECR
ECR := 881217801864.dkr.ecr.us-east-1.amazonaws.com
endif

# Defer default goal to the includer unless it's already set.
ifeq ($(.DEFAULT_GOAL),)
CLEAR_DEFAULT_GOAL = 1
endif

# List all available targets.
# Based on: http://stackoverflow.com/a/26339924
.PHONY: targets
targets:
	@$(MAKE) -pRrq -f $(lastword $(MAKEFILE_LIST)) : 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | sort | egrep -v -e '^[^[:alnum:]]' -e '^$@$$'

.PHONY: docker-login
docker-login:
	@docker login $(ECR) < /dev/null > /dev/null || aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin $(ECR)

.PHONY: update-fn-targets
update-fn-targets:
	test -d ../fn-go-utils || git clone git@github.com:acquia/fn-go-utils.git ../fn-go-utils && \
	cd ../fn-go-utils && \
	git fetch git@github.com:acquia/fn-go-utils.git master && \
	git worktree add -f ../fn-go-utils-temp FETCH_HEAD && \
	cd - && \
	cp ../fn-go-utils-temp/fn-targets.mk . && \
	cd -  && \
	git worktree remove ../fn-go-utils-temp

ifeq ($(CLEAR_DEFAULT_GOAL),1)
.DEFAULT_GOAL :=
endif
