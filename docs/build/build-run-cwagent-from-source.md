## Building and running Amazon CloudWatch Agent from source
### 1. Prerequisite
* Clone [Amazon CloudWatch Agent repository](https://github.com/aws/amazon-cloudwatch-agent.git)
* Install go. For more information, see [Getting started](https://golang.org/doc/install)
* The agent uses go modules for dependency management. For more information, see [Go Modules](https://github.com/golang/go/wiki/Modules)
* Install rpm-build
```
sudo yum install -y rpmdevtools rpm-build
```

### 2. Building the agent

* Run `make build` to build the CloudWatch Agent for Linux, Debian, Windows environment.

* Run `make release` to build the agent. This also packages it into a RPM, DEB and ZIP package.

The following folders are generated when the build completes:
```
build/bin/linux/arm64/amazon-cloudwatch-agent.rpm
build/bin/linux/amd64/amazon-cloudwatch-agent.rpm
build/bin/linux/arm64/amazon-cloudwatch-agent.deb
build/bin/linux/amd64/amazon-cloudwatch-agent.deb
build/bin/windows/amd64/amazon-cloudwatch-agent.zip
build/bin/darwin/amd64/amazon-cloudwatch-agent.tar.gz
```

### 3. Install your own build of the agent
#### 3.1 RPM package
* `rpm -Uvh amazon-cloudwatch-agent.rpm`
   
#### 3.2 DEB package
* `dpkg -i -E ./amazon-cloudwatch-agent.deb`

#### 3.3 Windows package
* unzip `amazon-cloudwatch-agent.zip`
* `./install.ps1`

#### Darwin package
* `tar -xvf amazon-cloudwatch-agent.tar.gz`
* `cp -rf ./opt/aws /opt`
* `cp -rf ./Library/LaunchDaemons/com.amazon.cloudwatch.agent.plist /Library/LaunchDaemons/`
