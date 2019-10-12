// vet
package vet

import (
	"fmt"
	"go/ast"

	// "go/parser"

	"go/build"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"golang.org/x/tools/go/buildutil"
	"golang.org/x/tools/go/packages"

	"github.com/helloyi/goastch"
	"github.com/helloyi/goastch/goastcher"
	"github.com/helloyi/govet/config"
)

type (
	// Vet ...
	Vet struct {
		ErrorLimit int
		Schedule   []*Check
		Errors     []Error

		pkgPath  string
		pkgInfos map[string]*PackageInfo
		buildCtx *build.Context
	}

	// Error ...
	Error struct {
		pos token.Position
		msg string
	}

	// Check ...
	Check struct {
		Info    *types.Info
		Node    ast.Node // ast.Package, ast.File
		Fset    *token.FileSet
		Ger     goastcher.Goastcher
		Message string
	}

	// PackageInfo ...
	PackageInfo struct {
		pkg   *packages.Package
		files map[string]*ast.File
	}
)

// New ...
func New(c *config.Config) (*Vet, error) {
	vet := &Vet{
		ErrorLimit: c.Choke,
		pkgPath:    c.Path,
		buildCtx:   c.Build,
	}

	// first load program and type check
	if err := vet.load(c); err != nil {
		return nil, err
	}

	// make schedule
	if err := vet.growSchedule(c, c.Path); err != nil {
		return nil, err
	}

	return vet, nil
}

// Do ...
func (v *Vet) Do() {
	for _, check := range v.Schedule {
		// var buf bytes.Buffer
		// printer.Fprint(&buf, check.Fset, check.Node)
		// fmt.Println(buf.String())
		binds, err := goastch.Find(check.Node, check.Info, check.Ger)
		if err != nil {
			v.appendError(nil, nil, err.Error())
		}
		for _, list := range binds {
			for _, node := range list {
				v.appendError(node, check.Fset, check.Message)
			}
		}
	}
	v.fatal()
}

func (v *Vet) fatal() {
	if len(v.Errors) == 0 {
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for _, err := range v.Errors {
		s := strings.TrimPrefix(err.pos.String(), v.pkgPath)
		pos := color.GreenString(".%s", s)
		fmt.Fprintf(w, "%s:\t%s\n", pos, err.msg)
	}
	_ = w.Flush()
	os.Exit(1)
}

func (v *Vet) appendError(node ast.Node, fset *token.FileSet, msg string) {
	if v.Errors == nil {
		v.Errors = make([]Error, 0, v.ErrorLimit)
	}
	if len(v.Errors) >= v.ErrorLimit {
		v.fatal()
	}

	var pos token.Position
	if node != nil {
		pos = fset.Position(node.Pos())
	}
	v.Errors = append(v.Errors, Error{
		pos: pos,
		msg: msg,
	})
}

func (v *Vet) growSchedule(c *config.Config, dir string) error {
	files, err := buildutil.ReadDir(v.buildCtx, dir)
	if err != nil {
		return err
	}

	filenames := make([]string, 0)
	for _, file := range files {
		path := filepath.Join(dir, file.Name())
		if c.Ignored[path] {
			continue
		}
		if strings.HasPrefix(filepath.Base(path), ".") {
			continue
		}

		if file.IsDir() {
			if file.Name() == "vendor" {
				continue
			}
			if err := v.growSchedule(c, path); err != nil {
				return err
			}
		}

		if filepath.Ext(path) != ".go" {
			continue
		}

		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		enabled := c.Override[path]
		if enabled != nil {
			node := v.getFile(file.Name())
			info := v.getInfo(dir)
			fset := v.getFset(dir)
			v.appendCheck(node, info, fset, enabled)
		} else {
			filenames = append(filenames, path)
		}
	}
	if len(filenames) == 0 {
		return nil
	}
	pkg := v.getPackage(filenames)
	info := v.getInfo(dir)
	fset := v.getFset(dir)
	enabled := c.Override[dir]
	if enabled == nil {
		enabled = c.Enabled
	}
	v.appendCheck(pkg, info, fset, enabled)
	return nil
}

// appendCheck ...
func (v *Vet) appendCheck(node ast.Node, info *types.Info, fset *token.FileSet, checkers map[string]*config.Checker) {
	if node == nil {
		return
	}
	if v.Schedule == nil {
		v.Schedule = make([]*Check, 0)
	}

	for _, checker := range checkers {
		v.Schedule = append(v.Schedule,
			&Check{
				Node:    node,
				Fset:    fset,
				Info:    info,
				Ger:     checker.Ger,
				Message: checker.Message,
			})
	}
}

func (v *Vet) getInfo(dir string) *types.Info {
	return v.pkgInfos[dir].pkg.TypesInfo
}

func (v *Vet) getFset(dir string) *token.FileSet {
	return v.pkgInfos[dir].pkg.Fset
}

func (v *Vet) getFile(filename string) *ast.File {
	dir := filepath.Dir(filename)
	pkgInfo := v.pkgInfos[dir]
	return pkgInfo.files[filename]
}

func (v *Vet) getPackage(filenames []string) *ast.Package {
	if len(filenames) == 0 {
		return nil
	}

	dir := filepath.Dir(filenames[0])
	pkgInfo := v.pkgInfos[dir]
	pkgName := pkgInfo.pkg.Name

	pkg := &ast.Package{
		Name:  pkgName,
		Files: make(map[string]*ast.File),
	}

	for _, filename := range filenames {
		pkg.Files[filename] = pkgInfo.files[filename]
	}
	return pkg
}

// load ...
func (v *Vet) load(c *config.Config) error {
	cfg := &packages.Config{Mode: packages.LoadSyntax}
	pkgs, err := packages.Load(cfg, filepath.Join(v.pkgPath, "..."))
	if err != nil {
		return err
	}
	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(1)
	}

	v.pkgInfos = make(map[string]*PackageInfo, len(pkgs))
	for _, pkg := range pkgs {
		pkgInfo := &PackageInfo{
			pkg:   pkg,
			files: make(map[string]*ast.File, len(pkg.Syntax)),
		}
		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Package).Name()
			pkgInfo.files[fname] = file
		}
		pkgdir := filepath.Dir(pkg.GoFiles[0])
		v.pkgInfos[pkgdir] = pkgInfo
	}
	return err
}
