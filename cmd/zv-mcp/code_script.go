package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/metrics"
	"time"

	"github.com/dop251/goja"
)

const (
	scriptTimeout      = 60 * time.Second
	maxScriptCodeBytes = 20_000
	maxScriptCalls     = 40
	maxScriptResult    = 64_000
	maxScriptHeapGrow  = 256 << 20
	maxScriptStack     = 1024
	maxScriptString    = 1 << 24
	maxScriptItems     = 1 << 22
	maxProcessMemory   = 2 << 30
	scriptHeapPoll     = 20 * time.Millisecond
	scriptHeapMetric   = "/memory/classes/heap/objects:bytes"
)

var errScriptTimeout = fmt.Errorf("the script stopped after %s", scriptTimeout)

var errScriptNotFunction = errors.New("code must be a JavaScript async arrow function, such as async () => { const m = await studio.get_match({ id }); return m.map }")

type scriptTool func(ctx context.Context, args map[string]any) (string, error)

type scriptRun struct {
	Code    string
	Tools   map[string]scriptTool
	Refused map[string]string
}

type scriptResult struct {
	Value any
	Calls int
}

func runScript(ctx context.Context, run scriptRun) (scriptResult, error) {
	if len(run.Code) > maxScriptCodeBytes {
		return scriptResult{}, fmt.Errorf("code is %d bytes, over the %d byte limit", len(run.Code), maxScriptCodeBytes)
	}
	ctx, cancel := context.WithTimeoutCause(ctx, scriptTimeout, errScriptTimeout)
	defer cancel()
	vm := goja.New()
	vm.SetMaxCallStackSize(maxScriptStack)
	if err := hardenScriptRuntime(vm); err != nil {
		return scriptResult{}, err
	}
	stop := guardScript(ctx, vm)
	defer stop()

	calls := 0
	parse, err := jsonFunc(vm, "parse")
	if err != nil {
		return scriptResult{}, err
	}
	studio := vm.NewObject()
	for name, tool := range run.Tools {
		bind := func(fc goja.FunctionCall) goja.Value {
			calls++
			if calls > maxScriptCalls {
				panic(vm.NewGoError(fmt.Errorf("a script makes at most %d tool calls", maxScriptCalls)))
			}
			args, err := scriptArgs(fc.Argument(0))
			if err != nil {
				panic(vm.NewTypeError("studio.%s: %v", name, err))
			}
			text, err := tool(ctx, args)
			if err != nil {
				panic(vm.NewGoError(fmt.Errorf("studio.%s: %w", name, err)))
			}
			value, err := parse(goja.Undefined(), vm.ToValue(text))
			if err != nil {
				return vm.ToValue(text)
			}
			return value
		}
		if err := studio.Set(name, bind); err != nil {
			return scriptResult{}, err
		}
	}
	for name, reason := range run.Refused {
		refuse := func(goja.FunctionCall) goja.Value {
			panic(vm.NewTypeError("studio.%s is not available in run_script: %s", name, reason))
		}
		if err := studio.Set(name, refuse); err != nil {
			return scriptResult{}, err
		}
	}
	if err := vm.Set("studio", studio); err != nil {
		return scriptResult{}, err
	}

	value, err := evalScript(vm, run.Code)
	if err != nil {
		return scriptResult{Calls: calls}, scriptError(ctx, err)
	}
	raw, err := scriptJSON(vm, value)
	if err != nil {
		return scriptResult{Calls: calls}, scriptError(ctx, err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return scriptResult{Calls: calls}, fmt.Errorf("the script's result is not JSON: %w", err)
	}
	return scriptResult{Value: out, Calls: calls}, nil
}

func evalScript(vm *goja.Runtime, code string) (goja.Value, error) {
	fnValue, err := vm.RunString("(" + code + "\n)")
	if err != nil {
		return nil, scriptFailure(vm, err)
	}
	fn, ok := goja.AssertFunction(fnValue)
	if !ok {
		return nil, errScriptNotFunction
	}
	out, err := fn(goja.Undefined())
	if err != nil {
		return nil, scriptFailure(vm, err)
	}
	promise, ok := out.Export().(*goja.Promise)
	if !ok {
		return out, nil
	}
	switch promise.State() {
	case goja.PromiseStateFulfilled:
		return promise.Result(), nil
	case goja.PromiseStateRejected:
		return nil, fmt.Errorf("script failed: %s", scriptValueText(vm, promise.Result()))
	default:
		return nil, errors.New("the script awaited something that never settled")
	}
}

func scriptJSON(vm *goja.Runtime, value goja.Value) (json.RawMessage, error) {
	if value == nil || goja.IsUndefined(value) {
		return json.RawMessage("null"), nil
	}
	stringify, err := jsonFunc(vm, "stringify")
	if err != nil {
		return nil, err
	}
	text, err := stringify(goja.Undefined(), value)
	if err != nil {
		return nil, fmt.Errorf("the script's result is not JSON: %w", scriptFailure(vm, err))
	}
	if goja.IsUndefined(text) {
		return json.RawMessage("null"), nil
	}
	raw := text.String()
	if len(raw) > maxScriptResult {
		return nil, fmt.Errorf("the result is %d bytes, over the %d byte limit; return only the fields the answer needs", len(raw), maxScriptResult)
	}
	return json.RawMessage(raw), nil
}

func jsonFunc(vm *goja.Runtime, name string) (goja.Callable, error) {
	fn, ok := goja.AssertFunction(vm.Get("JSON").ToObject(vm).Get(name))
	if !ok {
		return nil, fmt.Errorf("JSON.%s is missing from the script runtime", name)
	}
	return fn, nil
}

func scriptArgs(v goja.Value) (map[string]any, error) {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return map[string]any{}, nil
	}
	args, ok := v.Export().(map[string]any)
	if !ok {
		return nil, errors.New("pass one object of arguments, such as { match_id }")
	}
	return args, nil
}

