// Copyright 2020 - 2024 The xgen Authors. All rights reserved. Use of this
// source code is governed by a BSD-style license that can be found in the
// LICENSE file.
//
// Package xgen written in pure Go providing a set of functions that allow you
// to parse XSD (XML schema files). This library needs Go version 1.10 or
// later.

package xgen

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/net/html/charset"
)

// Options holds user-defined overrides and runtime data that are used when
// parsing from an XSD document.
type Options struct {
	FilePath            string
	FileDir             string
	InputDir            string
	OutputDir           string
	Extract             bool
	Dependency          bool
	Lang                string
	Package             string
	TargetNamespace     string
	NamespacePrefixMap  map[string]string
	ReferencedElements  map[string]bool
	IncludeMap          map[string]bool
	LocalNameNSMap      map[string]string
	NSSchemaLocationMap map[string]string
	ParseFileList       map[string]bool
	ParseFileMap        map[string][]interface{}
	ProtoTree           []interface{}
	RemoteSchema        map[string][]byte
	Hook                Hook

	InElement        string
	CurrentEle       string
	InGroup          int
	InUnion          bool
	InAttributeGroup bool
	InPluralSequence []bool

	SimpleType     *Stack
	ComplexType    *Stack
	Element        *Stack
	Attribute      *Stack
	Group          *Stack
	AttributeGroup *Stack
	Choice         *Stack
}

// NewParser creates a new parser options for the Parse. Useful for XML schema
// parsing.
func NewParser(options *Options) *Options {
	return options
}

// Parse reads XML documents and return proto tree for every element in the
// documents by given options. If value of the property extract is false,
// parse will fetch schema used in <import> or <include> statements.
func (opt *Options) Parse() (err error) {
	if opt.FilePath == "" {
		opt.FilePath = opt.InputDir
	}
	if opt.FilePath == "" {
		return fmt.Errorf("must specify file path or input directory")
	}
	opt.FileDir = filepath.Dir(opt.FilePath)
	if opt.NamespacePrefixMap == nil {
		opt.NamespacePrefixMap = make(map[string]string)
	}
	if opt.ReferencedElements == nil {
		opt.ReferencedElements = make(map[string]bool)
	}
	if len(opt.ReferencedElements) == 0 && opt.InputDir != "" {
		var refs map[string]bool
		refs, err = collectQualifiedElementRefs(opt.InputDir)
		if err != nil {
			return
		}
		for name := range refs {
			opt.ReferencedElements[name] = true
		}
	}
	var fi os.FileInfo
	fi, err = os.Stat(opt.FilePath)
	if err != nil {
		return
	}
	if fi.IsDir() {
		return opt.parseDirectory()
	}
	var xmlFile *os.File
	xmlFile, err = os.Open(opt.FilePath)
	if err != nil {
		return
	}
	defer xmlFile.Close()
	if !opt.Extract {
		opt.ParseFileList[opt.FilePath] = true
		opt.ParseFileMap[opt.FilePath] = opt.ProtoTree
	}
	opt.ProtoTree = make([]interface{}, 0)

	opt.InElement = ""
	opt.CurrentEle = ""
	opt.InGroup = 0
	opt.InUnion = false
	opt.InAttributeGroup = false
	opt.TargetNamespace = ""

	opt.SimpleType = NewStack()
	opt.ComplexType = NewStack()
	opt.Element = NewStack()
	opt.Attribute = NewStack()
	opt.Group = NewStack()
	opt.AttributeGroup = NewStack()
	opt.Choice = NewStack()

	decoder := xml.NewDecoder(xmlFile)
	decoder.CharsetReader = charset.NewReaderLabel
	for {
		token, _ := decoder.Token()
		if token == nil {
			break
		}

		next := true
		switch element := token.(type) {
		case xml.StartElement:
			if opt.Hook != nil {
				next, err = opt.Hook.OnStartElement(opt, element, opt.ProtoTree)
				if err != nil {
					return err
				}

				// skip to next element/token
				if !next {
					continue
				}
			}

			opt.InElement = element.Name.Local
			funcName := fmt.Sprintf("On%s", MakeFirstUpperCase(opt.InElement))
			if err = callFuncByName(opt, funcName, []reflect.Value{reflect.ValueOf(element), reflect.ValueOf(opt.ProtoTree)}); err != nil {
				return
			}
		case xml.EndElement:
			if opt.Hook != nil {
				next, err = opt.Hook.OnEndElement(opt, element, opt.ProtoTree)
				if err != nil {
					return err
				}

				// skip to next element/token
				if !next {
					continue
				}
			}

			funcName := fmt.Sprintf("End%s", MakeFirstUpperCase(element.Name.Local))
			if err = callFuncByName(opt, funcName, []reflect.Value{reflect.ValueOf(element), reflect.ValueOf(opt.ProtoTree)}); err != nil {
				return
			}
		case xml.CharData:
			if opt.Hook != nil {
				next, err = opt.Hook.OnCharData(opt, string(element), opt.ProtoTree)
				if err != nil {
					return err
				}

				// skip to next element/token
				if !next {
					continue
				}
			}

			if err = opt.OnCharData(string(element), opt.ProtoTree); err != nil {
				return
			}
		default:
		}
	}

	if !opt.Extract {
		opt.ParseFileList[opt.FilePath] = true
		opt.ParseFileMap[opt.FilePath] = opt.ProtoTree
		path := filepath.Join(opt.OutputDir, strings.TrimPrefix(opt.FilePath, opt.InputDir))
		packageName := opt.Package
		if opt.Lang == "Go" {
			packageName = goPackageName(opt.Package)
		}
		if err := PrepareOutputDir(filepath.Dir(path)); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		generator := &CodeGenerator{
			Lang:            opt.Lang,
			Package:         packageName,
			TargetNamespace: opt.TargetNamespace,
			NamespacePrefix: cloneStringMap(opt.NamespacePrefixMap),
			ReferencedNames: cloneBoolMap(opt.ReferencedElements),
			LocalNameNSMap:  cloneStringMap(opt.LocalNameNSMap),
			File:            path,
			ParseFileMap:    opt.ParseFileMap,
			ProtoTree:       opt.ProtoTree,
			StructAST:       map[string]string{},
			Hook:            opt.Hook,
		}
		funcName := fmt.Sprintf("Gen%s", MakeFirstUpperCase(opt.Lang))
		if err = callFuncByName(generator, funcName, []reflect.Value{}); err != nil {
			return
		}
	}
	return
}

