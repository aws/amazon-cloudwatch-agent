package append_dimensions

type ImageId struct {
}

const Reserved_Key_Image_Id = "ImageId"
const Reserved_Val_Image_Id = "${aws:ImageId}"

func (i *ImageId) ApplyRule(input interface{}) (string, interface{}) {
	return CheckIfExactMatch(input, Reserved_Key_Image_Id, Reserved_Val_Image_Id, "ec2_metadata_tags", Reserved_Key_Image_Id)
}

func init() {
	i := new(ImageId)
	RegisterRule(Reserved_Key_Image_Id, i)
}
