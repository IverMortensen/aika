package agents

type WorkerBehavior struct {
}

func NewWorkerBehavior() (WorkerBehavior, error) {
	return WorkerBehavior{}, nil
}

func (wb *WorkerBehavior) Run() error {
	return nil
}
