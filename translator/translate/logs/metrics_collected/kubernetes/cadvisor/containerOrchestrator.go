package cadvisor

const (
	SectionKeyContainerOrchestrator = "container_orchestrator"
	EKS                             = "eks"
)

type ContainerOrchestrator struct {
}

func (c *ContainerOrchestrator) ApplyRule(input interface{}) (string, interface{}) {
	return SectionKeyContainerOrchestrator, EKS
}

func init() {
	RegisterRule(SectionKeyContainerOrchestrator, new(ContainerOrchestrator))
}
