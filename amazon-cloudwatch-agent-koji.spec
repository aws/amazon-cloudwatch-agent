# This spec is used for koji build only. 
Summary:    Amazon CloudWatch Agent
Name:       amazon-cloudwatch-agent
Version:    1.247345.0
Release:    1
License:    Amazon Software License. Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
Group:      Applications/CloudWatch-Agent
ExcludeArch: %{ix86}
BuildRoot: %{_tmppath}/%{name}-%{version}-%{release}-buildroot-%(%{__id_u} -n)
Source:     amazon-cloudwatch-agent.tar.gz
BuildRequires: golang >= 1.7.4

%define _enable_debug_packages 0
%define debug_package %{nil}


%prep
%setup -c %{name}-%{version}

%description
This package provides daemon of Amazon CloudWatch Agent

############################# set up folder structure and build the binaries
%build
echo "build: " "$(pwd)"
mkdir -p opt/aws/amazon-cloudwatch-agent/logs
mkdir -p opt/aws/amazon-cloudwatch-agent/bin
mkdir -p opt/aws/amazon-cloudwatch-agent/etc
mkdir -p opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.d
mkdir -p opt/aws/amazon-cloudwatch-agent/manager
mkdir -p opt/aws/amazon-cloudwatch-agent/var
mkdir -p opt/aws/amazon-cloudwatch-agent/doc
mkdir -p etc/init
mkdir -p etc/systemd/system/

cd amazon-cloudwatch-agent

AGENT_VERSION=%{version}
BUILD=$(date --iso-8601=seconds)
LDFLAGS="-s -w -X github.com/aws/amazon-cloudwatch-agent/cfg/agentinfo.VersionStr=${AGENT_VERSION} -X github.com/aws/amazon-cloudwatch-agent/cfg/agentinfo.BuildStr=${BUILD}"

%ifarch x86_64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="${LDFLAGS}" -o ../opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="${LDFLAGS}" -o ../opt/aws/amazon-cloudwatch-agent/bin/config-translator github.com/aws/amazon-cloudwatch-agent/cmd/config-translator
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="${LDFLAGS}" -o ../opt/aws/amazon-cloudwatch-agent/bin/start-amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="${LDFLAGS}" -o ../opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-config-wizard github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags="${LDFLAGS}" -o ../opt/aws/amazon-cloudwatch-agent/bin/config-downloader github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader
%endif
%ifarch aarch64
GOOS=linux GOARCH=arm64 go build -mod=vendor -ldflags="${LDFLAGS}" -o ../opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent
GOOS=linux GOARCH=arm64 go build -mod=vendor -ldflags="${LDFLAGS}" -o ../opt/aws/amazon-cloudwatch-agent/bin/config-translator github.com/aws/amazon-cloudwatch-agent/cmd/config-translator
GOOS=linux GOARCH=arm64 go build -mod=vendor -ldflags="${LDFLAGS}" -o ../opt/aws/amazon-cloudwatch-agent/bin/start-amazon-cloudwatch-agent github.com/aws/amazon-cloudwatch-agent/cmd/start-amazon-cloudwatch-agent
GOOS=linux GOARCH=arm64 go build -mod=vendor -ldflags="${LDFLAGS}" -o ../opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-config-wizard github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent-config-wizard
GOOS=linux GOARCH=arm64 go build -mod=vendor -ldflags="${LDFLAGS}" -o ../opt/aws/amazon-cloudwatch-agent/bin/config-downloader github.com/aws/amazon-cloudwatch-agent/cmd/config-downloader
%endif

cp licensing/LICENSE ../opt/aws/amazon-cloudwatch-agent/
cp licensing/NOTICE ../opt/aws/amazon-cloudwatch-agent/
cp licensing/THIRD-PARTY-LICENSES ../opt/aws/amazon-cloudwatch-agent/
cp RELEASE_NOTES ../opt/aws/amazon-cloudwatch-agent/
echo "$(AGENT_VERSION)" > ../opt/aws/amazon-cloudwatch-agent/bin/CWAGENT_VERSION
cp packaging/dependencies/amazon-cloudwatch-agent-ctl ../opt/aws/amazon-cloudwatch-agent/bin/
cp packaging/dependencies/amazon-cloudwatch-agent.service ../etc/systemd/system/
cp cfg/commonconfig/common-config.toml ../opt/aws/amazon-cloudwatch-agent/etc/
cp packaging/linux/amazon-cloudwatch-agent.conf ../etc/init/amazon-cloudwatch-agent.conf
cp translator/config/schema.json ../opt/aws/amazon-cloudwatch-agent/doc/amazon-cloudwatch-agent-schema.json

