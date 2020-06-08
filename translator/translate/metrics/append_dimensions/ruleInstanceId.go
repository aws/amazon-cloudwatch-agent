package append_dimensions

type InstanceId struct {
}

const Reserved_Key_Instance_Id = "InstanceId"
const Reserved_Val_Instance_Id = "${aws:InstanceId}"

func (i *InstanceId) ApplyRule(input interface{}) (string, interface{}) {
	return CheckIfExactMatch(input, Reserved_Key_Instance_Id, Reserved_Val_Instance_Id, "ec2_metadata_tags", Reserved_Key_Instance_Id)
}

func init() {
	i := new(InstanceId)
	RegisterRule(Reserved_Key_Instance_Id, i)
}
