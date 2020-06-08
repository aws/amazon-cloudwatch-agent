package util

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/util/ec2util"
	"log"
	"net"
	"os"

	"strings"
)

const (
	instanceIdPlaceholder    = "{instance_id}"
	hostnamePlaceholder      = "{hostname}"
	localHostnamePlaceholder = "{local_hostname}" //regardless of ec2 metadata
	ipAddressPlaceholder     = "{ip_address}"

	unknownInstanceId = "i-UNKNOWN"
	unknownHostname   = "UNKNOWN-HOST"
	unknownIpAddress  = "UNKNOWN-IP"
)

//resolve place holder for log group and log stream.
func ResolvePlaceholder(placeholder string, metadata map[string]string) string {
	tmpString := placeholder
	if tmpString == "" {
		tmpString = instanceIdPlaceholder
	}
	for k, v := range metadata {
		tmpString = strings.Replace(tmpString, k, v, -1)
	}
	return tmpString
}

func GetMetadataInfo() map[string]string {
	localHostname := getHostName()

	instanceID := ec2util.GetEC2UtilSingleton().InstanceID
	if instanceID == "" {
		instanceID = unknownInstanceId
	}

	hostname := ec2util.GetEC2UtilSingleton().Hostname
	if hostname == "" {
		hostname = localHostname
	}

	ipAddress := ec2util.GetEC2UtilSingleton().PrivateIP
	if ipAddress == "" {
		ipAddress = getIpAddress()
	}
	return map[string]string{instanceIdPlaceholder: instanceID, hostnamePlaceholder: hostname,
		localHostnamePlaceholder: localHostname, ipAddressPlaceholder: ipAddress}
}

func getHostName() string {
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	} else {
		log.Println("E! getHostName: ", err)
		return unknownHostname
	}
}

func getIpAddress() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Println("E! getIpAddress -> getInterfaceAddrs: ", err)
		return unknownIpAddress
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return unknownIpAddress
}