func (opt *Options) parseDirectory() error {
	files, err := GetFileList(opt.FilePath)
	if err != nil {
		return err
	}
	inputDir := opt.InputDir
	if inputDir == "" {
		inputDir = opt.FilePath
	}
	for _, file := range files {
		if filepath.Ext(file) != ".xsd" {
			continue
		}
		parser := NewParser(&Options{
			FilePath:            file,
			InputDir:            inputDir,
			OutputDir:           opt.OutputDir,
			Extract:             opt.Extract,
			Lang:                opt.Lang,
			Package:             opt.Package,
			NamespacePrefixMap:  opt.NamespacePrefixMap,
			ReferencedElements:  opt.ReferencedElements,
			IncludeMap:          make(map[string]bool),
			LocalNameNSMap:      make(map[string]string),
			NSSchemaLocationMap: make(map[string]string),
			ParseFileList:       make(map[string]bool),
			ParseFileMap:        make(map[string][]interface{}),
			ProtoTree:           make([]interface{}, 0),
			RemoteSchema:        opt.RemoteSchema,
			Hook:                opt.Hook,
		})
		if err := parser.Parse(); err != nil {
			return err
		}
	}
	return nil
}

func (opt *Options) qualifyTypeReference(value string) string {
	prefix := getNSPrefix(value)
	if prefix == "" {
		return trimNSPrefix(value)
	}
	namespace := opt.parseNS(value)
	if namespace == "" || namespace == opt.TargetNamespace {
		return trimNSPrefix(value)
	}
	return fmt.Sprintf("%s:%s", prefix, trimNSPrefix(value))
}

func (opt *Options) qualifyResolvedType(originalValue, resolvedValue string) string {
	if isBuildInTypeByLang(resolvedValue, opt.Lang) {
		return resolvedValue
	}
	if _, ok := goBuildinType[resolvedValue]; ok {
		return resolvedValue
	}
	if getNSPrefix(resolvedValue) != "" {
		return opt.qualifyTypeReference(resolvedValue)
	}
	prefix := getNSPrefix(originalValue)
	if prefix == "" {
		return trimNSPrefix(resolvedValue)
	}
	namespace := opt.parseNS(originalValue)
	if namespace == "" || namespace == opt.TargetNamespace {
		return trimNSPrefix(resolvedValue)
	}
	return fmt.Sprintf("%s:%s", prefix, trimNSPrefix(resolvedValue))
}

func (opt *Options) preserveGoTypeReference(value string, XSDSchema []interface{}) (string, bool) {
	if opt.Lang != "Go" {
		return "", false
	}
	simpleType := findSimpleTypeInTree(value, XSDSchema)
	if simpleType == nil {
		return "", false
	}
	if simpleType.Union && len(simpleType.MemberTypes) > 0 {
		return opt.qualifyTypeReference(value), true
	}
	baseType := getBasefromSimpleType(simpleType.Base, XSDSchema)
	if len(simpleType.Restriction.Enum) > 0 && goSupportsSimpleTypeValidation(baseType) {
		return opt.qualifyTypeReference(value), true
	}
	return "", false
}

