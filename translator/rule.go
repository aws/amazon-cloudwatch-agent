package translator

//Concrete Rule implementation should return a (key string,val interface{})
type Rule interface {
	ApplyRule(interface{}) (string, interface{})
}
