package ecsservicediscovery

// Filter out the tasks not matching the discovery configs
// Filter out the tasks with nil task definition
type TaskFilterProcessor struct {
}

func NewTaskFilterProcessor() *TaskFilterProcessor {
	return &TaskFilterProcessor{}
}

func (p *TaskFilterProcessor) Process(cluster string, taskList []*DecoratedTask) ([]*DecoratedTask, error) {
	var filteredClusterTasks []*DecoratedTask
	for _, v := range taskList {
		if v.DockerLabelBased || v.TaskDefinitionBased {
			filteredClusterTasks = append(filteredClusterTasks, v)
		}
	}
	return filteredClusterTasks, nil
}

func (p *TaskFilterProcessor) ProcessorName() string {
	return "TaskFilterProcessor"
}