chmod ug+rx ../opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent
chmod ug+rx ../opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl
chmod ug+rx ../opt/aws/amazon-cloudwatch-agent/bin/start-amazon-cloudwatch-agent


############################# copy from BUILD to BUILDROOT
%install
rm -rf $RPM_BUILD_ROOT
mkdir $RPM_BUILD_ROOT
rm -rf %{_topdir}/BUILD/%{name}-%{version}/amazon-cloudwatch-agent
cp -r %{_topdir}/BUILD/%{name}-%{version}/*  $RPM_BUILD_ROOT/


############################# create the symbolic links
# bin
mkdir -p ${RPM_BUILD_ROOT}/usr/bin
ln -f -s /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl ${RPM_BUILD_ROOT}/usr/bin/amazon-cloudwatch-agent-ctl
# etc
mkdir -p ${RPM_BUILD_ROOT}/etc/amazon
ln -f -s /opt/aws/amazon-cloudwatch-agent/etc ${RPM_BUILD_ROOT}/etc/amazon/amazon-cloudwatch-agent
# log
mkdir -p ${RPM_BUILD_ROOT}/var/log/amazon
ln -f -s /opt/aws/amazon-cloudwatch-agent/logs ${RPM_BUILD_ROOT}/var/log/amazon/amazon-cloudwatch-agent
# pid
mkdir -p ${RPM_BUILD_ROOT}/var/run/amazon
ln -f -s /opt/aws/amazon-cloudwatch-agent/var ${RPM_BUILD_ROOT}/var/run/amazon/amazon-cloudwatch-agent

%files
%dir /opt/aws
%dir /opt/aws/amazon-cloudwatch-agent
%dir /opt/aws/amazon-cloudwatch-agent/bin
%dir /opt/aws/amazon-cloudwatch-agent/doc
%dir /opt/aws/amazon-cloudwatch-agent/etc
%dir /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.d
%dir /opt/aws/amazon-cloudwatch-agent/logs
%dir /opt/aws/amazon-cloudwatch-agent/var
/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent
/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl
/opt/aws/amazon-cloudwatch-agent/bin/CWAGENT_VERSION
/opt/aws/amazon-cloudwatch-agent/bin/config-translator
/opt/aws/amazon-cloudwatch-agent/bin/config-downloader
/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-config-wizard
/opt/aws/amazon-cloudwatch-agent/bin/start-amazon-cloudwatch-agent
/opt/aws/amazon-cloudwatch-agent/doc/amazon-cloudwatch-agent-schema.json
%config(noreplace) /opt/aws/amazon-cloudwatch-agent/etc/common-config.toml
/opt/aws/amazon-cloudwatch-agent/LICENSE
/opt/aws/amazon-cloudwatch-agent/NOTICE

/opt/aws/amazon-cloudwatch-agent/THIRD-PARTY-LICENSES
/opt/aws/amazon-cloudwatch-agent/RELEASE_NOTES
/etc/init/amazon-cloudwatch-agent.conf
/etc/systemd/system/amazon-cloudwatch-agent.service

/usr/bin/amazon-cloudwatch-agent-ctl
/etc/amazon/amazon-cloudwatch-agent
/var/log/amazon/amazon-cloudwatch-agent
/var/run/amazon/amazon-cloudwatch-agent

%pre
# Stop the agent before upgrades.
if [ $1 -ge 2 ]; then
    if [ -x /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl ]; then
        /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a stop
    fi
fi

if ! grep "^cwagent:" /etc/group >/dev/null 2>&1; then
    groupadd -r cwagent >/dev/null 2>&1
    echo "create group cwagent, result: $?"
fi

if ! id cwagent >/dev/null 2>&1; then
    useradd -r -M cwagent -d /home/cwagent -g cwagent -c "Cloudwatch Agent" -s $(test -x /sbin/nologin && echo /sbin/nologin || (test -x /usr/sbin/nologin && echo /usr/sbin/nologin || (test -x /bin/false && echo /bin/false || echo /bin/sh))) >/dev/null 2>&1
    echo "create user cwagent, result: $?"
fi

%preun
# Stop the agent after uninstall
if [ $1 -eq 0 ] ; then
    if [ -x /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl ]; then
        /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a preun
    fi
fi

%clean
