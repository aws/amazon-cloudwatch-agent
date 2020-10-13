package ecsservicediscovery

type Processor interface {
	Process(cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error)
	ProcessorName() string
}
