export BASE_SPACE=$(shell pwd)
export BUILD_SPACE=$(BASE_SPACE)/build

VERSION = $(shell echo `git describe --tag --dirty``git status --porcelain 2>/dev/null| grep -q "^??" &&echo '-untracked'`)
VERSION := $(shell echo ${VERSION} | sed -e "s/^v//")
nightly-release: VERSION := $(shell echo ${VERSION}-nightly-build)
nightly-release-mac: VERSION := $(shell echo ${VERSION}-nightly-build)
# In case building outside of a git repo, use the version presented in the CWAGENT_VERSION file as a fallback
ifeq ($(VERSION),)
VERSION := `cat CWAGENT_VERSION`
endif

# Determine agent build mode, default to PIE mode
ifndef CWAGENT_BUILD_MODE
CWAGENT_BUILD_MODE=default
endif

BUILD := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -s -w
LDFLAGS +=  -X github.com/aws/amazon-cloudwatch-agent/cfg/agentinfo.VersionStr=${VERSION}
LDFLAGS +=  -X github.com/aws/amazon-cloudwatch-agent/cfg/agentinfo.BuildStr=${BUILD}
LINUX_AMD64_BUILD = CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildmode=${CWAGENT_BUILD_MODE} -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/linux_amd64
LINUX_ARM64_BUILD = CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildmode=${CWAGENT_BUILD_MODE} -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/linux_arm64
WIN_BUILD = GOOS=windows GOARCH=amd64 go build -trimpath -buildmode=${CWAGENT_BUILD_MODE} -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/windows_amd64
DARWIN_BUILD_AMD64 = CGO_ENABLED=1 GO111MODULE=on GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/darwin_amd64
DARWIN_BUILD_ARM64 = CGO_ENABLED=1 GO111MODULE=on GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/darwin_arm64

IMAGE_REGISTRY = amazon
IMAGE_REPO = cloudwatch-agent
IMAGE_TAG = $(VERSION)
IMAGE = $(IMAGE_REGISTRY)/$(IMAGE_REPO):$(IMAGE_TAG)
DOCKER_BUILD_FROM_SOURCE = docker build -t $(IMAGE) -f ./amazon-cloudwatch-container-insights/cloudwatch-agent-dockerfile/source/Dockerfile
DOCKER_WINDOWS_BUILD_FROM_SOURCE = docker build -t $(IMAGE) -f ./amazon-cloudwatch-container-insights/cloudwatch-agent-dockerfile/source/Dockerfile.Windows

CW_AGENT_IMPORT_PATH=github.com/aws/amazon-cloudwatch-agent
ALL_SRC := $(shell find . -name '*.go' -type f | sort)
TOOLS_BIN_DIR := $(abspath ./build/tools)

GOIMPORTS_OPT?= -w -local $(CW_AGENT_IMPORT_PATH)

GOIMPORTS = $(TOOLS_BIN_DIR)/goimports
SHFMT = $(TOOLS_BIN_DIR)/shfmt
LINTER = $(TOOLS_BIN_DIR)/golangci-lint
IMPI = $(TOOLS_BIN_DIR)/impi
ADDLICENSE = $(TOOLS_BIN_DIR)/addlicense

prepackage: clean test build
release: prepackage package-rpm package-deb package-win package-darwin
nightly-release: prepackage package-rpm package-deb package-win
nightly-release-mac: prepackage package-darwin

build: check_secrets amazon-cloudwatch-agent-linux amazon-cloudwatch-agent-darwin amazon-cloudwatch-agent-windows

check_secrets::
	if grep --exclude-dir=build --exclude-dir=vendor -exclude=integration/msi/tools/amazon-cloudwatch-agent.wxs -E "(A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}|(\"|')?(AWS|aws|Aws)?_?(SECRET|secret|Secret)?_?(ACCESS|access|Access)?_?(KEY|key|Key)(\"|')?\\s*(:|=>|=)\\s*(\"|')?[A-Za-z0-9/\\+=]{40}(\"|')?" -Rn .; then echo "check_secrets failed"; exit 1; fi;

