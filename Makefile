export BASE_SPACE=$(shell pwd)
export BUILD_SPACE=$(BASE_SPACE)/build

release: clean test build package-rpm package-deb package-win

build: amazon-cloudwatch-agent config-translator start-amazon-cloudwatch-agent amazon-cloudwatch-agent-config-wizard config-downloader

amazon-cloudwatch-agent:
	@echo Building amazon-cloudwatch-agent
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_SPACE)/bin/linux_amd64/amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_SPACE)/bin/linux_arm64/amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_SPACE)/bin/windows_amd64/amazon-cloudwatch-agent.exe github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent

config-translator:
	@echo Building config-translator
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_SPACE)/bin/linux_amd64/config-translator github.com/aws/amazon-cloudwatch-agent/cmd/config-translator
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_SPACE)/bin/linux_arm64/config-translator github.com/aws/amazon-cloudwatch-agent/cmd/config-translator
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_SPACE)/bin/windows_amd64/config-translator.exe github.com/aws/amazon-cloudwatch-agent/cmd/config-translator

start-amazon-cloudwatch-agent:
	@echo Building start-amazon-cloudwatch-agent
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_SPACE)/bin/linux_amd64/start-amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_SPACE)/bin/linux_arm64/start-amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_SPACE)/bin/windows_amd64/start-amazon-cloudwatch-agent.exe github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent

amazon-cloudwatch-agent-config-wizard:
	@echo Building amazon-cloudwatch-agent-config-wizard
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_SPACE)/bin/linux_amd64/amazon-cloudwatch-agent-config-wizard github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_SPACE)/bin/linux_arm64/amazon-cloudwatch-agent-config-wizard github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_SPACE)/bin/windows_amd64/amazon-cloudwatch-agent-config-wizard.exe github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard

config-downloader:
	@echo Building config-downloader
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_SPACE)/bin/linux_amd64/config-downloader github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_SPACE)/bin/linux_arm64/config-downloader github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_SPACE)/bin/windows_amd64/config-downloader.exe github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader

test:
	go test -v -failfast ./...

clean::
	rm -rf release/ build/

package-prepare-rpm:
	# amd64 rpm
	mkdir -p $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg
	cp $(BUILD_SPACE)/bin/linux_amd64/* $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/LICENSE $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/NOTICE $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/licensing/THIRD-PARTY-LICENSES $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/RELEASE_NOTES $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
	cp $(BASE_SPACE)/CWAGENT_VERSION $(BUILD_SPACE)/private/linux/amd64/rpm/amazon-cloudwatch-agent-pre-pkg/
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
	cp $(BASE_SPACE)/CWAGENT_VERSION $(BUILD_SPACE)/private/linux/arm64/rpm/amazon-cloudwatch-agent-pre-pkg/
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
	cp $(BASE_SPACE)/CWAGENT_VERSION $(BUILD_SPACE)/private/linux/amd64/deb/amazon-cloudwatch-agent-pre-pkg/
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
	cp $(BASE_SPACE)/CWAGENT_VERSION $(BUILD_SPACE)/private/linux/arm64/deb/amazon-cloudwatch-agent-pre-pkg/
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
	cp $(BASE_SPACE)/CWAGENT_VERSION $(BUILD_SPACE)/private/windows/amd64/zip/amazon-cloudwatch-agent-pre-pkg/
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