// GetValueType convert XSD schema value type to the build-in type for the
// given value and proto tree.
func (opt *Options) GetValueType(value string, XSDSchema []interface{}) (valueType string, err error) {
	if buildType, ok := getBuildInTypeByLang(trimNSPrefix(value), opt.Lang); ok {
		valueType = buildType
		return
	}
	if valueType, ok := opt.preserveGoTypeReference(value, XSDSchema); ok {
		return valueType, nil
	}
	valueType = getBasefromSimpleType(value, XSDSchema)
	if valueType != value && valueType != trimNSPrefix(value) && valueType != "" {
		valueType = opt.qualifyResolvedType(value, valueType)
		return
	}
	if opt.Extract {
		valueType = opt.qualifyTypeReference(value)
		return
	}
	schemaLocation := opt.NSSchemaLocationMap[opt.parseNS(value)]
	if isValidURL(schemaLocation) {
		return
	}
	xsdFile := filepath.Join(opt.FileDir, schemaLocation)
	var fi os.FileInfo
	fi, err = os.Stat(xsdFile)
	if err != nil {
		return
	}
	if fi.IsDir() {
		// extract type of value from include schema.
		valueType = ""
		for include := range opt.IncludeMap {
			parser := NewParser(&Options{
				FilePath:            filepath.Join(opt.FileDir, include),
				InputDir:            opt.InputDir,
				OutputDir:           opt.OutputDir,
				Extract:             true,
				Package:             opt.Package,
				Lang:                opt.Lang,
				IncludeMap:          cloneBoolMap(opt.IncludeMap),
				LocalNameNSMap:      cloneStringMap(opt.LocalNameNSMap),
				NamespacePrefixMap:  opt.NamespacePrefixMap,
				ReferencedElements:  opt.ReferencedElements,
				NSSchemaLocationMap: cloneStringMap(opt.NSSchemaLocationMap),
				ParseFileList:       opt.ParseFileList,
				ParseFileMap:        opt.ParseFileMap,
				ProtoTree:           make([]interface{}, 0),
				RemoteSchema:        opt.RemoteSchema,
				Hook:                opt.Hook,
			})
			if parser.Parse() != nil {
				return
			}
			if vt, ok := opt.preserveGoTypeReference(value, parser.ProtoTree); ok {
				valueType = vt
			} else if vt := getBasefromSimpleType(value, parser.ProtoTree); vt != value && vt != trimNSPrefix(value) {
				valueType = opt.qualifyResolvedType(value, vt)
			}
		}
		if valueType == "" {
			valueType = opt.qualifyTypeReference(value)
		}
		return
	}

	depXSDSchema, ok := opt.ParseFileMap[xsdFile]
	if !ok {
		parser := NewParser(&Options{
			FilePath:            xsdFile,
			InputDir:            opt.InputDir,
			OutputDir:           opt.OutputDir,
			Extract:             false,
			Dependency:          true,
			Package:             opt.Package,
			Lang:                opt.Lang,
			IncludeMap:          cloneBoolMap(opt.IncludeMap),
			LocalNameNSMap:      cloneStringMap(opt.LocalNameNSMap),
			NamespacePrefixMap:  opt.NamespacePrefixMap,
			ReferencedElements:  opt.ReferencedElements,
			NSSchemaLocationMap: cloneStringMap(opt.NSSchemaLocationMap),
			ParseFileList:       opt.ParseFileList,
			ParseFileMap:        opt.ParseFileMap,
			ProtoTree:           make([]interface{}, 0),
			RemoteSchema:        opt.RemoteSchema,
			Hook:                opt.Hook,
		})
		if parser.Parse() != nil {
			return
		}
		depXSDSchema = parser.ProtoTree
	}
	if valueType, ok = opt.preserveGoTypeReference(value, depXSDSchema); ok {
		return valueType, nil
	}
	valueType = getBasefromSimpleType(value, depXSDSchema)
	if valueType != value && valueType != trimNSPrefix(value) && valueType != "" {
		valueType = opt.qualifyResolvedType(value, valueType)
		return
	}
	parser := NewParser(&Options{
		FilePath:            xsdFile,
		InputDir:            opt.InputDir,
		OutputDir:           opt.OutputDir,
		Extract:             true,
		Package:             opt.Package,
		Lang:                opt.Lang,
		IncludeMap:          cloneBoolMap(opt.IncludeMap),
		LocalNameNSMap:      cloneStringMap(opt.LocalNameNSMap),
		NamespacePrefixMap:  opt.NamespacePrefixMap,
		ReferencedElements:  opt.ReferencedElements,
		NSSchemaLocationMap: cloneStringMap(opt.NSSchemaLocationMap),
		ParseFileList:       opt.ParseFileList,
		ParseFileMap:        opt.ParseFileMap,
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        opt.RemoteSchema,
		Hook:                opt.Hook,
	})
	if parser.Parse() != nil {
		return
	}
	if valueType, ok = opt.preserveGoTypeReference(value, parser.ProtoTree); ok {
		return valueType, nil
	}
	valueType = getBasefromSimpleType(value, parser.ProtoTree)
	if valueType == value || valueType == trimNSPrefix(value) || valueType == "" {
		valueType = opt.qualifyTypeReference(value)
		return
	}
	valueType = opt.qualifyResolvedType(value, valueType)
	return
}
