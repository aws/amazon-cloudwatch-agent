package ec2tagger

import (
	"github.com/aws/amazon-cloudwatch-agent/translator"
	parent "github.com/aws/amazon-cloudwatch-agent/translator/translate/logs/metrics_collected/kubernetes"
)

type Rule translator.Rule

var ChildRule = map[string]Rule{}

const (
	SubSectionKey = "ec2tagger"
)

func GetCurPath() string {
	curPath := parent.GetCurPath() + SubSectionKey + "/"
	return curPath
}

func RegisterRule(fieldname string, r Rule) {
	ChildRule[fieldname] = r
}

type Ec2Tagger struct {
}

func (e *Ec2Tagger) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
	returnKey = SubSectionKey
	returnVal = map[string]interface{}{"ec2_instance_tag_keys": []string{"aws:autoscaling:groupName"}, "ec2_metadata_tags": []string{"InstanceId", "InstanceType"}, "ebs_device_keys": []string{"*"}, "disk_device_tag_key": "device"}
	return
}

func init() {
	e := new(Ec2Tagger)
	parent.RegisterRule(SubSectionKey, e)
}
