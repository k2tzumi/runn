package runn

import (
	"context"
	"errors"
	"path/filepath"
)

const includeRunnerKey = "include"

type includeRunner struct {
	name      string
	path      string
	params    map[string]any
	runResult *RunResult
}

type includeConfig struct {
	path     string
	vars     map[string]any
	skipTest bool
	force    bool
	step     *step
}

type includedRunErr struct {
	err error
}

func newIncludedRunErr(err error) *includedRunErr {
	return &includedRunErr{err: err}
}

func (e *includedRunErr) Error() string {
	return e.err.Error()
}

func (e *includedRunErr) Unwrap() error {
	return e.err
}

func (e *includedRunErr) Is(target error) bool {
	err := target
	for {
		_, ok := err.(*includedRunErr) //nolint:errorlint
		if ok {
			return true
		}
		if err = errors.Unwrap(err); err == nil {
			break
		}
	}
	return false
}

func newIncludeRunner() (*includeRunner, error) {
	return &includeRunner{}, nil
}

func (rnr *includeRunner) Run(ctx context.Context, s *step) error {
	o := s.parent
	c := s.includeConfig
	if o.thisT != nil {
		o.thisT.Helper()
	}
	rnr.runResult = nil

	ipath := rnr.path
	if ipath == "" {
		ipath = c.path
	}
	// ipath must not be variable expanded. Because it will be impossible to identify the step of the included runbook in case of run failure.
	if !hasRemotePrefix(ipath) {
		ipath = filepath.Join(o.root, ipath)
	}

	// Store before record
	store := o.store.toMap()
	store[storeRootKeyIncluded] = o.included
	store[storeRootKeyPrevious] = o.store.latest()

	nodes, err := s.expandNodes()
	if err != nil {
		return err
	}
	if rnr.name != "" {
		v, ok := nodes[rnr.name].(map[string]any)
		if ok {
			store[storeRootKeyNodes] = v
		}
	}

	params := map[string]any{}
	for k, v := range rnr.params {
		switch ov := v.(type) {
		case string:
			var vv any
			vv, err = o.expandBeforeRecord(ov)
			if err != nil {
				return err
			}
			evv, err := evaluateSchema(vv, o.root, store)
			if err != nil {
				return err
			}
			params[k] = evv
		case map[string]any, []any:
			vv, err := o.expandBeforeRecord(ov)
			if err != nil {
				return err
			}
			params[k] = vv
		default:
			params[k] = ov
		}
	}
	if len(params) > 0 {
		store[storeRootKeyParams] = params
	}

	pstore := map[string]any{
		storeRootKeyParent: store,
	}

	oo, err := o.newNestedOperator(c.step, bookWithStore(ipath, pstore), SkipTest(c.skipTest))
	if err != nil {
		return err
	}

	// Override vars
	for k, v := range c.vars {
		switch ov := v.(type) {
		case string:
			var vv any
			vv, err = o.expandBeforeRecord(ov)
			if err != nil {
				return err
			}
			evv, err := evaluateSchema(vv, oo.root, store)
			if err != nil {
				return err
			}
			oo.store.vars[k] = evv
		case map[string]any, []any:
			vv, err := o.expandBeforeRecord(ov)
			if err != nil {
				return err
			}
			oo.store.vars[k] = vv
		default:
			oo.store.vars[k] = ov
		}
	}

	if err := rnr.run(ctx, oo, s); err != nil {
		return err
	}
	return nil
}

func (rnr *includeRunner) run(ctx context.Context, oo *operator, s *step) error {
	o := s.parent
	if err := oo.run(ctx); err != nil {
		rnr.runResult = oo.runResult
		return newIncludedRunErr(err)
	}
	rnr.runResult = oo.runResult
	o.record(oo.store.toNormalizedMap())
	return nil
}

// newNestedOperator create nested operator.
func (o *operator) newNestedOperator(parent *step, opts ...Option) (*operator, error) {
	var popts []Option
	popts = append(popts, included(true))

	// Set parent runners for re-use
	for k, r := range o.httpRunners {
		popts = append(popts, reuseHTTPRunner(k, r))
	}
	for k, r := range o.dbRunners {
		popts = append(popts, reuseDBRunner(k, r))
	}
	for k, r := range o.grpcRunners {
		popts = append(popts, reuseGrpcRunner(k, r))
	}
	for k, r := range o.sshRunners {
		popts = append(popts, reuseSSHRunner(k, r))
	}

	popts = append(popts, Debug(o.debug))
	popts = append(popts, Profile(o.profile))
	popts = append(popts, SkipTest(o.skipTest))
	popts = append(popts, Force(o.force))
	popts = append(popts, Trace(o.trace))
	for k, f := range o.store.funcs {
		popts = append(popts, Func(k, f))
	}

	// Prefer child runbook opts
	// For example, if a runner with the same name is defined in the child runbook to be included, it takes precedence.
	opts = append(popts, opts...)
	oo, err := New(opts...)
	if err != nil {
		return nil, err
	}
	// Nested operators do not inherit beforeFuncs/afterFuncs
	oo.t = o.thisT
	oo.thisT = o.thisT
	oo.sw = o.sw
	oo.capturers = o.capturers
	oo.parent = parent
	oo.store.parentVars = o.store.toMap()
	oo.store.kv = o.store.kv
	oo.dbg = o.dbg
	return oo, nil
}
