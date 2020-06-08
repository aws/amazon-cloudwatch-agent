package cadvisor

const (
	SectionKeyContainerOrchestrator = "container_orchestrator"
	ECS                             = "ecs"
)

type ContainerOrchestrator struct {
}

func (c *ContainerOrchestrator) ApplyRule(input interface{}) (string, interface{}) {
	return SectionKeyContainerOrchestrator, ECS
}

func init() {
	RegisterRule(SectionKeyContainerOrchestrator, new(ContainerOrchestrator))
}