create-version-file:
	@echo Version: ${VERSION}
	@echo Building time: ${BUILD}
	echo "$(VERSION)" > CWAGENT_VERSION

copy-version-file: create-version-file
	mkdir -p build/bin/
	cp CWAGENT_VERSION $(BUILD_SPACE)/bin/CWAGENT_VERSION

amazon-cloudwatch-agent-linux: copy-version-file
	@echo Building CloudWatchAgent for Linux,Debian with ARM64 and AMD64
	$(LINUX_AMD64_BUILD)/config-downloader github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader
	$(LINUX_ARM64_BUILD)/config-downloader github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader
	$(LINUX_AMD64_BUILD)/config-translator github.com/aws/amazon-cloudwatch-agent/cmd/config-translator
	$(LINUX_ARM64_BUILD)/config-translator github.com/aws/amazon-cloudwatch-agent/cmd/config-translator
	$(LINUX_AMD64_BUILD)/amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
	$(LINUX_ARM64_BUILD)/amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
	$(LINUX_AMD64_BUILD)/start-amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
	$(LINUX_ARM64_BUILD)/start-amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
	$(LINUX_AMD64_BUILD)/amazon-cloudwatch-agent-config-wizard github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard
	$(LINUX_ARM64_BUILD)/amazon-cloudwatch-agent-config-wizard github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard


amazon-cloudwatch-agent-darwin: copy-version-file
ifneq ($(OS),Windows_NT)
ifeq ($(shell uname -s),Darwin)
	@echo Building CloudWatchAgent for MacOS with ARM64 and AMD64
	$(DARWIN_BUILD_AMD64)/config-downloader github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader
	$(DARWIN_BUILD_ARM64)/config-downloader github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader
	$(DARWIN_BUILD_AMD64)/config-translator github.com/aws/amazon-cloudwatch-agent/cmd/config-translator
	$(DARWIN_BUILD_ARM64)/config-translator github.com/aws/amazon-cloudwatch-agent/cmd/config-translator
	$(DARWIN_BUILD_AMD64)/amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
	$(DARWIN_BUILD_ARM64)/amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
	$(DARWIN_BUILD_AMD64)/start-amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
	$(DARWIN_BUILD_ARM64)/start-amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
	$(DARWIN_BUILD_AMD64)/amazon-cloudwatch-agent-config-wizard github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard
	$(DARWIN_BUILD_ARM64)/amazon-cloudwatch-agent-config-wizard github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard
endif
endif

amazon-cloudwatch-agent-windows:
	@echo Building CloudWatchAgent for Windows with AMD64
	$(WIN_BUILD)/config-downloader.exe github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader
	$(WIN_BUILD)/config-translator.exe github.com/aws/amazon-cloudwatch-agent/cmd/config-translator
	$(WIN_BUILD)/amazon-cloudwatch-agent.exe github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
	$(WIN_BUILD)/start-amazon-cloudwatch-agent.exe github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
	$(WIN_BUILD)/amazon-cloudwatch-agent-config-wizard.exe github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard

# A fast build that only builds amd64, we don't need wizard and config downloader
build-for-docker: build-for-docker-amd64

build-for-docker-amd64:
	$(LINUX_AMD64_BUILD)/amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
	$(LINUX_AMD64_BUILD)/start-amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
	$(LINUX_AMD64_BUILD)/config-translator github.com/aws/amazon-cloudwatch-agent/cmd/config-translator

build-for-docker-windows-amd64:
	$(WIN_BUILD)/amazon-cloudwatch-agent.exe github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
	$(WIN_BUILD)/start-amazon-cloudwatch-agent.exe github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
	$(WIN_BUILD)/config-translator.exe github.com/aws/amazon-cloudwatch-agent/cmd/config-translator

build-for-docker-arm64:
	$(LINUX_ARM64_BUILD)/amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
	$(LINUX_ARM64_BUILD)/start-amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
	$(LINUX_ARM64_BUILD)/config-translator github.com/aws/amazon-cloudwatch-agent/cmd/config-translator

docker-build: build-for-docker-amd64 build-for-docker-arm64
	docker buildx build --platform linux/amd64,linux/arm64 . -f amazon-cloudwatch-container-insights/cloudwatch-agent-dockerfile/localbin/Dockerfile -t $(IMAGE)

