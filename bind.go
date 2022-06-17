package runn

import (
	"context"
	"fmt"
)

const bindRunnerKey = "bind"

type bindRunner struct {
	operator *operator
}

func newBindRunner(o *operator) (*bindRunner, error) {
	return &bindRunner{
		operator: o,
	}, nil
}

func (rnr *bindRunner) Run(ctx context.Context, cond map[string]string) error {
	for k, v := range cond {
		if k == storeVarsKey || k == storeStepsKey {
			return fmt.Errorf("'%s' is reserved", k)
		}
		vv, err := rnr.operator.expand(v)
		if err != nil {
			return err
		}
		rnr.operator.store.bindVars[k] = vv
	}
	return nil
}