func scriptError(ctx context.Context, err error) error {
	var interrupted *goja.InterruptedError
	if errors.As(err, &interrupted) {
		if reason, ok := interrupted.Value().(string); ok {
			return errors.New(reason)
		}
	}
	if ctx.Err() != nil {
		return errors.New(stopReason(ctx))
	}
	return err
}

func guardScript(ctx context.Context, vm *goja.Runtime) func() {
	done := make(chan struct{})
	sample := []metrics.Sample{{Name: scriptHeapMetric}}
	metrics.Read(sample)
	base := sample[0].Value.Uint64()
	go func() {
		tick := time.NewTicker(scriptHeapPoll)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				vm.Interrupt(stopReason(ctx))
				return
			case <-tick.C:
				metrics.Read(sample)
				if sample[0].Value.Uint64() > base+maxScriptHeapGrow {
					vm.Interrupt(fmt.Sprintf("the script used over %d MiB of memory", maxScriptHeapGrow>>20))
					return
				}
			}
		}
	}()
	return func() { close(done) }
}

func stopReason(ctx context.Context) string {
	if errors.Is(context.Cause(ctx), errScriptTimeout) {
		return errScriptTimeout.Error()
	}
	return "the call was cancelled or ran out of time, so the script stopped"
}

func hardenScriptRuntime(vm *goja.Runtime) error {
	for _, name := range []string{"ArrayBuffer", "SharedArrayBuffer", "DataView", "Int8Array", "Uint8Array", "Uint8ClampedArray",
		"Int16Array", "Uint16Array", "Int32Array", "Uint32Array", "Float32Array", "Float64Array", "BigInt64Array", "BigUint64Array"} {
		if err := vm.GlobalObject().Delete(name); err != nil {
			return err
		}
	}
	_, err := vm.RunString(fmt.Sprintf(`(() => {
		const cap = %d;
		const guard = (name, length) => {
			const original = String.prototype[name];
			Object.defineProperty(String.prototype, name, { writable: false, configurable: false, value: function (...args) {
				if (length(String(this), args) > cap) throw new RangeError(name + " would build a string over " + cap + " characters");
				return original.apply(this, args);
			} });
		};
		guard("repeat", (s, args) => s.length * Number(args[0]));
		guard("padStart", (s, args) => Number(args[0]));
		guard("padEnd", (s, args) => Number(args[0]));

		const items = %d;
		const tooLong = (n) => {
			if (Number(n) > items) throw new RangeError("a script's arrays hold at most " + items + " items");
		};
		const NativeArray = Array;
		for (const name of Object.getOwnPropertyNames(NativeArray.prototype)) {
			const original = NativeArray.prototype[name];
			if (name === "constructor" || typeof original !== "function") continue;
			Object.defineProperty(NativeArray.prototype, name, { writable: false, configurable: false, value: function (...args) {
				if (this != null) tooLong(this.length);
				return original.apply(this, args);
			} });
		}
		const from = NativeArray.from;
		Object.defineProperty(NativeArray, "from", { writable: false, configurable: false, value: function (source, ...rest) {
			if (source != null) tooLong(source.length);
			return from.call(this, source, ...rest);
		} });
	})()`, maxScriptString, maxScriptItems))
	return err
}

func scriptFailure(vm *goja.Runtime, err error) error {
	var exception *goja.Exception
	if !errors.As(err, &exception) || exception.Value() == nil {
		return err
	}
	return fmt.Errorf("script failed: %s", scriptValueText(vm, exception.Value()))
}

func scriptValueText(vm *goja.Runtime, v goja.Value) string {
	text := ""
	if v == nil || vm.Try(func() { text = v.String() }) != nil {
		return "the script threw a value that cannot be shown"
	}
	return text
}
