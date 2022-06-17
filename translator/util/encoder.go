package util

type Encoder interface {
	// Encode encodes the input into the output.
	Encode(in interface{}, out interface{}) error
}