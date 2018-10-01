package ldbuilder

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"

	"github.com/QMSTR/qmstr/pkg/builder"
	"github.com/QMSTR/qmstr/pkg/common"
	"github.com/QMSTR/qmstr/pkg/gnubuilder"
	"github.com/spf13/pflag"

	"github.com/QMSTR/qmstr/pkg/qmstr/service"
)

const undef = "undef"

const (
	linkedTrg = "linkedtarget"
	obj       = "objectfile"
	src       = "sourcecode"
)

const mode = "Link"

type LdBuilder struct {
	Input      []string
	Output     []string
	WorkDir    string
	LinkLibs   []string
	LibPath    []string
	Args       []string
	ActualLibs map[string]string
	staticLink bool
	StaticLibs map[string]struct{}
	builder.GeneralBuilder
}

func NewLdBuilder(workDir string, logger *log.Logger, debug bool) *LdBuilder {
	return &LdBuilder{[]string{}, []string{}, workDir, []string{}, []string{}, []string{}, map[string]string{}, false, map[string]struct{}{}, builder.GeneralBuilder{logger, debug}}
}

//TODO use ccache
func (ld *LdBuilder) GetPrefix() (string, error) {
	return "", errors.New("ld not prefixed")
}

func (ld *LdBuilder) GetName() string {
	return "GNU ld linker"
}

func (ld *LdBuilder) Analyze(commandline []string) (*service.BuildMessage, error) {
	if err := ld.parseCommandLine(commandline[1:]); err != nil {
		return nil, fmt.Errorf("Failed to parse commandline: %v", err)
	}

	if ld.staticLink {
		ld.Logger.Printf("ld linking statically")
	} else {
		ld.Logger.Printf("ld linking")
	}
	fileNodes := []*service.FileNode{}
	linkedTarget := builder.NewFileNode(common.BuildCleanPath(ld.WorkDir, ld.Output[0], false), linkedTrg)
	dependencies := []*service.FileNode{}
	for _, inFile := range ld.Input {
		inputFileNode := &service.FileNode{}
		ext := filepath.Ext(inFile)
		if ext == ".o" {
			inputFileNode = builder.NewFileNode(common.BuildCleanPath(ld.WorkDir, inFile, false), obj)
		} else if ext == ".c" {
			inputFileNode = builder.NewFileNode(common.BuildCleanPath(ld.WorkDir, inFile, false), src)
		} else {
			inputFileNode = builder.NewFileNode(common.BuildCleanPath(ld.WorkDir, inFile, false), linkedTrg)
		}
		dependencies = append(dependencies, inputFileNode)
	}
	err := gnubuilder.FindActualLibraries(ld.ActualLibs, ld.LinkLibs, ld.LibPath, ld.staticLink, ld.StaticLibs)
	if err != nil {
		ld.Logger.Fatalf("Failed to collect dependencies: %v", err)
	}
	for _, actualLib := range ld.ActualLibs {
		linkLib := builder.NewFileNode(common.BuildCleanPath(ld.WorkDir, actualLib, false), linkedTrg)
		dependencies = append(dependencies, linkLib)
	}
	linkedTarget.DerivedFrom = dependencies
	fileNodes = append(fileNodes, linkedTarget)
	return &service.BuildMessage{FileNodes: fileNodes}, nil
}

func (ld *LdBuilder) parseCommandLine(args []string) error {
	if ld.Debug {
		ld.Logger.Printf("Parsing arguments: %v", args)
	}

	// remove all flags we don't care about but that would break parsing
	ld.Args = gnubuilder.CleanCmdLine(args, ld.Logger, ld.Debug, ld.staticLink, ld.StaticLibs, mode)

	ldFlags := pflag.NewFlagSet("ld", pflag.ContinueOnError)
	ldFlags.StringP("output", "o", undef, "output")
	ldFlags.StringSliceVarP(&ld.LinkLibs, "linklib", "l", []string{}, "link libs")
	ldFlags.StringSliceVarP(&ld.LibPath, "linklibdir", "L", []string{}, "search dir for link libs")

	if ld.Debug {
		ld.Logger.Printf("Parsing cleaned commandline: %v", ld.Args)
	}
	err := ldFlags.Parse(ld.Args)
	if err != nil {
		return fmt.Errorf("Unrecoverable commandline parsing error: %v", err)
	}

	ld.Input = ldFlags.Args()

	if output, err := ldFlags.GetString("output"); err == nil && output != undef {
		ld.Output = []string{output}
	} else {
		// no output defined
		if len(ld.Input) == 0 {
			// No input no output
			return nil
		}
		ld.Output = []string{"a.out"}
	}
	return nil
}