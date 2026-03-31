package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
)

type hookRuntime struct {
	initOnce sync.Once
	mu       sync.Mutex
	initErr  error
	mounted  []string
}

func (r *hookRuntime) setInitErr(err error) {
	r.mu.Lock()
	r.initErr = err
	r.mu.Unlock()
}

func (r *hookRuntime) getInitErr() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.initErr
}

func (r *hookRuntime) setMounted(names []string) {
	r.mu.Lock()
	r.mounted = append([]string(nil), names...)
	r.mu.Unlock()
}

func (r *hookRuntime) reset(al *AgentLoop) {
	r.mu.Lock()
	names := append([]string(nil), r.mounted...)
	r.mounted = nil
	r.initErr = nil
	r.initOnce = sync.Once{}
	r.mu.Unlock()

	for _, name := range names {
		al.UnmountHook(name)
	}
}

// BuiltinHookFactory constructs an in-process hook from config.
type BuiltinHookFactory func(ctx context.Context, spec config.BuiltinHookConfig) (any, error)

var (
	builtinHookRegistryMu sync.RWMutex
	builtinHookRegistry   = map[string]BuiltinHookFactory{}
)

// RegisterBuiltinHook registers a named in-process hook factory for config-driven mounting.
func RegisterBuiltinHook(name string, factory BuiltinHookFactory) error {
	if name == "" {
		return fmt.Errorf("builtin hook name is required")
	}
	if factory == nil {
		return fmt.Errorf("builtin hook %q factory is nil", name)
	}

	builtinHookRegistryMu.Lock()
	defer builtinHookRegistryMu.Unlock()

	if _, exists := builtinHookRegistry[name]; exists {
		return fmt.Errorf("builtin hook %q is already registered", name)
	}
	builtinHookRegistry[name] = factory
	return nil
}

func unregisterBuiltinHook(name string) {
	if name == "" {
		return
	}
	builtinHookRegistryMu.Lock()
	delete(builtinHookRegistry, name)
	builtinHookRegistryMu.Unlock()
}

func lookupBuiltinHook(name string) (BuiltinHookFactory, bool) {
	builtinHookRegistryMu.RLock()
	defer builtinHookRegistryMu.RUnlock()

	factory, ok := builtinHookRegistry[name]
	return factory, ok
}

func configureHookManagerFromConfig(hm *HookManager, cfg *config.Config) {
	if hm == nil || cfg == nil {
		return
	}
	hm.ConfigureTimeouts(
		hookTimeoutFromMS(cfg.Hooks.Defaults.ObserverTimeoutMS),
		hookTimeoutFromMS(cfg.Hooks.Defaults.InterceptorTimeoutMS),
		hookTimeoutFromMS(cfg.Hooks.Defaults.ApprovalTimeoutMS),
	)

	// Wire the built-in YOLO/Approval tool approver based on agent defaults.
	mode := cfg.Agents.Defaults.ApprovalMode
	if mode == "" {
		mode = "yolo"
	}
	timeoutSeconds := cfg.Agents.Defaults.ApprovalTimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}
	approvalHook := NewApprovalHook(mode, timeoutSeconds)
	_ = hm.Mount(NamedHook("approval", approvalHook))
}

func hookTimeoutFromMS(ms int) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

func (al *AgentLoop) ensureHooksInitialized(ctx context.Context) error {
	if al == nil || al.cfg == nil || al.hooks == nil {
		return nil
	}

	al.hookRuntime.initOnce.Do(func() {
		al.hookRuntime.setInitErr(al.loadConfiguredHooks(ctx))
	})

	return al.hookRuntime.getInitErr()
}

func (al *AgentLoop) loadConfiguredHooks(ctx context.Context) (err error) {
	if al == nil || al.cfg == nil || !al.cfg.Hooks.Enabled {
		return nil
	}

	mounted := make([]string, 0)
	defer func() {
		if err != nil {
			for _, name := range mounted {
				al.UnmountHook(name)
			}
			return
		}
		al.hookRuntime.setMounted(mounted)
	}()

	builtinNames := enabledBuiltinHookNames(al.cfg.Hooks.Builtins)
	for _, name := range builtinNames {
		spec := al.cfg.Hooks.Builtins[name]
		factory, ok := lookupBuiltinHook(name)
		if !ok {
			return fmt.Errorf("builtin hook %q is not registered", name)
		}

		hook, factoryErr := factory(ctx, spec)
		if factoryErr != nil {
			return fmt.Errorf("build builtin hook %q: %w", name, factoryErr)
		}
		if err := al.MountHook(HookRegistration{
			Name:     name,
			Priority: spec.Priority,
			Source:   HookSourceInProcess,
			Hook:     hook,
		}); err != nil {
			return fmt.Errorf("mount builtin hook %q: %w", name, err)
		}
		mounted = append(mounted, name)
	}

	processNames := enabledProcessHookNames(al.cfg.Hooks.Processes)
	for _, name := range processNames {
		spec := al.cfg.Hooks.Processes[name]
		opts, buildErr := processHookOptionsFromConfig(spec)
		if buildErr != nil {
			return fmt.Errorf("configure process hook %q: %w", name, buildErr)
		}

		processHook, buildErr := NewProcessHook(ctx, name, opts)
		if buildErr != nil {
			return fmt.Errorf("start process hook %q: %w", name, buildErr)
		}
		if err := al.MountHook(HookRegistration{
			Name:     name,
			Priority: spec.Priority,
			Source:   HookSourceProcess,
			Hook:     processHook,
		}); err != nil {
			_ = processHook.Close()
			return fmt.Errorf("mount process hook %q: %w", name, err)
		}
		mounted = append(mounted, name)
	}

	commandNames := enabledCommandHookNames(al.cfg.Hooks.Commands)
	for _, name := range commandNames {
		spec := al.cfg.Hooks.Commands[name]
		cmdHook, buildErr := commandHookFromConfig(name, spec, al.cfg.WorkspacePath())
		if buildErr != nil {
			return fmt.Errorf("configure command hook %q: %w", name, buildErr)
		}
		if err := al.MountHook(HookRegistration{
			Name:     name,
			Priority: spec.Priority,
			Source:   HookSourceInProcess,
			Hook:     cmdHook,
		}); err != nil {
			return fmt.Errorf("mount command hook %q: %w", name, err)
		}
		mounted = append(mounted, name)
	}

	return nil
}

