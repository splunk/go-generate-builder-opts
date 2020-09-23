/*
 * Copyright 2020 Splunk Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"strings"
	"unicode"
)

var _ flag.Value = (*flagStringSet)(nil)

// flagStringSet is a custom flag value type that acts like a set.
type flagStringSet map[string]struct{}

func (s *flagStringSet) String() string {
	slc := make([]string, 0)
	for k := range *s {
		slc = append(slc, k)
	}
	return strings.Join(slc, ", ")
}

func (s *flagStringSet) Set(str string) error {
	slc := strings.Split(str, ",")
	result := make(map[string]struct{})
	for _, str := range slc {
		result[str] = struct{}{}
	}
	*s = result
	return nil
}

type flagOpts struct {
	definitionFile              string
	outFile                     string
	structTypeName              string
	exportFnType                bool
	generateForUnexportedFields bool
	ignoreUnsupported           bool
	skipStructFields            flagStringSet
}

func getFlags() *flagOpts {
	opts := new(flagOpts)
	fs := flag.NewFlagSet("", flag.ExitOnError)
	fs.StringVar(&opts.definitionFile, "definitionFile", "", "file where type is defined (required)")
	fs.StringVar(&opts.outFile, "outFile", "", "file to write builder option functions to; stdout if omitted (optional)")
	fs.StringVar(&opts.structTypeName, "structTypeName", "", "fieldName of type to generate builder options for (required)")
	fs.BoolVar(&opts.exportFnType, "exportOptionFuncType", true, "whether to export the configuration function type (optional)")
	fs.BoolVar(&opts.generateForUnexportedFields, "generateForUnexportedFields", false, "whether to generate configuration functions for unexported fields (optional)")
	fs.BoolVar(&opts.ignoreUnsupported, "ignoreUnsupported", true, "whether to skip fields whose type we can't handle (error otherwise) (optional)")
	fs.Var(&opts.skipStructFields, "skipStructFields", "comma-separated list of struct fields to ignore (exact match)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fail(err.Error())
	}

	if opts.definitionFile == "" || opts.structTypeName == "" {
		fs.Usage()
		fail("")
	}

	return opts
}

func fail(msg string) {
	if msg != "" {
		fmt.Fprintln(os.Stderr, msg)
	}
	os.Exit(1)
}

func main() {
	opts := getFlags()

	// Handle all but I/O in the run function to make testing easier.
	outReader, err := run(opts)
	if err != nil {
		fail(err.Error())
	}

	var dest io.Writer
	if opts.outFile == "" {
		dest = os.Stdout
	} else {
		dest, err = os.OpenFile(opts.outFile, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			fail(err.Error())
		}
	}

	if _, err := io.Copy(dest, outReader); err != nil {
		fail(err.Error())
	}
}

func run(opts *flagOpts) (io.Reader, error) {
	// Read input file
	fset := token.NewFileSet()
	astF, err := parser.ParseFile(fset, opts.definitionFile, nil, 0)
	if err != nil {
		return nil, err
	}

	// Look for specified struct type.
	structType, ok := findRequestedStructType(astF, opts.structTypeName)
	if !ok {
		return nil, fmt.Errorf("could not find struct type in definition file")
	}

	fnTypeIdent := funcTypeIdent(structType.Name.Name, opts.exportFnType)
	fnParamType := &ast.StarExpr{
		X: structType.Name,
	}

	// Initialize output
	astOut := &ast.File{Name: astF.Name}

	// Add type definition for functional option function signature
	withTypeDef(astOut, fnTypeIdent, fnParamType)

	// Add function for each applicable struct field
	if err := withFuncs(
		astOut,
		structType,
		fnTypeIdent,
		fnParamType,
		opts.generateForUnexportedFields,
		opts.ignoreUnsupported,
		opts.skipStructFields); err != nil {
		return nil, err
	}

	// Generate output file
	out := new(bytes.Buffer)
	if err := printer.Fprint(out, token.NewFileSet(), astOut); err != nil {
		return nil, err
	}

	return out, nil
}

// findRequestedStructType searches the input file for a struct type with name
// structName. If found, return the type spec, true; else return nil, false.
func findRequestedStructType(f *ast.File, structName string) (*ast.TypeSpec, bool) {
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		if genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			if _, ok := typeSpec.Type.(*ast.StructType); ok && typeSpec.Name.Name == structName {
				return typeSpec, true
			}
		}
	}

	return nil, false
}

// funcTypeIdent returns the identifier for the name of the functional option
// function type.
func funcTypeIdent(structName string, exportFnType bool) *ast.Ident {
	const nameF = "%sFieldSetter"
	var casedStructName string
	if exportFnType {
		casedStructName = withFirstCharUppper(structName)
	} else {
		casedStructName = withFirstCharLower(structName)
	}
	return ast.NewIdent(fmt.Sprintf(nameF, casedStructName))
}

// withTypeDef makes a type definition declaration for the functional option
// function type and adds it to astOut.
func withTypeDef(astOut *ast.File, fnIdent *ast.Ident, paramType *ast.StarExpr) {
	fnType := &ast.FuncType{
		Params: &ast.FieldList{
			List: []*ast.Field{
				{
					Type: paramType,
				},
			},
		},
	}

	typeSpec := &ast.TypeSpec{
		Name: fnIdent,
		Type: fnType,
	}

	astOut.Decls = append(astOut.Decls, &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			typeSpec,
		},
	})
}

// withFuncs creates a functional option function for each applicable field and
// adds it to astOut.
func withFuncs(
	astOut *ast.File,
	structType *ast.TypeSpec,
	fnIdent *ast.Ident,
	fnParamType *ast.StarExpr,
	generateForUnexportedFields, ignoreUnsupported bool,
	skipStructFields map[string]struct{},
) error {
	structTypeTyped, ok := structType.Type.(*ast.StructType)
	if !ok {
		panic("bad type for struct type")
	}

	var numFnsAdded int

	// Look at fields. Each entry in list is actually a list: could be embedded
	// field (length 0), "regular" field (length 1), or multiple named fields
	// with same type (length > 1).
	for _, field := range structTypeTyped.Fields.List {

		// No embedded fields
		if len(field.Names) == 0 {
			if ignoreUnsupported {
				continue
			} else {
				return fmt.Errorf("embedded fields disallowed")
			}
		}

		// No fields whose type is imported from another package
		var fieldContainsImport bool
		ast.Inspect(field, func(n ast.Node) bool {
			_, ok := n.(*ast.SelectorExpr)
			if ok {
				fieldContainsImport = true
				return false
			}
			return true
		})
		if fieldContainsImport {
			if ignoreUnsupported {
				continue
			} else {
				return fmt.Errorf("cannot generate for fields whose type is imported")
			}
		}

		// Now that we're operating on non-imported types and non-embedded
		// fields, let's look at each actual field name and generate a setter
		// for it.
		for _, fieldName := range field.Names {

			if _, ok := skipStructFields[fieldName.Name]; ok {
				continue
			}

			if unicode.IsLower(rune(fieldName.Name[0])) && !generateForUnexportedFields {
				continue
			}

			outerParamIdent := ast.NewIdent(withFirstCharLower(fieldName.Name) + "Gen")
			newFunc := &ast.FuncDecl{
				Name: ast.NewIdent("Set" + withFirstCharUppper(fieldName.Name)),
				Type: &ast.FuncType{
					Params: &ast.FieldList{
						List: []*ast.Field{
							{
								Names: []*ast.Ident{outerParamIdent},
								Type:  field.Type,
							},
						},
					},
					Results: &ast.FieldList{
						List: []*ast.Field{{Type: fnIdent}},
					},
				},
				Body: &ast.BlockStmt{
					List: []ast.Stmt{
						&ast.ReturnStmt{
							Results: []ast.Expr{
								getInnerFn(
									structType.Name,
									fieldName,
									outerParamIdent,
									fnParamType,
								),
							},
						},
					},
				},
			}
			astOut.Decls = append(astOut.Decls, newFunc)
			numFnsAdded++
		}
	}

	if numFnsAdded == 0 {
		return fmt.Errorf("no fields in struct (aside from ignored errors)")
	}

	return nil
}

// getInnerFn returns a function literal for the inner function - the one that
// does the assignment of the struct field.
func getInnerFn(
	structTypeIdent, fieldIdent, outerParamIdent *ast.Ident,
	innerParamType *ast.StarExpr,
) *ast.FuncLit {
	paramIdent := ast.NewIdent(withFirstCharLower(structTypeIdent.Name) + "Gen")
	return &ast.FuncLit{
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{paramIdent},
						Type:  innerParamType,
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   paramIdent,
							Sel: fieldIdent,
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						outerParamIdent,
					},
				},
			},
		},
	}
}

func withFirstCharLower(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToLower(s[0:1]) + s[1:]
}

func withFirstCharUppper(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[0:1]) + s[1:]
}
