package agents

type FinalBehavior struct {
}

func NewFinalBehavior() (FinalBehavior, error) {
	return FinalBehavior{}, nil
}

func (fb *FinalBehavior) Run() error {
	return nil
}