func enabledBuiltinHookNames(specs map[string]config.BuiltinHookConfig) []string {
	if len(specs) == 0 {
		return nil
	}

	names := make([]string, 0, len(specs))
	for name, spec := range specs {
		if spec.Enabled {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func enabledProcessHookNames(specs map[string]config.ProcessHookConfig) []string {
	if len(specs) == 0 {
		return nil
	}

	names := make([]string, 0, len(specs))
	for name, spec := range specs {
		if spec.Enabled {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func processHookOptionsFromConfig(spec config.ProcessHookConfig) (ProcessHookOptions, error) {
	transport := spec.Transport
	if transport == "" {
		transport = "stdio"
	}
	if transport != "stdio" {
		return ProcessHookOptions{}, fmt.Errorf("unsupported transport %q", transport)
	}
	if len(spec.Command) == 0 {
		return ProcessHookOptions{}, fmt.Errorf("command is required")
	}

	opts := ProcessHookOptions{
		Command: append([]string(nil), spec.Command...),
		Dir:     spec.Dir,
		Env:     processHookEnvFromMap(spec.Env),
	}

	observeKinds, observeEnabled, err := processHookObserveKindsFromConfig(spec.Observe)
	if err != nil {
		return ProcessHookOptions{}, err
	}
	opts.Observe = observeEnabled
	opts.ObserveKinds = observeKinds

	for _, intercept := range spec.Intercept {
		switch intercept {
		case "before_llm", "after_llm":
			opts.InterceptLLM = true
		case "before_tool", "after_tool":
			opts.InterceptTool = true
		case "approve_tool":
			opts.ApproveTool = true
		case "":
			continue
		default:
			return ProcessHookOptions{}, fmt.Errorf("unsupported intercept %q", intercept)
		}
	}

	if !opts.Observe && !opts.InterceptLLM && !opts.InterceptTool && !opts.ApproveTool {
		return ProcessHookOptions{}, fmt.Errorf("no hook modes enabled")
	}

	return opts, nil
}

func processHookEnvFromMap(envMap map[string]string) []string {
	if len(envMap) == 0 {
		return nil
	}

	keys := make([]string, 0, len(envMap))
	for key := range envMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+envMap[key])
	}
	return env
}

func processHookObserveKindsFromConfig(observe []string) ([]string, bool, error) {
	if len(observe) == 0 {
		return nil, false, nil
	}

	validKinds := validHookEventKinds()
	normalized := make([]string, 0, len(observe))
	for _, kind := range observe {
		switch kind {
		case "", "*", "all":
			return nil, true, nil
		default:
			if _, ok := validKinds[kind]; !ok {
				return nil, false, fmt.Errorf("unsupported observe event %q", kind)
			}
			normalized = append(normalized, kind)
		}
	}

	if len(normalized) == 0 {
		return nil, false, nil
	}
	return normalized, true, nil
}

func validHookEventKinds() map[string]struct{} {
	kinds := make(map[string]struct{}, int(eventKindCount))
	for kind := EventKind(0); kind < eventKindCount; kind++ {
		kinds[kind.String()] = struct{}{}
	}
	return kinds
}

func enabledCommandHookNames(specs map[string]config.CommandHookConfig) []string {
	if len(specs) == 0 {
		return nil
	}
	names := make([]string, 0, len(specs))
	for name, spec := range specs {
		if spec.Enabled {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func commandHookFromConfig(name string, spec config.CommandHookConfig, workspace string) (*CommandHook, error) {
	if spec.Script == "" {
		return nil, fmt.Errorf("script path is required")
	}
	if spec.Mode != "observe" && spec.Mode != "intercept" {
		return nil, fmt.Errorf("mode must be 'observe' or 'intercept', got %q", spec.Mode)
	}

	scriptPath := spec.Script
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Join(workspace, scriptPath)
	}

	events := make(map[string]struct{}, len(spec.Events))
	if len(spec.Events) > 0 {
		validKinds := validHookEventKinds()
		for _, e := range spec.Events {
			if _, ok := validKinds[e]; !ok {
				return nil, fmt.Errorf("unsupported event kind %q", e)
			}
			events[e] = struct{}{}
		}
	}

	tools := make(map[string]struct{}, len(spec.Tools))
	for _, t := range spec.Tools {
		tools[t] = struct{}{}
	}

	timeout := time.Duration(spec.TimeoutMS) * time.Millisecond
	shell := spec.Shell

	return NewCommandHook(CommandHookOptions{
		Name:       name,
		ScriptPath: scriptPath,
		Mode:       spec.Mode,
		Events:     events,
		Tools:      tools,
		Timeout:    timeout,
		Shell:      shell,
	}), nil
}
