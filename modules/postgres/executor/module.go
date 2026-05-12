package executor

import "compass.com/app"

type ExecutorModule struct{}

func (qm *ExecutorModule) Configure(module app.Configurator) error {
	return nil
}

func (qm *ExecutorModule) ProvideExecutor() Executor {
	return NewExecutor()
}
