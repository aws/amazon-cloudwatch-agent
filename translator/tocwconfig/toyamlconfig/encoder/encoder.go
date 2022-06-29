package encoder

type Encoder interface {
	Encode(in interface{}, out interface{}) error
}
