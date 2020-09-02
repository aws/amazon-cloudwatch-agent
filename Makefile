export BASE_SPACE=$(shell pwd)
export BUILD_SPACE=$(BASE_SPACE)/build

VERSION = $(shell echo `git describe --tag --dirty``git status --porcelain 2>/dev/null| grep -q "^??" &&echo '-untracked'`)
VERSION := $(shell echo ${VERSION} | sed -e "s/^v//")
# In case building outside of a git repo, use the version presented in the CWAGENT_VERSION file as a fallback
ifeq ($(VERSION),)
VERSION := `cat CWAGENT_VERSION`
endif
BUILD = $(shell date --iso-8601=seconds)
LDFLAGS = -s -w
LDFLAGS +=  -X github.com/aws/amazon-cloudwatch-agent/cfg/agentinfo.VersionStr=${VERSION}
LDFLAGS +=  -X github.com/aws/amazon-cloudwatch-agent/cfg/agentinfo.BuildStr=${BUILD}

release: clean test build package-rpm package-deb package-win

build: amazon-cloudwatch-agent config-translator start-amazon-cloudwatch-agent amazon-cloudwatch-agent-config-wizard config-downloader

create-version-file:
	@echo Version: ${VERSION}
	@echo Building time: ${BUILD}
	echo "$(VERSION)" > CWAGENT_VERSION

copy-version-file: create-version-file
	mkdir -p build/bin/
	cp CWAGENT_VERSION $(BUILD_SPACE)/bin/CWAGENT_VERSION

amazon-cloudwatch-agent: copy-version-file
	@echo Building amazon-cloudwatch-agent
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/linux_amd64/amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
	GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/linux_arm64/amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
	GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/windows_amd64/amazon-cloudwatch-agent.exe github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent

config-translator: copy-version-file
	@echo Building config-translator
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/linux_amd64/config-translator github.com/aws/amazon-cloudwatch-agent/cmd/config-translator
	GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/linux_arm64/config-translator github.com/aws/amazon-cloudwatch-agent/cmd/config-translator
	GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/windows_amd64/config-translator.exe github.com/aws/amazon-cloudwatch-agent/cmd/config-translator

start-amazon-cloudwatch-agent: copy-version-file
	@echo Building start-amazon-cloudwatch-agent
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/linux_amd64/start-amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
	GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/linux_arm64/start-amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
	GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/windows_amd64/start-amazon-cloudwatch-agent.exe github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent

amazon-cloudwatch-agent-config-wizard: copy-version-file
	@echo Building amazon-cloudwatch-agent-config-wizard
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/linux_amd64/amazon-cloudwatch-agent-config-wizard github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard
	GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/linux_arm64/amazon-cloudwatch-agent-config-wizard github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard
	GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/windows_amd64/amazon-cloudwatch-agent-config-wizard.exe github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard

config-downloader: copy-version-file
	@echo Building config-downloader
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/linux_amd64/config-downloader github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader
	GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/linux_arm64/config-downloader github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader
	GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o $(BUILD_SPACE)/bin/windows_amd64/config-downloader.exe github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader

test:
	go test -v -failfast ./awscsm/... ./cfg/... ./cmd/... ./handlers/... ./internal/... ./logger/... ./logs/... ./metric/... ./plugins/... ./profiler/... ./tool/... ./translator/...

clean::
	rm -rf release/ build/
	rm -f CWAGENT_VERSION

package-prepare-rpm:
	# amd64 rpm
	mkdir -p $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg
	cp $(BUILD_SPACE)/bin/linux_amd64/* $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/LICENSE $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/NOTICE $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/THIRD-PARTY-LICENSES $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
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
	cp $(BASE_SPACE)/licensing/LICENSE $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/NOTICE $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/THIRD-PARTY-LICENSES $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
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
	cp $(BASE_SPACE)/licensing/LICENSE $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/NOTICE $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/THIRD-PARTY-LICENSES $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/
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
	cp $(BASE_SPACE)/licensing/LICENSE $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/NOTICE $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/THIRD-PARTY-LICENSES $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/
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
	cp $(BASE_SPACE)/licensing/LICENSE $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/NOTICE $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/THIRD-PARTY-LICENSES $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/RELEASE_NOTES $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp $(BUILD_SPACE)/bin/CWAGENT_VERSION $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/cfg/commonconfig/common-config.toml $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/packaging/linux/amazon-cloudwatch-agent.conf $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/translator/config/schema.json $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/amazon-cloudwatch-agent-schema.json
	cp ${BASE_SPACE}/packaging/windows/amazon-cloudwatch-agent-ctl.ps1 $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp ${BASE_SPACE}/packaging/windows/install.ps1 $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
	cp ${BASE_SPACE}/packaging/windows/uninstall.ps1 $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
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

.PHONY: build test clean
