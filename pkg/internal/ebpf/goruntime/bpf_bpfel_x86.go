// Code generated by bpf2go; DO NOT EDIT.
//go:build 386 || amd64
// +build 386 amd64

package goruntime

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type bpfFuncInvocation struct {
	StartMonotimeNs uint64
	Regs            struct {
		R15     uint64
		R14     uint64
		R13     uint64
		R12     uint64
		Rbp     uint64
		Rbx     uint64
		R11     uint64
		R10     uint64
		R9      uint64
		R8      uint64
		Rax     uint64
		Rcx     uint64
		Rdx     uint64
		Rsi     uint64
		Rdi     uint64
		OrigRax uint64
		Rip     uint64
		Cs      uint64
		Eflags  uint64
		Rsp     uint64
		Ss      uint64
	}
}

type bpfGoroutineMetadata struct {
	Parent    uint64
	Timestamp uint64
}

// loadBpf returns the embedded CollectionSpec for bpf.
func loadBpf() (*ebpf.CollectionSpec, error) {
	reader := bytes.NewReader(_BpfBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("can't load bpf: %w", err)
	}

	return spec, err
}

// loadBpfObjects loads bpf and converts it into a struct.
//
// The following types are suitable as obj argument:
//
//	*bpfObjects
//	*bpfPrograms
//	*bpfMaps
//
// See ebpf.CollectionSpec.LoadAndAssign documentation for details.
func loadBpfObjects(obj interface{}, opts *ebpf.CollectionOptions) error {
	spec, err := loadBpf()
	if err != nil {
		return err
	}

	return spec.LoadAndAssign(obj, opts)
}

// bpfSpecs contains maps and programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfSpecs struct {
	bpfProgramSpecs
	bpfMapSpecs
}

// bpfSpecs contains programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfProgramSpecs struct {
	UprobeProcGoexit1     *ebpf.ProgramSpec `ebpf:"uprobe_proc_goexit1"`
	UprobeProcNewproc1    *ebpf.ProgramSpec `ebpf:"uprobe_proc_newproc1"`
	UprobeProcNewproc1Ret *ebpf.ProgramSpec `ebpf:"uprobe_proc_newproc1_ret"`
}

// bpfMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfMapSpecs struct {
	Events                *ebpf.MapSpec `ebpf:"events"`
	Newproc1              *ebpf.MapSpec `ebpf:"newproc1"`
	OngoingGoroutines     *ebpf.MapSpec `ebpf:"ongoing_goroutines"`
	OngoingServerRequests *ebpf.MapSpec `ebpf:"ongoing_server_requests"`
}

// bpfObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfObjects struct {
	bpfPrograms
	bpfMaps
}

func (o *bpfObjects) Close() error {
	return _BpfClose(
		&o.bpfPrograms,
		&o.bpfMaps,
	)
}

// bpfMaps contains all maps after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfMaps struct {
	Events                *ebpf.Map `ebpf:"events"`
	Newproc1              *ebpf.Map `ebpf:"newproc1"`
	OngoingGoroutines     *ebpf.Map `ebpf:"ongoing_goroutines"`
	OngoingServerRequests *ebpf.Map `ebpf:"ongoing_server_requests"`
}

func (m *bpfMaps) Close() error {
	return _BpfClose(
		m.Events,
		m.Newproc1,
		m.OngoingGoroutines,
		m.OngoingServerRequests,
	)
}

// bpfPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfPrograms struct {
	UprobeProcGoexit1     *ebpf.Program `ebpf:"uprobe_proc_goexit1"`
	UprobeProcNewproc1    *ebpf.Program `ebpf:"uprobe_proc_newproc1"`
	UprobeProcNewproc1Ret *ebpf.Program `ebpf:"uprobe_proc_newproc1_ret"`
}

func (p *bpfPrograms) Close() error {
	return _BpfClose(
		p.UprobeProcGoexit1,
		p.UprobeProcNewproc1,
		p.UprobeProcNewproc1Ret,
	)
}

func _BpfClose(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Do not access this directly.
//
//go:embed bpf_bpfel_x86.o
var _BpfBytes []byte