docker-build-amd64: build-for-docker-amd64
	docker buildx build --platform linux/amd64 . -f amazon-cloudwatch-container-insights/cloudwatch-agent-dockerfile/localbin/Dockerfile -t $(IMAGE) --load

docker-build-arm64: build-for-docker-arm64
	docker buildx build --platform linux/arm64 . -f amazon-cloudwatch-container-insights/cloudwatch-agent-dockerfile/localbin/Dockerfile -t $(IMAGE) --load

docker-push:
	docker push $(IMAGE)

build-for-docker-fast-windows-amd64: build-for-docker-windows-amd64
	rm -rf tmp
	mkdir -p tmp/windows_amd64
	cp build/bin/windows_amd64/* tmp/windows_amd64
	docker build --platform windows/amd64 -f amazon-cloudwatch-container-insights/cloudwatch-agent-dockerfile/localbin/Dockerfile.Windows . -t amazon-cloudwatch-agent
	rm -rf tmp

install-goimports:
	GOBIN=$(TOOLS_BIN_DIR) go install golang.org/x/tools/cmd/goimports

install-shfmt:
	GOBIN=$(TOOLS_BIN_DIR) go install mvdan.cc/sh/v3/cmd/shfmt@latest

install-impi:
	GOBIN=$(TOOLS_BIN_DIR) go install github.com/pavius/impi/cmd/impi@v0.0.3

install-addlicense:
	# Using 04bfe4e to get SPDX template changes that are not present in the most recent tag v1.0.0
	# This is required to be able to easily omit the year in our license header.
	GOBIN=$(TOOLS_BIN_DIR) go install github.com/google/addlicense@04bfe4e

install-golangci-lint:
	#Install from source for golangci-lint is not recommended based on https://golangci-lint.run/usage/install/#install-from-source so using binary
	#installation
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(TOOLS_BIN_DIR) v1.50.1

fmt: install-goimports addlicense
	go fmt ./...
	@echo $(ALL_SRC) | xargs -n 10 $(GOIMPORTS) $(GOIMPORTS_OPT)

fmt-sh: install-shfmt
	${SHFMT} -w -d -i 5 .

impi: install-impi
	# Skip plugins/plugins.go
	@echo $(ALL_SRC) | xargs -n 10 $(IMPI) --local $(CW_AGENT_IMPORT_PATH) --scheme stdThirdPartyLocal --skip plugins/plugins.go
	@echo "Check import order/grouping finished"

addlicense: install-addlicense
	@ADDLICENSEOUT=`$(ADDLICENSE) -y="" -s=only -l="mit" -c="Amazon.com, Inc. or its affiliates. All Rights Reserved." $(ALL_SRC) 2>&1`; \
    		if [ "$$ADDLICENSEOUT" ]; then \
    			echo "$(ADDLICENSE) FAILED => add License errors:\n"; \
    			echo "$$ADDLICENSEOUT\n"; \
    			exit 1; \
    		else \
    			echo "Add License finished successfully"; \
    		fi

checklicense: install-addlicense
	@ADDLICENSEOUT=`$(ADDLICENSE) -check $(ALL_SRC) 2>&1`; \
    		if [ "$$ADDLICENSEOUT" ]; then \
    			echo "$(ADDLICENSE) FAILED => add License errors:\n"; \
    			echo "$$ADDLICENSEOUT\n"; \
    			echo "Use 'make addlicense' to fix this."; \
    			exit 1; \
    		else \
    			echo "Check License finished successfully"; \
    		fi

simple-lint: checklicense impi

lint: install-golangci-lint simple-lint
	${LINTER} run ./...

test:
	CGO_ENABLED=0 go test -timeout 15m -coverprofile coverage.txt -failfast ./...

clean::
	rm -rf release/ build/
	rm -f CWAGENT_VERSION

package-prepare-rpm:
	# amd64 rpm
	mkdir -p $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg
	cp $(BUILD_SPACE)/bin/linux_amd64/* $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/* $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/RELEASE_NOTES $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BUILD_SPACE)/bin/CWAGENT_VERSION $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/dependencies/amazon-cloudwatch-agent-ctl $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/dependencies/amazon-cloudwatch-agent.service $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/cfg/commonconfig/common-config.toml $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/linux/amazon-cloudwatch-agent.conf $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/linux/amazon-cloudwatch-agent.spec $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/translator/config/schema.json $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/amazon-cloudwatch-agent-schema.json

	# arm64 rpm
	mkdir -p $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg
	cp $(BUILD_SPACE)/bin/linux_arm64/* $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/* $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/RELEASE_NOTES $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BUILD_SPACE)/bin/CWAGENT_VERSION $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/dependencies/amazon-cloudwatch-agent-ctl $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/dependencies/amazon-cloudwatch-agent.service $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/cfg/commonconfig/common-config.toml $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/linux/amazon-cloudwatch-agent.conf $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/linux/amazon-cloudwatch-agent.spec $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/translator/config/schema.json $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/amazon-cloudwatch-agent-schema.json
	cp -rf $(BASE_SPACE)/Tools $(BUILD_SPACE)/

package-prepare-deb:
	# amd64 deb
	mkdir -p $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg
	cp $(BUILD_SPACE)/bin/linux_amd64/* $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/* $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/RELEASE_NOTES $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BUILD_SPACE)/bin/CWAGENT_VERSION $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/dependencies/amazon-cloudwatch-agent-ctl $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/dependencies/amazon-cloudwatch-agent.service $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/cfg/commonconfig/common-config.toml $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/linux/amazon-cloudwatch-agent.conf $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/translator/config/schema.json $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/amazon-cloudwatch-agent-schema.json

	# arm64 deb
	mkdir -p $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg
	cp $(BUILD_SPACE)/bin/linux_arm64/* $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/* $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/RELEASE_NOTES $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BUILD_SPACE)/bin/CWAGENT_VERSION $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/dependencies/amazon-cloudwatch-agent-ctl $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/dependencies/amazon-cloudwatch-agent.service $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/cfg/commonconfig/common-config.toml $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/linux/amazon-cloudwatch-agent.conf $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/translator/config/schema.json $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/amazon-cloudwatch-agent-schema.json

	cp -rf $(BASE_SPACE)/Tools $(BUILD_SPACE)/
	cp -rf $(BASE_SPACE)/packaging $(BUILD_SPACE)/

package-prepare-win-zip:
	# amd64 win
	mkdir -p $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg
	cp $(BUILD_SPACE)/bin/windows_amd64/* $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/* $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/RELEASE_NOTES $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp $(BUILD_SPACE)/bin/CWAGENT_VERSION $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/cfg/commonconfig/common-config.toml $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/translator/config/schema.json $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/amazon-cloudwatch-agent-schema.json
	cp ${BASE_SPACE}/packaging/windows/amazon-cloudwatch-agent-ctl.ps1 $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp ${BASE_SPACE}/packaging/windows/install.ps1 $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp ${BASE_SPACE}/packaging/windows/uninstall.ps1 $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp -rf $(BASE_SPACE)/Tools $(BUILD_SPACE)/

package-prepare-darwin-tar:
	# amd64 darwin
	mkdir -p $(BUILD_SPACE)/private/darwin/amd64/tar/amazon-cloudwatch-agent-pre-pkg
	cp $(BUILD_SPACE)/bin/darwin_amd64/* $(BUILD_SPACE)/private/darwin/amd64/tar/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/* $(BUILD_SPACE)/private/darwin/amd64/tar/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/RELEASE_NOTES $(BUILD_SPACE)/private/darwin/amd64/tar/amazon-cloudwatch-agent-pre-pkg/
	cp $(BUILD_SPACE)/bin/CWAGENT_VERSION $(BUILD_SPACE)/private/darwin/amd64/tar/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/cfg/commonconfig/common-config.toml $(BUILD_SPACE)/private/darwin/amd64/tar/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/translator/config/schema.json $(BUILD_SPACE)/private/darwin/amd64/tar/amazon-cloudwatch-agent-pre-pkg/amazon-cloudwatch-agent-schema.json
	cp $(BASE_SPACE)/packaging/darwin/amazon-cloudwatch-agent-ctl $(BUILD_SPACE)/private/darwin/amd64/tar/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/darwin/com.amazon.cloudwatch.agent.plist $(BUILD_SPACE)/private/darwin/amd64/tar/amazon-cloudwatch-agent-pre-pkg/

	# arm64 darwin
	mkdir -p $(BUILD_SPACE)/private/darwin/arm64/tar/amazon-cloudwatch-agent-pre-pkg
	cp $(BUILD_SPACE)/bin/darwin_arm64/* $(BUILD_SPACE)/private/darwin/arm64/tar/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/* $(BUILD_SPACE)/private/darwin/arm64/tar/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/RELEASE_NOTES $(BUILD_SPACE)/private/darwin/arm64/tar/amazon-cloudwatch-agent-pre-pkg/
	cp $(BUILD_SPACE)/bin/CWAGENT_VERSION $(BUILD_SPACE)/private/darwin/arm64/tar/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/cfg/commonconfig/common-config.toml $(BUILD_SPACE)/private/darwin/arm64/tar/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/translator/config/schema.json $(BUILD_SPACE)/private/darwin/arm64/tar/amazon-cloudwatch-agent-pre-pkg/amazon-cloudwatch-agent-schema.json
	cp $(BASE_SPACE)/packaging/darwin/amazon-cloudwatch-agent-ctl $(BUILD_SPACE)/private/darwin/arm64/tar/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/darwin/com.amazon.cloudwatch.agent.plist $(BUILD_SPACE)/private/darwin/arm64/tar/amazon-cloudwatch-agent-pre-pkg/

	cp -rf $(BASE_SPACE)/Tools $(BUILD_SPACE)/

.PHONY: package-rpm
package-rpm: package-prepare-rpm
	ARCH=amd64 TARGET_SUPPORTED_ARCH=x86_64 PREPKGPATH="$(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg" $(BUILD_SPACE)/Tools/src/create_rpm.sh
	ARCH=arm64 TARGET_SUPPORTED_ARCH=aarch64 PREPKGPATH="$(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg" $(BUILD_SPACE)/Tools/src/create_rpm.sh

.PHONY: package-deb
package-deb: package-prepare-deb
	ARCH=amd64 TARGET_SUPPORTED_ARCH=x86_64 PREPKGPATH="$(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg" $(BUILD_SPACE)/Tools/src/create_deb.sh
	ARCH=arm64 TARGET_SUPPORTED_ARCH=aarch64 PREPKGPATH="$(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg" $(BUILD_SPACE)/Tools/src/create_deb.sh

.PHONY: package-win
package-win: package-prepare-win-zip
	ARCH=amd64 TARGET_SUPPORTED_ARCH=x86_64 PREPKGPATH="$(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg" $(BUILD_SPACE)/Tools/src/create_win.sh

.PHONY: package-darwin
package-darwin: package-prepare-darwin-tar
	ARCH=amd64 TARGET_SUPPORTED_ARCH=x86_64 PREPKGPATH="$(BUILD_SPACE)/private/darwin/amd64/tar/amazon-cloudwatch-agent-pre-pkg" $(BUILD_SPACE)/Tools/src/create_darwin.sh
	ARCH=arm64 TARGET_SUPPORTED_ARCH=aarch64 PREPKGPATH="$(BUILD_SPACE)/private/darwin/arm64/tar/amazon-cloudwatch-agent-pre-pkg" $(BUILD_SPACE)/Tools/src/create_darwin.sh

.PHONY: fmt fmt-sh build test clean

.PHONY: dockerized-build dockerized-build-vendor
dockerized-build:
	$(DOCKER_BUILD_FROM_SOURCE) .
	@echo Built image:
	@echo $(IMAGE)

# Use vendor instead of proxy when building w/ vendor folder
dockerized-build-vendor:
	$(DOCKER_BUILD_FROM_SOURCE) --build-arg GO111MODULE=off .

.PHONY: dockerized-windows-build
dockerized-windows-build:
	$(DOCKER_WINDOWS_BUILD_FROM_SOURCE) .
	@echo Built image:
	@echo $(IMAGE)