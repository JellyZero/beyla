//go:build linux

package ebpf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
	"strings"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"golang.org/x/sys/unix"

	ebpfcommon "github.com/grafana/beyla/pkg/internal/ebpf/common"
	"github.com/grafana/beyla/pkg/internal/exec"
	"github.com/grafana/beyla/pkg/internal/goexec"
	"github.com/grafana/beyla/pkg/internal/request"
)

// Tracer is an individual eBPF program (e.g. the net/http or the grpc tracers)
type Tracer interface {
	// Load the bpf object that is generated by the bpf2go compiler
	Load() (*ebpf.CollectionSpec, error)
	// Constants returns a map of constants to be overriden into the eBPF program.
	// The key is the constant name and the value is the value to overwrite.
	Constants(*exec.FileInfo, *goexec.Offsets) map[string]any
	// BpfObjects that are created by the bpf2go compiler
	BpfObjects() any
	// GoProbes returns a map with the name of Go functions that need to be inspected
	// in the executable, as well as the eBPF programs that optionally need to be
	// inserted as the Go function start and end probes
	GoProbes() map[string]ebpfcommon.FunctionPrograms
	// KProbes returns a map with the name of the kernel probes that need to be
	// tapped into. Start matches kprobe, End matches kretprobe
	KProbes() map[string]ebpfcommon.FunctionPrograms
	// KProbes returns a map with the module name mapping to the uprobes that need to be
	// tapped into. Start matches uprobe, End matches uretprobe
	UProbes() map[string]map[string]ebpfcommon.FunctionPrograms
	// Socket filters returns a list of programs that need to be loaded as a
	// generic eBPF socket filter
	SocketFilters() []*ebpf.Program
	// Run will do the action of listening for eBPF traces and forward them
	// periodically to the output channel.
	// It optionally receives the service name as a second attribute, to
	// populate each forwarded span with its value. But some
	// tracers might ignore it (e.g. system-wide HTTP filter will directly set the
	// executable name of each request).
	Run(context.Context, chan<- []request.Span, string)
	// AddCloser adds io.Closer instances that need to be invoked when the
	// Run function ends.
	AddCloser(c ...io.Closer)
}

// ProcessTracer instruments an executable with eBPF and provides the eBPF readers
// that will forward the traces to later stages in the pipeline
// TODO: split in two, the instrumenter and the reader
type ProcessTracer struct {
	programs []Tracer
	ELFInfo  *exec.FileInfo
	goffsets *goexec.Offsets
	exe      *link.Executable
	pinPath  string

	systemWide          bool
	overrideServiceName string
}

func (pt *ProcessTracer) Run(ctx context.Context, out chan<- []request.Span) {
	var log = logger()

	// Searches for traceable functions
	trcrs, err := pt.tracers()
	if err != nil {
		log.Error("couldn't trace process", err, "path", pt.ELFInfo.CmdExePath, "pid", pt.ELFInfo.Pid)
		return
	}

	svcName := pt.overrideServiceName
	// If the user does not override the service name via configuration
	// the service name is the name of the found executable
	// Unless the case of system-wide tracing, where the name of the
	// executable will be dynamically set for each traced http request call.
	if svcName == "" && !pt.systemWide {
		svcName = pt.ELFInfo.ExecutableName()
	}
	// run each tracer program
	for _, t := range trcrs {
		go t.Run(ctx, out, svcName)
	}
}

// tracers returns Tracer implementer for each discovered eBPF traceable source: GRPC, HTTP...
func (pt *ProcessTracer) tracers() ([]Tracer, error) {
	var log = logger()

	// tracerFuncs contains the eBPF programs (HTTP, GRPC tracers...)
	var tracers []Tracer

	for _, p := range pt.programs {
		plog := log.With("program", reflect.TypeOf(p))
		plog.Debug("loading eBPF program")
		spec, err := p.Load()
		if err != nil {
			return nil, fmt.Errorf("loading eBPF program: %w", err)
		}
		if err := spec.RewriteConstants(p.Constants(pt.ELFInfo, pt.goffsets)); err != nil {
			return nil, fmt.Errorf("rewriting BPF constants definition: %w", err)
		}
		if err := spec.LoadAndAssign(p.BpfObjects(), &ebpf.CollectionOptions{
			Maps: ebpf.MapOptions{
				PinPath: pt.pinPath,
			}}); err != nil {
			printVerifierErrorInfo(err)
			return nil, fmt.Errorf("loading and assigning BPF objects: %w", err)
		}
		i := instrumenter{
			exe:     pt.exe,
			offsets: pt.goffsets,
		}

		//Go style Uprobes
		if err := i.goprobes(p); err != nil {
			printVerifierErrorInfo(err)
			return nil, err
		}

		//Kprobes to be used for native instrumentation points
		if err := i.kprobes(p); err != nil {
			printVerifierErrorInfo(err)
			return nil, err
		}

		//Uprobes to be used for native module instrumentation points
		if err := i.uprobes(pt.ELFInfo.Pid, p); err != nil {
			printVerifierErrorInfo(err)
			return nil, err
		}

		//Sock filters support
		if err := i.sockfilters(p); err != nil {
			printVerifierErrorInfo(err)
			return nil, err
		}

		tracers = append(tracers, p)
	}

	return tracers, nil
}

func logger() *slog.Logger { return slog.With("component", "ebpf.TracerProvider") }

// filterNotFoundPrograms will filter these programs whose required functions (as
// returned in the Offsets method) haven't been found in the offsets
func filterNotFoundPrograms(programs []Tracer, offsets *goexec.Offsets) []Tracer {
	var filtered []Tracer
	funcs := offsets.Funcs
programs:
	for _, p := range programs {
		for fn, fp := range p.GoProbes() {
			if !fp.Required {
				continue
			}
			if _, ok := funcs[fn]; !ok {
				continue programs
			}
		}
		filtered = append(filtered, p)
	}
	return filtered
}

func allGoFunctionNames(programs []Tracer) []string {
	uniqueFunctions := map[string]struct{}{}
	var functions []string
	for _, p := range programs {
		for funcName := range p.GoProbes() {
			// avoid duplicating function names
			if _, ok := uniqueFunctions[funcName]; !ok {
				uniqueFunctions[funcName] = struct{}{}
				functions = append(functions, funcName)
			}
		}
	}
	return functions
}

func inspect(ctx context.Context, cfg *ebpfcommon.TracerConfig, functions []string) (*exec.FileInfo, *goexec.Offsets, error) {
	// Finding the process by port is more complex, it needs to skip proxies
	if cfg.Port != 0 {
		return inspectByPort(ctx, cfg, functions)
	}

	finder := exec.ProcessNamed(cfg.Exec)
	elfs, err := exec.FindExecELF(ctx, finder)
	for _, exec := range elfs {
		defer exec.ELF.Close()
	}
	if err != nil || len(elfs) == 0 {
		return nil, nil, fmt.Errorf("looking for executable ELF: %w", err)
	}

	// when we look by executable name we pick the first one, we look at all processes only when we pick by port to avoid proxies
	execElf := elfs[0]
	var offsets *goexec.Offsets

	if !cfg.SystemWide {
		if cfg.SkipGoSpecificTracers {
			logger().Info("Skipping inspection for Go functions. Using only generic instrumentation.", "pid", execElf.Pid, "comm", execElf.CmdExePath)
		} else {
			logger().Info("inspecting", "pid", execElf.Pid, "comm", execElf.CmdExePath)
			offsets, err = goexec.InspectOffsets(&execElf, functions)
			if err != nil {
				logger().Info("Go HTTP/gRPC support not detected. Using only generic instrumentation.", "error", err)
			}
		}
	}

	return &execElf, offsets, nil
}

func inspectByPort(ctx context.Context, cfg *ebpfcommon.TracerConfig, functions []string) (*exec.FileInfo, *goexec.Offsets, error) {
	finder := exec.OwnedPort(cfg.Port)

	elfs, err := exec.FindExecELF(ctx, finder)
	for _, exec := range elfs {
		defer exec.ELF.Close()
	}
	if err != nil {
		return nil, nil, fmt.Errorf("looking for executable ELF: %w", err)
	}

	pidMap := map[int32]exec.FileInfo{}

	var fallBackInfos []exec.FileInfo
	var goProxies []exec.FileInfo

	// look for suitable Go application first
	for _, execElf := range elfs {
		var offsets *goexec.Offsets
		var err error

		if cfg.SkipGoSpecificTracers {
			logger().Info("skipping inspection for Go functions", "pid", execElf.Pid, "comm", execElf.CmdExePath)
		} else {
			logger().Info("inspecting", "pid", execElf.Pid, "comm", execElf.CmdExePath)
			offsets, err = goexec.InspectOffsets(&execElf, functions)
		}

		if cfg.SkipGoSpecificTracers || err != nil {
			fallBackInfos = append(fallBackInfos, execElf)
			pidMap[execElf.Pid] = execElf
			logger().Info("adding fall-back generic executable", "pid", execElf.Pid, "comm", execElf.CmdExePath)
			continue
		}

		// we found go offsets, let's see if this application is not a proxy
		for f := range offsets.Funcs {
			// if we find anything of interest other than the Go runtime, we consider this a valid application
			if !strings.HasPrefix(f, "runtime.") {
				return &execElf, offsets, nil
			}
		}

		logger().Info("ignoring Go proxy for now", "pid", execElf.Pid, "comm", execElf.CmdExePath)
		goProxies = append(goProxies, execElf)
		pidMap[execElf.Pid] = execElf
	}

	var execElf exec.FileInfo

	if len(goProxies) != 0 {
		execElf = goProxies[len(goProxies)-1]
	} else if len(fallBackInfos) != 0 {
		execElf = fallBackInfos[len(fallBackInfos)-1]
	} else {
		return nil, nil, fmt.Errorf("looking for executable ELF, no suitable processes found")
	}

	// check if the executable is a subprocess of another we have found, f so use the parent
	parentElf, ok := pidMap[execElf.Ppid]

	if ok {
		execElf = parentElf
	}

	logger().Info("Go HTTP/gRPC support not detected. Using only generic instrumentation.")
	logger().Info("instrumented", "comm", execElf.CmdExePath, "pid", execElf.Pid)

	return &execElf, nil, nil
}

func printVerifierErrorInfo(err error) {
	var ve *ebpf.VerifierError
	if errors.As(err, &ve) {
		_, _ = fmt.Fprintf(os.Stderr, "Error Log:\n %v\n", strings.Join(ve.Log, "\n"))
	}
}

func bpfMount(pinPath string) error {
	return unix.Mount(pinPath, pinPath, "bpf", 0, "")
}
