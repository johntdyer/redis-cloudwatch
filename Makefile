#
#  Makefile for Go
#
GO_CMD=go
GO_BUILD=$(GO_CMD) build
GO_BUILD_RACE=$(GO_CMD) build -race
GO_TEST=$(GO_CMD) test
GO_TEST_VERBOSE=$(GO_CMD) test -v
GO_INSTALL=$(GO_CMD) install -v
GO_CLEAN=$(GO_CMD) clean
GO_DEPS=$(GO_CMD) get -d -v
GO_DEPS_UPDATE=$(GO_CMD) get -d -v -u
GO_VET=$(GO_CMD) vet
GO_FMT=$(GO_CMD) fmt
GO_LINT=golint

# Packages
PACKAGE_LIST := redis.go version.go cloud_watch.go redis-cloudwatch.go structs.go

.PHONY: all build install clean fmt vet lint

all: build

build: vet
		echo "==> Build package ...";
		$(GO_BUILD) redis-cloudwatch.go redis.go version.go cloud_watch.go redis-cloudwatch.go structs.go || exit 1;

build-race: vet

install:
	@for p in $(PACKAGE_LIST); do \
		echo "==> Install $$p ..."; \
		$(GO_INSTALL) $$p || exit 1; \
	done

clean:
	@for p in $(PACKAGE_LIST); do \
		echo "==> Clean $$p ..."; \
		$(GO_CLEAN) $$p; \
	done

fmt:
	@for p in $(PACKAGE_LIST); do \
		echo "==> Formatting $$p ..."; \
		$(GO_FMT) $$p || exit 1; \
	done
vet:
	@for p in $(PACKAGE_LIST); do \
		echo "==> Vet $$p ..."; \
		$(GO_VET) $$p; \
	done

lint:
	@for p in $(PACKAGE_LIST); do \
		echo "==> Lint $$p ..."; \
		$(GO_LINT) src/$$p; \
	done

# vim: set noexpandtab shiftwidth=8 softtabstop=0:
