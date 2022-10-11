package metric

type Statistics int

const (
	AVERAGE Statistics = iota
)

func (s Statistics) String() string {
	return [...]string{"Average"}[s]
}
