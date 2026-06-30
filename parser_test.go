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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testFixtureDir = "test"
	// externalFixtureDir is where one copy their own XSDs to run validation on them. For a set
	// of XSDs to run tests on, see https://github.com/xuri/xsd. Note that external tests leave the
	// generated output for inspection to support use-cases of manual review of generated code
	externalFixtureDir = "data"
)

func TestParseGo(t *testing.T) {
	testParseForSource(t, "Go", "go", "go", testFixtureDir, false, nil)
}

// TestParseGoExternal runs tests on any external XSDs within the externalFixtureDir
func TestParseGoExternal(t *testing.T) {
	testParseForSource(t, "Go", "go", "go", externalFixtureDir, true, nil)
}

func TestParseGoFromInputDirOnly(t *testing.T) {
	codeDir := filepath.Join(testFixtureDir, "go")
	tempDir, err := ioutil.TempDir(codeDir, "inputdir-output-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(testFixtureDir, "xsd")
	parser := NewParser(&Options{
		InputDir:            inputDir,
		OutputDir:           tempDir,
		Lang:                "Go",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
	})
	require.NoError(t, parser.Parse())

	actualGenerated, err := ioutil.ReadFile(filepath.Join(tempDir, "base64.xsd.go"))
	require.NoError(t, err)
	assert.NotEmpty(t, actualGenerated)
}

// testParseForSource runs parsing tests for a given language. The sourceDirectory specifies the root of the
// input for the tests. The expected structure of the sourceDirectory is as follows:
//
//	source
//	├── xsd (with the input xsd files to run through the parser)
//	└── <langDirName> (with the expected generated code named <xsd-file>.<fileExt>
//
// The test cleans up files it generates unless leaveOutput is set to true. In which case, the generate file is left
// on disk for manual inspection under <sourceDirectory>/<langDirName>/output.
func testParseForSource(t *testing.T, lang string, fileExt string, langDirName string, sourceDirectory string, leaveOutput bool, hook Hook) {
	codeDir := filepath.Join(sourceDirectory, langDirName)

	outputDir := filepath.Join(codeDir, "output")
	if leaveOutput {
		err := PrepareOutputDir(outputDir)
		require.NoError(t, err)
	} else {
		tempDir, err := ioutil.TempDir(codeDir, "output-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		outputDir = tempDir
	}

	inputDir := filepath.Join(sourceDirectory, "xsd")
	files, err := GetFileList(inputDir)
	// Abort testing if the source directory doesn't include a xsd directory with inputs
	if os.IsNotExist(err) {
		return
	}

	require.NoError(t, err)
	for _, file := range files {
		if filepath.Ext(file) == ".xsd" {
			xsdName, err := filepath.Rel(inputDir, file)
			require.NoError(t, err)

			t.Run(xsdName, func(t *testing.T) {
				parser := NewParser(&Options{
					FilePath:            file,
					InputDir:            inputDir,
					OutputDir:           outputDir,
					Lang:                lang,
					IncludeMap:          make(map[string]bool),
					LocalNameNSMap:      make(map[string]string),
					NSSchemaLocationMap: make(map[string]string),
					ParseFileList:       make(map[string]bool),
					ParseFileMap:        make(map[string][]interface{}),
					ProtoTree:           make([]interface{}, 0),
					Hook:                hook,
				})
				err = parser.Parse()
				assert.NoError(t, err, file)
				generatedFileName := strings.TrimPrefix(file, inputDir) + "." + fileExt
				actualFilename := filepath.Join(outputDir, generatedFileName)

				actualGenerated, err := ioutil.ReadFile(actualFilename)
				assert.NoError(t, err)

				if lang == "Go" {
					assert.NotEmpty(t, actualGenerated, fmt.Sprintf("error in generated code for %s", file))
					return
				}

				expectedFilename := filepath.Join(codeDir, generatedFileName)
				expectedGenerated, err := ioutil.ReadFile(expectedFilename)
				assert.NoError(t, err)

				assert.Equal(t, string(expectedGenerated), string(actualGenerated), fmt.Sprintf("error in generated code for %s", file))
			})
		}
	}
}

func TestParseTypeScript(t *testing.T) {
	testParseForSource(t, "TypeScript", "ts", "ts", testFixtureDir, false, nil)
}

func TestParseTypeScriptExternal(t *testing.T) {
	testParseForSource(t, "TypeScript", "ts", "ts", externalFixtureDir, true, nil)
}

func TestParseC(t *testing.T) {
	testParseForSource(t, "C", "h", "c", testFixtureDir, false, nil)
}

func TestParseCExternal(t *testing.T) {
	testParseForSource(t, "C", "h", "c", externalFixtureDir, true, nil)
}

func TestParseJava(t *testing.T) {
	testParseForSource(t, "Java", "java", "java", testFixtureDir, false, nil)
}

func TestParseJavaExternal(t *testing.T) {
	testParseForSource(t, "Java", "java", "java", externalFixtureDir, true, nil)
}

func TestParseRust(t *testing.T) {
	testParseForSource(t, "Rust", "rs", "rs", testFixtureDir, false, nil)
}

func TestParseRustExternal(t *testing.T) {
	testParseForSource(t, "Rust", "rs", "rs", externalFixtureDir, true, nil)
}

type Appinfo struct {
	Doc    string
	Parent string
}

type AppinfoHook struct {
	Override          bool
	OnStartElementRan bool
	OnEndElementRan   bool
	OnCharDataRan     bool
	OnGenerateRan     bool

	Appinfo *Stack
}

func (h *AppinfoHook) ShouldOverride() bool {
	return h.Override
}

func (h *AppinfoHook) OnStartElement(opt *Options, ele xml.StartElement, protoTree []interface{}) (next bool, err error) {
	if ele.Name.Local != "appinfo" {
		return true, nil
	}

	h.OnStartElementRan = true

	a := &Appinfo{}

	a.Parent = opt.CurrentEle

	if opt.InElement != "" && opt.Element.Peek() != nil {
		a.Parent = opt.Element.Peek().(*Element).Name
	}

	switch opt.CurrentEle {
	case "simpleType":
		if opt.SimpleType.Peek() != nil {
			a.Parent = opt.SimpleType.Peek().(*SimpleType).Name
		}
	case "complexType":
		if opt.ComplexType.Peek() != nil {
			a.Parent = opt.ComplexType.Peek().(*ComplexType).Name
		}
	case "element":
		if opt.Element.Peek() != nil {
			a.Parent = opt.Element.Peek().(*Element).Name
		}
	}

	h.Appinfo.Push(a)

	return true, nil
}

func (h *AppinfoHook) OnEndElement(opt *Options, ele xml.EndElement, protoTree []interface{}) (next bool, err error) {
	if ele.Name.Local == "appinfo" {
		return true, nil
	}

	h.OnEndElementRan = true
	opt.ProtoTree = append(opt.ProtoTree, h.Appinfo.Pop())

	return true, nil
}

func (h *AppinfoHook) OnCharData(opt *Options, ele string, protoTree []interface{}) (next bool, err error) {
	if h.Appinfo.Peek() != nil {
		h.OnCharDataRan = true
		h.Appinfo.Peek().(*Appinfo).Doc = ele
	}
	return true, nil
}

func (h *AppinfoHook) OnGenerate(gen *CodeGenerator, protoName string, ele interface{}) (next bool, err error) {
	h.OnGenerateRan = false
	switch v := ele.(type) {
	case *ComplexType:
		if _, ok := gen.StructAST[v.Name]; !ok {
			// for this fixture, at least one attribute must exist, and must have a name
			for _, attribute := range v.Attributes {
				h.OnGenerateRan = h.OnGenerateRan || attribute.Name != ""
			}
		}
	}
	return true, nil
}

func (h *AppinfoHook) OnAddContent(gen *CodeGenerator, content *string) {
	// no-op
}

func TestParseGoWithAppinfoHook(t *testing.T) {
	appinfoHook := &AppinfoHook{}
	appinfoHook.Appinfo = NewStack()
	testParseForSource(t, "Go", "go", "go", testFixtureDir, false, appinfoHook)
	assert.True(t, appinfoHook.OnStartElementRan)
	assert.True(t, appinfoHook.OnEndElementRan)
	assert.True(t, appinfoHook.OnCharDataRan)
	assert.True(t, appinfoHook.OnGenerateRan)
}

// ComprehensiveHook tests skipping elements, filtering generation, and content modification
type ComprehensiveHook struct {
	SkippedAnnotations int
	SkippedTypes       []string
	ModifiedContent    bool
}

func (h *ComprehensiveHook) OnStartElement(opt *Options, ele xml.StartElement, protoTree []interface{}) (bool, error) {
	// Skip all <annotation> elements to test filtering behavior
	if ele.Name.Local == "annotation" {
		h.SkippedAnnotations++
		return false, nil // Skip processing this element
	}
	return true, nil
}

func (h *ComprehensiveHook) OnEndElement(opt *Options, ele xml.EndElement, protoTree []interface{}) (next bool, err error) {
	return true, nil
}

func (h *ComprehensiveHook) OnCharData(opt *Options, ele string, protoTree []interface{}) (next bool, err error) {
	return true, nil
}

func (h *ComprehensiveHook) OnGenerate(gen *CodeGenerator, protoName string, v interface{}) (next bool, err error) {
	// Skip generating code for SimpleType named "myType1" to test generation filtering
	if protoName == "SimpleType" {
		if st, ok := v.(*SimpleType); ok && st.Name == "myType1" {
			h.SkippedTypes = append(h.SkippedTypes, st.Name)
			return false, nil // Skip generating this type
		}
	}
	return true, nil
}

func (h *ComprehensiveHook) OnAddContent(gen *CodeGenerator, content *string) {
	// Modify generated content to add a custom marker comment
	typeName := gen.goTypeIdentifier(gen.TargetNamespace, "myType2")
	if strings.Contains(*content, "type "+typeName) {
		*content = strings.Replace(*content, "type "+typeName, "// HOOK_MODIFIED\ntype "+typeName, 1)
		h.ModifiedContent = true
	}
}

func TestHookSkipAndModify(t *testing.T) {
	hook := &ComprehensiveHook{
		SkippedTypes: make([]string, 0),
	}

	// Create temp directory for output
	tempDir, err := ioutil.TempDir(filepath.Join(testFixtureDir, "go"), "hook-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Run parser with hook
	inputDir := filepath.Join(testFixtureDir, "xsd")
	files, err := GetFileList(inputDir)
	require.NoError(t, err)

	for _, file := range files {
		if filepath.Ext(file) == ".xsd" {
			parser := NewParser(&Options{
				FilePath:            file,
				InputDir:            inputDir,
				OutputDir:           tempDir,
				Lang:                "Go",
				IncludeMap:          make(map[string]bool),
				LocalNameNSMap:      make(map[string]string),
				NSSchemaLocationMap: make(map[string]string),
				ParseFileList:       make(map[string]bool),
				ParseFileMap:        make(map[string][]interface{}),
				ProtoTree:           make([]interface{}, 0),
				Hook:                hook,
			})
			err = parser.Parse()
			assert.NoError(t, err)
		}
	}

	// Verify skipping worked - annotations in base64.xsd should have been skipped
	assert.Greater(t, hook.SkippedAnnotations, 0, "Hook should have skipped annotation elements")

	// Verify type filtering worked
	assert.Contains(t, hook.SkippedTypes, "myType1", "Hook should have skipped myType1 generation")

	// Read generated file and verify modifications
	generatedFile := filepath.Join(tempDir, "base64.xsd.go")
	content, err := ioutil.ReadFile(generatedFile)
	require.NoError(t, err)

	generatedCode := string(content)
	basePrefix := "Here"

	// Verify hook comment was added
	if hook.ModifiedContent {
		assert.Contains(t, generatedCode, "// HOOK_MODIFIED", "Generated code should contain hook modifications")
	}

	// Verify skipped type was not generated
	// MyType1 should not have its own type declaration (it's used as a field type but shouldn't have "type <Prefix>MyType1 ")
	assert.NotContains(t, generatedCode, "type "+basePrefix+"MyType1 ", "Skipped type should not have its own type declaration")

	// Verify it's still referenced in TopLevel (the field name, not the type declaration)
	assert.Contains(t, generatedCode, "MyType1         []string", "Field referencing the type should still exist")

	// Verify other types were still generated
	assert.Contains(t, generatedCode, "type "+basePrefix+"MyType2", "Non-skipped types should be generated")
}

// ErrorTestHook tests error propagation from hooks
type ErrorTestHook struct{}

func (h *ErrorTestHook) OnStartElement(opt *Options, ele xml.StartElement, protoTree []interface{}) (next bool, err error) {
	// Return error when encountering specific element
	if ele.Name.Local == "complexType" {
		for _, attr := range ele.Attr {
			if attr.Name.Local == "name" && attr.Value == "myType3" {
				return false, fmt.Errorf("intentional error for testing: forbidden type myType3")
			}
		}
	}
	return true, nil
}

func (h *ErrorTestHook) OnEndElement(opt *Options, ele xml.EndElement, protoTree []interface{}) (next bool, err error) {
	return true, nil
}

func (h *ErrorTestHook) OnCharData(opt *Options, ele string, protoTree []interface{}) (next bool, err error) {
	return true, nil
}

func (h *ErrorTestHook) OnGenerate(gen *CodeGenerator, protoName string, v interface{}) (next bool, err error) {
	return true, nil
}

func (h *ErrorTestHook) OnAddContent(gen *CodeGenerator, content *string) {
	// no-op
}

func TestHookErrorHandling(t *testing.T) {
	hook := &ErrorTestHook{}

	inputDir := filepath.Join(testFixtureDir, "xsd")
	file := filepath.Join(inputDir, "base64.xsd")

	tempDir, err := ioutil.TempDir(filepath.Join(testFixtureDir, "go"), "error-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	parser := NewParser(&Options{
		FilePath:            file,
		InputDir:            inputDir,
		OutputDir:           tempDir,
		Lang:                "Go",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		Hook:                hook,
	})

	err = parser.Parse()

	// Verify that the error from the hook was propagated
	assert.Error(t, err, "Hook error should be propagated")
	assert.Contains(t, err.Error(), "intentional error for testing", "Error should contain hook's error message")
	assert.Contains(t, err.Error(), "forbidden type myType3", "Error should identify the specific type")
}

// CharDataErrorHook tests error handling in OnCharData
type CharDataErrorHook struct {
	EncounteredCharData bool
}

func (h *CharDataErrorHook) OnStartElement(opt *Options, ele xml.StartElement, protoTree []interface{}) (next bool, err error) {
	return true, nil
}

func (h *CharDataErrorHook) OnEndElement(opt *Options, ele xml.EndElement, protoTree []interface{}) (next bool, err error) {
	return true, nil
}

func (h *CharDataErrorHook) OnCharData(opt *Options, ele string, protoTree []interface{}) (next bool, err error) {
	// Return error on non-empty character data
	trimmed := strings.TrimSpace(ele)
	if trimmed != "" {
		h.EncounteredCharData = true
		return false, fmt.Errorf("intentional OnCharData error: got data '%s'", trimmed)
	}
	return true, nil
}

func (h *CharDataErrorHook) OnGenerate(gen *CodeGenerator, protoName string, v interface{}) (next bool, err error) {
	return true, nil
}

func (h *CharDataErrorHook) OnAddContent(gen *CodeGenerator, content *string) {
	// no-op
}

func TestHookOnCharDataError(t *testing.T) {
	hook := &CharDataErrorHook{}

	inputDir := filepath.Join(testFixtureDir, "xsd")
	file := filepath.Join(inputDir, "base64.xsd")

	tempDir, err := ioutil.TempDir(filepath.Join(testFixtureDir, "go"), "chardata-error-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	parser := NewParser(&Options{
		FilePath:            file,
		InputDir:            inputDir,
		OutputDir:           tempDir,
		Lang:                "Go",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		Hook:                hook,
	})

	err = parser.Parse()

	// Verify that the error from OnCharData was propagated
	assert.Error(t, err, "OnCharData error should be propagated")
	assert.Contains(t, err.Error(), "intentional OnCharData error", "Error should contain OnCharData's error message")
	assert.True(t, hook.EncounteredCharData, "Hook should have encountered character data")
}

// CharDataSkipHook tests skipping behavior in OnCharData
type CharDataSkipHook struct {
	SkippedCharData int
	ProcessedCount  int
}

func (h *CharDataSkipHook) OnStartElement(opt *Options, ele xml.StartElement, protoTree []interface{}) (next bool, err error) {
	return true, nil
}

func (h *CharDataSkipHook) OnEndElement(opt *Options, ele xml.EndElement, protoTree []interface{}) (next bool, err error) {
	return true, nil
}

func (h *CharDataSkipHook) OnCharData(opt *Options, ele string, protoTree []interface{}) (next bool, err error) {
	h.ProcessedCount++
	// Skip processing for character data containing "appinfo"
	if strings.Contains(ele, "appinfo") {
		h.SkippedCharData++
		return false, nil // Skip further processing
	}
	return true, nil
}

func (h *CharDataSkipHook) OnGenerate(gen *CodeGenerator, protoName string, v interface{}) (next bool, err error) {
	return true, nil
}

func (h *CharDataSkipHook) OnAddContent(gen *CodeGenerator, content *string) {
	// no-op
}

func TestHookOnCharDataSkip(t *testing.T) {
	hook := &CharDataSkipHook{}

	inputDir := filepath.Join(testFixtureDir, "xsd")
	file := filepath.Join(inputDir, "base64.xsd")

	tempDir, err := ioutil.TempDir(filepath.Join(testFixtureDir, "go"), "chardata-skip-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	parser := NewParser(&Options{
		FilePath:            file,
		InputDir:            inputDir,
		OutputDir:           tempDir,
		Lang:                "Go",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		Hook:                hook,
	})

	err = parser.Parse()

	// Parsing should succeed even though we skipped some character data
	assert.NoError(t, err, "Parse should succeed when skipping character data")

	// Verify that the hook processed character data and skipped some
	assert.Greater(t, hook.ProcessedCount, 0, "Hook should have processed character data")
	assert.Greater(t, hook.SkippedCharData, 0, "Hook should have skipped some character data")
}

// OnAddContentTestHook tracks OnAddContent calls
type OnAddContentTestHook struct {
	OnAddContentCallCount int
}

func (h *OnAddContentTestHook) OnStartElement(opt *Options, ele xml.StartElement, protoTree []interface{}) (next bool, err error) {
	return true, nil
}

func (h *OnAddContentTestHook) OnEndElement(opt *Options, ele xml.EndElement, protoTree []interface{}) (next bool, err error) {
	return true, nil
}

func (h *OnAddContentTestHook) OnCharData(opt *Options, ele string, protoTree []interface{}) (next bool, err error) {
	return true, nil
}

func (h *OnAddContentTestHook) OnGenerate(gen *CodeGenerator, protoName string, v interface{}) (next bool, err error) {
	return true, nil
}

func (h *OnAddContentTestHook) OnAddContent(gen *CodeGenerator, content *string) {
	h.OnAddContentCallCount++
}

// TestOnAddContentHookNotNil tests all locations where: if gen.Hook != nil { gen.Hook.OnAddContent(gen, &output) }
// This covers the TRUE branch (gen.Hook != nil)
func TestOnAddContentHookNotNil(t *testing.T) {
	fieldNameCount = make(map[string]int)

	hook := &OnAddContentTestHook{}
	gen := &CodeGenerator{
		Lang:      "Go",
		StructAST: make(map[string]string),
		Hook:      hook, // NOT nil
	}

	tests := []struct {
		name     string
		testFunc func()
	}{
		{
			name: "GoSimpleType_List",
			testFunc: func() {
				gen.GoSimpleType(&SimpleType{Name: "ListType", Base: "string", List: true})
			},
		},
		{
			name: "GoSimpleType_Union",
			testFunc: func() {
				gen.GoSimpleType(&SimpleType{
					Name:        "UnionType",
					Union:       true,
					MemberTypes: map[string]string{"string": "string", "int": "int"},
				})
			},
		},
		{
			name: "GoSimpleType_Base",
			testFunc: func() {
				gen.GoSimpleType(&SimpleType{Name: "BaseType", Base: "string"})
			},
		},
		{
			name: "GoComplexType",
			testFunc: func() {
				gen.GoComplexType(&ComplexType{
					Name:     "ComplexType",
					Elements: []Element{{Name: "Field1", Type: "string"}},
				})
			},
		},
		{
			name: "GoGroup",
			testFunc: func() {
				gen.GoGroup(&Group{
					Name:     "GroupType",
					Elements: []Element{{Name: "Element1", Type: "string"}},
				})
			},
		},
		{
			name: "GoAttributeGroup",
			testFunc: func() {
				gen.GoAttributeGroup(&AttributeGroup{
					Name:       "AttrGroupType",
					Attributes: []Attribute{{Name: "Attr1", Type: "string"}},
				})
			},
		},
		{
			name: "GoElement",
			testFunc: func() {
				gen.GoElement(&Element{Name: "ElementType", Type: "string"})
			},
		},
		{
			name: "GoAttribute",
			testFunc: func() {
				gen.GoAttribute(&Attribute{Name: "AttributeType", Type: "string"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialCount := hook.OnAddContentCallCount
			tt.testFunc()
			assert.Greater(t, hook.OnAddContentCallCount, initialCount,
				"OnAddContent should be called when Hook is not nil for %s", tt.name)
		})
	}
}

// TestOnAddContentHookIsNil tests all locations where: if gen.Hook != nil { gen.Hook.OnAddContent(gen, &output) }
// This covers the FALSE branch (gen.Hook == nil)
func TestOnAddContentHookIsNil(t *testing.T) {
	fieldNameCount = make(map[string]int)

	gen := &CodeGenerator{
		Lang:      "Go",
		StructAST: make(map[string]string),
		Hook:      nil, // IS nil
	}

	tests := []struct {
		name     string
		testFunc func()
	}{
		{
			name: "GoSimpleType_List",
			testFunc: func() {
				gen.GoSimpleType(&SimpleType{Name: "ListType", Base: "string", List: true})
			},
		},
		{
			name: "GoSimpleType_Union",
			testFunc: func() {
				gen.GoSimpleType(&SimpleType{
					Name:        "UnionType",
					Union:       true,
					MemberTypes: map[string]string{"string": "string", "int": "int"},
				})
			},
		},
		{
			name: "GoSimpleType_Base",
			testFunc: func() {
				gen.GoSimpleType(&SimpleType{Name: "BaseType", Base: "string"})
			},
		},
		{
			name: "GoComplexType",
			testFunc: func() {
				gen.GoComplexType(&ComplexType{
					Name:     "ComplexType",
					Elements: []Element{{Name: "Field1", Type: "string"}},
				})
			},
		},
		{
			name: "GoGroup",
			testFunc: func() {
				gen.GoGroup(&Group{
					Name:     "GroupType",
					Elements: []Element{{Name: "Element1", Type: "string"}},
				})
			},
		},
		{
			name: "GoAttributeGroup",
			testFunc: func() {
				gen.GoAttributeGroup(&AttributeGroup{
					Name:       "AttrGroupType",
					Attributes: []Attribute{{Name: "Attr1", Type: "string"}},
				})
			},
		},
		{
			name: "GoElement",
			testFunc: func() {
				gen.GoElement(&Element{Name: "ElementType", Type: "string"})
			},
		},
		{
			name: "GoAttribute",
			testFunc: func() {
				gen.GoAttribute(&Attribute{Name: "AttributeType", Type: "string"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, tt.testFunc,
				"Should not panic when Hook is nil for %s", tt.name)
		})
	}
}

func TestParseGoPrefixesNamespacesInSinglePackage(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-namespace-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	rootSchema := `<?xml version="1.0" encoding="UTF-8"?>
<schema xmlns="http://www.w3.org/2001/XMLSchema"
	xmlns:root="http://example.com/root"
	xmlns:alpha="http://example.com/alpha"
	xmlns:beta="http://example.com/beta"
	targetNamespace="http://example.com/root"
	elementFormDefault="qualified">
	<import namespace="http://example.com/alpha" schemaLocation="alpha.xsd"/>
	<import namespace="http://example.com/beta" schemaLocation="beta.xsd"/>

	<complexType name="Envelope">
		<sequence>
			<element name="alphaValue" type="alpha:SharedType"/>
			<element name="betaValue" type="beta:SharedType"/>
		</sequence>
	</complexType>

	<element name="Envelope" type="root:Envelope"/>
</schema>`
	alphaSchema := `<?xml version="1.0" encoding="UTF-8"?>
<schema xmlns="http://www.w3.org/2001/XMLSchema"
	xmlns:alpha="http://example.com/alpha"
	targetNamespace="http://example.com/alpha"
	elementFormDefault="qualified">
	<complexType name="SharedType">
		<sequence>
			<element name="value" type="string"/>
		</sequence>
	</complexType>
</schema>`
	betaSchema := `<?xml version="1.0" encoding="UTF-8"?>
<schema xmlns="http://www.w3.org/2001/XMLSchema"
	xmlns:beta="http://example.com/beta"
	targetNamespace="http://example.com/beta"
	elementFormDefault="qualified">
	<complexType name="SharedType">
		<sequence>
			<element name="count" type="int"/>
		</sequence>
	</complexType>
</schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "root.xsd"), []byte(rootSchema), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "alpha.xsd"), []byte(alphaSchema), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "beta.xsd"), []byte(betaSchema), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "root.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	alphaPrefix := "Alpha"
	betaPrefix := "Beta"
	rootPrefix := "Root"

	rootGenerated, err := os.ReadFile(filepath.Join(outputDir, "root.xsd.go"))
	require.NoError(t, err)
	rootCode := string(rootGenerated)
	assert.NotContains(t, rootCode, "schema/")
	assert.Contains(t, rootCode, "package schema")
	assert.Contains(t, rootCode, fmt.Sprintf("type %sEnvelope struct", rootPrefix))
	assert.Contains(t, rootCode, fmt.Sprintf("*%sSharedType", alphaPrefix))
	assert.Contains(t, rootCode, fmt.Sprintf("*%sSharedType", betaPrefix))

	alphaGenerated, err := os.ReadFile(filepath.Join(outputDir, "alpha.xsd.go"))
	require.NoError(t, err)
	assert.Contains(t, string(alphaGenerated), "package schema")
	assert.Contains(t, string(alphaGenerated), fmt.Sprintf("type %sSharedType struct", alphaPrefix))

	betaGenerated, err := os.ReadFile(filepath.Join(outputDir, "beta.xsd.go"))
	require.NoError(t, err)
	assert.Contains(t, string(betaGenerated), "package schema")
	assert.Contains(t, string(betaGenerated), fmt.Sprintf("type %sSharedType struct", betaPrefix))

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoUsesReadablePrefixesAndAvoidsQNameXMLNameConflict(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-oadr-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:oadr="http://openadr.org/oadr-2.0b/2012/07"
	targetNamespace="http://openadr.org/oadr-2.0b/2012/07"
	elementFormDefault="qualified">
	<xs:element name="oadrPayload">
		<xs:complexType>
			<xs:sequence>
				<xs:element ref="oadr:oadrSignedObject"/>
			</xs:sequence>
		</xs:complexType>
	</xs:element>
	<xs:element name="oadrSignedObject">
		<xs:complexType>
			<xs:sequence>
				<xs:element name="payloadValue" type="xs:string"/>
			</xs:sequence>
		</xs:complexType>
	</xs:element>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "oadr.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "oadr.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "oadr.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "type OadrPayload struct")
	assert.Contains(t, code, "*OadrSignedObject `xml:\"http://openadr.org/oadr-2.0b/2012/07 oadrSignedObject\"`")
	assert.Contains(t, code, "type OadrSignedObject struct")
	assert.NotContains(t, code, "type Ns")
	assert.Contains(t, code, "XMLName      xml.Name `xml:\"http://openadr.org/oadr-2.0b/2012/07 oadrSignedObject\"`")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"testing"
)

func TestQNameMarshalUnmarshal(t *testing.T) {
		input := []byte("<oadr:oadrPayload xmlns:oadr=\"http://openadr.org/oadr-2.0b/2012/07\"><oadr:oadrSignedObject><oadr:payloadValue>x</oadr:payloadValue></oadr:oadrSignedObject></oadr:oadrPayload>")
		var payload OadrPayload
		if err := xml.Unmarshal(input, &payload); err != nil {
			t.Fatal(err)
		}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "oadr_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoAddsElementSuffixOnReadableNameCollision(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-collision-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:ei="http://example.com/ei"
	targetNamespace="http://example.com/ei"
	elementFormDefault="qualified">
	<xs:element name="optType" type="xs:string"/>
	<xs:complexType name="EiOptType">
		<xs:sequence>
			<xs:element name="value" type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "collision.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "collision.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "collision.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "type EiOptTypeElement string")
	assert.Contains(t, code, "type EiOptType struct")
}

func TestParseGoUsesNamedEnumTypesForElementValidation(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-enum-validation-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:ei="http://example.com/ei"
	targetNamespace="http://example.com/ei"
	elementFormDefault="qualified">
	<xs:simpleType name="EventStatusEnumeratedType">
		<xs:restriction base="xs:string">
			<xs:enumeration value="active"/>
			<xs:enumeration value="cancelled"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:element name="payload">
		<xs:complexType>
			<xs:sequence>
				<xs:element name="eventStatus" type="ei:EventStatusEnumeratedType"/>
			</xs:sequence>
		</xs:complexType>
	</xs:element>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "enum-validation.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "enum-validation.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "enum-validation.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "type EiEventStatusEnumeratedType string")
	assert.Contains(t, code, "EventStatus *EiEventStatusEnumeratedType")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestNamedEnumValidationDuringUnmarshal(t *testing.T) {
	input := []byte("<ei:payload xmlns:ei=\"http://example.com/ei\"><ei:eventStatus>invalid</ei:eventStatus></ei:payload>")
	var payload EiPayload
	err := xml.Unmarshal(input, &payload)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "EiEventStatusEnumeratedType") {
		t.Fatalf("expected enum validation error, got %v", err)
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "enum_validation_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoUsesNamedEnumTypesForComplexSimpleContentValidation(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-complex-enum-validation-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:ei="http://example.com/ei"
	targetNamespace="http://example.com/ei"
	elementFormDefault="qualified">
	<xs:simpleType name="EventStatusEnumeratedType">
		<xs:restriction base="xs:string">
			<xs:enumeration value="active"/>
			<xs:enumeration value="cancelled"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:complexType name="StatusWrapper">
		<xs:simpleContent>
			<xs:extension base="ei:EventStatusEnumeratedType">
				<xs:attribute name="source" type="xs:string"/>
			</xs:extension>
		</xs:simpleContent>
	</xs:complexType>
	<xs:element name="payload" type="ei:StatusWrapper"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "complex-enum-validation.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "complex-enum-validation.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "complex-enum-validation.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "type EiEventStatusEnumeratedType string")
	assert.Regexp(t, "Value\\s+EiEventStatusEnumeratedType\\s+`xml:\\\",chardata\\\"`", code)

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestNamedEnumValidationDuringComplexSimpleContentUnmarshal(t *testing.T) {
	input := []byte("<ei:payload xmlns:ei=\"http://example.com/ei\" source=\"system\">invalid</ei:payload>")
	var payload EiPayload
	err := xml.Unmarshal(input, &payload)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "EiEventStatusEnumeratedType") {
		t.Fatalf("expected enum validation error, got %v", err)
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "complex_enum_validation_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoResolvesImportedSubstitutionGroupTypes(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-substitution-group-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	mainSchema := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:oadr="http://example.com/oadr"
	xmlns:ei="http://example.com/ei"
	targetNamespace="http://example.com/oadr"
	elementFormDefault="qualified">
	<xs:import namespace="http://example.com/ei" schemaLocation="ei.xsd"/>
	<xs:element name="payload">
		<xs:complexType>
			<xs:sequence>
				<xs:element ref="ei:registrationID" minOccurs="0"/>
			</xs:sequence>
		</xs:complexType>
	</xs:element>
</xs:schema>`
	depSchema := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:ei="http://example.com/ei"
	targetNamespace="http://example.com/ei"
	elementFormDefault="qualified">
	<xs:element name="registrationID" substitutionGroup="ei:refID"/>
	<xs:element name="refID" substitutionGroup="ei:uid"/>
	<xs:element name="uid" type="ei:UidType" abstract="true"/>
	<xs:simpleType name="UidType">
		<xs:restriction base="xs:string"/>
	</xs:simpleType>
	<xs:complexType name="ContainerType">
		<xs:sequence>
			<xs:element ref="ei:uid" minOccurs="0"/>
		</xs:sequence>
	</xs:complexType>
	<xs:element name="container" type="ei:ContainerType"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "main.xsd"), []byte(mainSchema), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "ei.xsd"), []byte(depSchema), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "main.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	mainGenerated, err := os.ReadFile(filepath.Join(outputDir, "main.xsd.go"))
	require.NoError(t, err)
	mainCode := string(mainGenerated)
	assert.Contains(t, mainCode, "EiRegistrationID")
	assert.Contains(t, mainCode, "*string")
	assert.Contains(t, mainCode, "`xml:\"http://example.com/ei registrationID\"`")
	assert.NotContains(t, mainCode, "OadrRegistrationID")

	depGenerated, err := os.ReadFile(filepath.Join(outputDir, "ei.xsd.go"))
	require.NoError(t, err)
	depCode := string(depGenerated)
	assert.Contains(t, depCode, "type EiRegistrationID string")
	assert.Contains(t, depCode, "type EiUid interface {\n\tisEiUid()\n}")
	assert.Contains(t, depCode, "type EiRefID interface {\n\tisEiRefID()\n\tisEiUid()\n}")
	assert.Contains(t, depCode, "type EiRefIDElement string")
	assert.Contains(t, depCode, "func (*EiRefIDElement) isEiRefID() {}")
	assert.Contains(t, depCode, "func (*EiRefIDElement) isEiUid() {}")
	assert.Contains(t, depCode, "func (*EiRegistrationID) isEiRefID() {}")
	assert.Contains(t, depCode, "func (*EiRegistrationID) isEiUid() {}")
	assert.Contains(t, depCode, "case xml.Name{Space: \"http://example.com/ei\", Local: \"refID\"}:")
	assert.Contains(t, depCode, "case xml.Name{Space: \"http://example.com/ei\", Local: \"registrationID\"}:")
	assert.NotContains(t, depCode, "func (v EiUid) MarshalXML")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"strings"
	"testing"
)

var _ EiUid = (*EiRefIDElement)(nil)
var _ EiRefID = (*EiRefIDElement)(nil)
var _ EiUid = (*EiRegistrationID)(nil)
var _ EiRefID = (*EiRegistrationID)(nil)

func TestSubstitutionGroupIntermediateHeadIsConcreteChoice(t *testing.T) {
	var refContainer EiContainer
	if err := xml.Unmarshal([]byte("<container xmlns=\"http://example.com/ei\"><refID>r1</refID></container>"), &refContainer); err != nil {
		t.Fatal(err)
	}
	refID, ok := refContainer.EiUid.Get().(*EiRefIDElement)
	if !ok {
		t.Fatalf("expected *EiRefIDElement, got %T", refContainer.EiUid.Get())
	}
	if *refID != EiRefIDElement("r1") {
		t.Fatalf("unexpected refID: %q", *refID)
	}
	output, err := xml.Marshal(refContainer)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "<refID") || !strings.Contains(string(output), ">r1</refID>") {
		t.Fatalf("expected refID element, got %s", output)
	}

	var registrationContainer EiContainer
	if err := xml.Unmarshal([]byte("<container xmlns=\"http://example.com/ei\"><registrationID>reg1</registrationID></container>"), &registrationContainer); err != nil {
		t.Fatal(err)
	}
	registrationID, ok := registrationContainer.EiUid.Get().(*EiRegistrationID)
	if !ok {
		t.Fatalf("expected *EiRegistrationID, got %T", registrationContainer.EiUid.Get())
	}
	if *registrationID != EiRegistrationID("reg1") {
		t.Fatalf("unexpected registrationID: %q", *registrationID)
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "substitution_group_intermediate_runtime_test.go"), []byte(runtimeTest), 0o644))
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoSubstitutionGroupPolymorphism(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-substitution-group-polymorphism-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:tns="http://example.com/animals"
	targetNamespace="http://example.com/animals"
	elementFormDefault="qualified">
	<xs:complexType name="AnimalType">
		<xs:sequence>
			<xs:element name="name" type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
	<xs:complexType name="DogType">
		<xs:complexContent>
			<xs:extension base="tns:AnimalType">
				<xs:sequence>
					<xs:element name="barkVolume" type="xs:int"/>
				</xs:sequence>
			</xs:extension>
		</xs:complexContent>
	</xs:complexType>
	<xs:complexType name="CatType">
		<xs:complexContent>
			<xs:extension base="tns:AnimalType">
				<xs:sequence>
					<xs:element name="lives" type="xs:int"/>
				</xs:sequence>
			</xs:extension>
		</xs:complexContent>
	</xs:complexType>
	<xs:element name="animal" type="tns:AnimalType" abstract="true"/>
	<xs:element name="dog" type="tns:DogType" substitutionGroup="tns:animal"/>
	<xs:element name="cat" type="tns:CatType" substitutionGroup="tns:animal"/>
	<xs:complexType name="ZooType">
		<xs:sequence>
			<xs:element ref="tns:animal" maxOccurs="unbounded"/>
		</xs:sequence>
	</xs:complexType>
	<xs:element name="zoo" type="tns:ZooType"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "animals.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "animals.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "animals.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "type TnsAnimal interface")
	assert.Contains(t, code, "TnsAnimal TnsZooTypeTnsAnimalSubstitutionGroup")
	assert.Contains(t, code, "case xml.Name{Space: \"http://example.com/animals\", Local: \"dog\"}:")
	assert.Contains(t, code, "case xml.Name{Space: \"http://example.com/animals\", Local: \"cat\"}:")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestSubstitutionGroupUnmarshalDispatchesToConcreteMembers(t *testing.T) {
	input := []byte("<zoo xmlns=\"http://example.com/animals\"><dog><name>Fido</name><barkVolume>7</barkVolume></dog><cat><name>Mog</name><lives>9</lives></cat></zoo>")
	var zoo TnsZoo
	if err := xml.Unmarshal(input, &zoo); err != nil {
		t.Fatal(err)
	}
	animals := zoo.TnsAnimal.Values()
	if len(animals) != 2 {
		t.Fatalf("expected 2 animals, got %d", len(animals))
	}
	dog, ok := animals[0].(*TnsDog)
	if !ok {
		t.Fatalf("expected first animal to be *TnsDog, got %T", animals[0])
	}
	if dog.Name != "Fido" || dog.BarkVolume != 7 {
		t.Fatalf("unexpected dog: %#v", dog)
	}
	cat, ok := animals[1].(*TnsCat)
	if !ok {
		t.Fatalf("expected second animal to be *TnsCat, got %T", animals[1])
	}
	if cat.Name != "Mog" || cat.Lives != 9 {
		t.Fatalf("unexpected cat: %#v", cat)
	}
}

func TestSubstitutionGroupMarshalUsesConcreteMemberNames(t *testing.T) {
	zoo := TnsZoo{}
	zoo.TnsAnimal.Append(
		&TnsDog{TnsAnimalType: &TnsAnimalType{Name: "Fido"}, BarkVolume: 7},
		&TnsCat{TnsAnimalType: &TnsAnimalType{Name: "Mog"}, Lives: 9},
	)
	output, err := xml.Marshal(zoo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "<dog") || !strings.Contains(string(output), "<cat") {
		t.Fatalf("expected dog and cat elements, got: %s", output)
	}
	var decoded TnsZoo
	if err := xml.Unmarshal(output, &decoded); err != nil {
		t.Fatal(err)
	}
	animals := decoded.TnsAnimal.Values()
	if len(animals) != 2 {
		t.Fatalf("expected 2 round-tripped animals, got %d", len(animals))
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "substitution_group_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoNestedSubstitutionGroupPolymorphism(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-nested-substitution-group-polymorphism-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:emix="http://example.com/emix"
	targetNamespace="http://example.com/emix"
	elementFormDefault="qualified">
	<xs:complexType name="ItemBaseType">
		<xs:sequence>
			<xs:element name="id" type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
	<xs:complexType name="EnergyItemType" abstract="true">
		<xs:complexContent>
			<xs:extension base="emix:ItemBaseType"/>
		</xs:complexContent>
	</xs:complexType>
	<xs:complexType name="PowerEnergyItemType">
		<xs:complexContent>
			<xs:extension base="emix:EnergyItemType">
				<xs:sequence>
					<xs:element name="watts" type="xs:int"/>
				</xs:sequence>
			</xs:extension>
		</xs:complexContent>
	</xs:complexType>
	<xs:element name="itemBase" type="emix:ItemBaseType" abstract="true"/>
	<xs:element name="energyItem" type="emix:EnergyItemType" substitutionGroup="emix:itemBase"/>
	<xs:element name="powerEnergyItem" type="emix:PowerEnergyItemType" substitutionGroup="emix:energyItem"/>
	<xs:complexType name="EnvelopeType">
		<xs:sequence>
			<xs:element ref="emix:itemBase" maxOccurs="unbounded"/>
		</xs:sequence>
	</xs:complexType>
	<xs:element name="envelope" type="emix:EnvelopeType"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "emix.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "emix.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "emix.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "case xml.Name{Space: \"http://example.com/emix\", Local: \"powerEnergyItem\"}:")
	assert.NotContains(t, code, "type EmixEnergyItemElement")
	assert.NotContains(t, code, "var value EmixEnergyItemElement")
	assert.Contains(t, code, "func (*EmixPowerEnergyItem) isEmixItemBase() {}")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"testing"
)

func TestNestedSubstitutionGroupUnmarshalDispatchesToLeafMember(t *testing.T) {
	input := []byte("<envelope xmlns=\"http://example.com/emix\"><powerEnergyItem><id>a</id><watts>42</watts></powerEnergyItem></envelope>")
	var envelope EmixEnvelope
	if err := xml.Unmarshal(input, &envelope); err != nil {
		t.Fatal(err)
	}
	items := envelope.EmixItemBase.Values()
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	item, ok := items[0].(*EmixPowerEnergyItem)
	if !ok {
		t.Fatalf("expected *EmixPowerEnergyItem, got %T", items[0])
	}
	if item.Id != "a" || item.Watts != 42 {
		t.Fatalf("unexpected item: %#v", item)
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "nested_substitution_group_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoFlattenedBaseFieldsPreserveOwnerNamespace(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-flattened-base-namespace-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	mainSchema := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:oadr="http://example.com/oadr"
	xmlns:ei="http://example.com/ei"
	targetNamespace="http://example.com/oadr"
	elementFormDefault="qualified">
	<xs:import namespace="http://example.com/ei" schemaLocation="ei.xsd"/>
	<xs:complexType name="OptTypeType">
		<xs:sequence>
			<xs:element name="oadrValue" type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
	<xs:complexType name="DerivedType">
		<xs:complexContent>
			<xs:extension base="ei:BaseType">
				<xs:sequence>
					<xs:element name="derivedValue" type="xs:string"/>
				</xs:sequence>
			</xs:extension>
		</xs:complexContent>
	</xs:complexType>
	<xs:element name="derived" type="oadr:DerivedType"/>
</xs:schema>`
	depSchema := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:ei="http://example.com/ei"
	targetNamespace="http://example.com/ei"
	elementFormDefault="qualified">
	<xs:complexType name="OptTypeType">
		<xs:sequence>
			<xs:element name="eiValue" type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
	<xs:complexType name="OptReasonType">
		<xs:sequence>
			<xs:element name="reason" type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
	<xs:complexType name="BaseType">
		<xs:sequence>
			<xs:element name="optType" type="ei:OptTypeType"/>
			<xs:element name="optReason" type="ei:OptReasonType"/>
		</xs:sequence>
	</xs:complexType>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "oadr.xsd"), []byte(mainSchema), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "ei.xsd"), []byte(depSchema), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "oadr.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "oadr.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Regexp(t, `OptType\s+\*EiOptTypeType\s+`+"`"+`xml:"http://example.com/ei optType"`+"`", code)
	assert.Regexp(t, `OptReason\s+\*EiOptReasonType\s+`+"`"+`xml:"http://example.com/ei optReason"`+"`", code)
	assert.NotContains(t, code, "OptType *OadrOptTypeType `xml:\"http://example.com/ei optType\"`")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoImportedConcreteHeadUsesWrapperInterface(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-imported-concrete-head-wrapper-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	mainSchema := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:oadr="http://example.com/oadr"
	xmlns:emix="http://example.com/emix"
	targetNamespace="http://example.com/oadr"
	elementFormDefault="qualified">
	<xs:import namespace="http://example.com/emix" schemaLocation="emix.xsd"/>
	<xs:complexType name="FooType">
		<xs:sequence>
			<xs:element name="value" type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
	<xs:element name="foo" type="oadr:FooType" substitutionGroup="emix:itemBase"/>
	<xs:complexType name="ContainerType">
		<xs:sequence>
			<xs:element ref="emix:itemBase" minOccurs="0"/>
		</xs:sequence>
	</xs:complexType>
	<xs:element name="container" type="oadr:ContainerType"/>
</xs:schema>`
	depSchema := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:emix="http://example.com/emix"
	targetNamespace="http://example.com/emix"
	elementFormDefault="qualified">
	<xs:complexType name="ItemBaseType">
		<xs:sequence>
			<xs:element name="id" type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
	<xs:element name="itemBase" type="emix:ItemBaseType" abstract="true"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "oadr.xsd"), []byte(mainSchema), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "emix.xsd"), []byte(depSchema), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "oadr.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "oadr.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "type OadrContainerTypeEmixItemBaseSubstitutionGroupMember interface")
	assert.Contains(t, code, "Value OadrContainerTypeEmixItemBaseSubstitutionGroupMember")
	assert.NotContains(t, code, "Value EmixItemBase")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"testing"
)

func TestImportedConcreteHeadWrapperUsesLocalInterface(t *testing.T) {
	input := []byte("<container xmlns=\"http://example.com/oadr\"><foo><value>x</value></foo></container>")
	var container OadrContainer
	if err := xml.Unmarshal(input, &container); err != nil {
		t.Fatal(err)
	}
	foo, ok := container.EmixItemBase.Get().(*OadrFoo)
	if !ok {
		t.Fatalf("expected *OadrFoo, got %T", container.EmixItemBase.Get())
	}
	if foo.Value != "x" {
		t.Fatalf("unexpected value: %#v", foo)
	}
	output, err := xml.Marshal(container)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) == 0 {
		t.Fatal("expected marshal output")
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "imported_concrete_head_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoDirectoryResolvesReverseImportedSubstitutionGroupMembers(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-reverse-imported-substitution-group-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	streamSchema := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:strm="http://example.com/stream"
	targetNamespace="http://example.com/stream"
	elementFormDefault="qualified">
	<xs:complexType name="StreamPayloadBaseType" abstract="true"/>
	<xs:element name="streamPayloadBase" type="strm:StreamPayloadBaseType" abstract="true"/>
	</xs:schema>`
	eiSchema := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:ei="http://example.com/ei"
	xmlns:strm="http://example.com/stream"
	targetNamespace="http://example.com/ei"
	elementFormDefault="qualified">
	<xs:import namespace="http://example.com/stream" schemaLocation="stream.xsd"/>
	<xs:complexType name="IntervalType">
		<xs:sequence>
			<xs:element ref="strm:streamPayloadBase" maxOccurs="unbounded"/>
		</xs:sequence>
	</xs:complexType>
	<xs:element name="interval" type="ei:IntervalType"/>
</xs:schema>`
	oadrSchema := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:oadr="http://example.com/oadr"
	xmlns:strm="http://example.com/stream"
	targetNamespace="http://example.com/oadr"
	elementFormDefault="qualified">
	<xs:import namespace="http://example.com/stream" schemaLocation="stream.xsd"/>
	<xs:complexType name="ReportPayloadType">
		<xs:complexContent>
			<xs:extension base="strm:StreamPayloadBaseType">
				<xs:sequence>
					<xs:element name="rID" type="xs:string"/>
				</xs:sequence>
			</xs:extension>
		</xs:complexContent>
	</xs:complexType>
	<xs:element name="reportPayload" type="oadr:ReportPayloadType" substitutionGroup="strm:streamPayloadBase"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "stream.xsd"), []byte(streamSchema), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "ei.xsd"), []byte(eiSchema), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "oadr.xsd"), []byte(oadrSchema), 0o644))

	parser := NewParser(&Options{
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	eiGenerated, err := os.ReadFile(filepath.Join(outputDir, "ei.xsd.go"))
	require.NoError(t, err)
	eiCode := string(eiGenerated)
	assert.Contains(t, eiCode, "func (*OadrReportPayload) isEiIntervalTypeStrmStreamPayloadBaseSubstitutionGroupMember() {}")
	assert.Contains(t, eiCode, "case xml.Name{Space: \"http://example.com/oadr\", Local: \"reportPayload\"}:")
	assert.Contains(t, eiCode, "var value OadrReportPayload")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"testing"
)

func TestReverseImportedSubstitutionGroupMemberUnmarshals(t *testing.T) {
	input := []byte("<interval xmlns=\"http://example.com/ei\" xmlns:oadr=\"http://example.com/oadr\"><oadr:reportPayload><oadr:rID>rid-1</oadr:rID></oadr:reportPayload></interval>")
	var interval EiInterval
	if err := xml.Unmarshal(input, &interval); err != nil {
		t.Fatal(err)
	}
	members := interval.StrmStreamPayloadBase.Values()
	if len(members) != 1 {
		t.Fatalf("expected one stream payload, got %d", len(members))
	}
	payload, ok := members[0].(*OadrReportPayload)
	if !ok {
		t.Fatalf("expected *OadrReportPayload, got %T", members[0])
	}
	if payload.RID != "rid-1" {
		t.Fatalf("unexpected rID: %q", payload.RID)
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "reverse_imported_substitution_group_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoTopLevelListElementDoesNotForwardValidate(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-list-element-validate-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:gml="http://www.opengis.net/gml/3.2"
	targetNamespace="http://www.opengis.net/gml/3.2"
	elementFormDefault="qualified">
	<xs:simpleType name="doubleList">
		<xs:list itemType="xs:double"/>
	</xs:simpleType>
	<xs:element name="posList" type="gml:doubleList"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "gml.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "gml.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "gml.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "type GmlDoubleList []float64")
	assert.Contains(t, code, "type GmlPosList GmlDoubleList")
	assert.NotContains(t, code, "GmlDoubleList(v).Validate()")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoTopLevelSimpleElementUsesElementNameDuringMarshal(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-top-level-simple-element-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:ei="http://example.com/ei"
	targetNamespace="http://example.com/ei"
	elementFormDefault="qualified">
	<xs:simpleType name="EventStatusEnumeratedType">
		<xs:restriction base="xs:string">
			<xs:enumeration value="completed"/>
			<xs:enumeration value="cancelled"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:element name="eventStatus" type="ei:EventStatusEnumeratedType"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "event-status.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "event-status.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "event-status.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "type EiEventStatus EiEventStatusEnumeratedType")
	assert.Contains(t, code, "func (v EiEventStatus) MarshalXML")
	assert.Contains(t, code, "start.Name = xml.Name{Space: \"http://example.com/ei\", Local: \"eventStatus\"}")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"testing"
)

func TestTopLevelSimpleElementMarshalUsesElementName(t *testing.T) {
	statusValue := EiEventStatusEnumeratedType("completed")
	status := EiEventStatus(statusValue)
	output, err := xml.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "<eventStatus xmlns=\"http://example.com/ei\">completed</eventStatus>" {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestTopLevelSimpleElementUnmarshalUsesReferencedValidation(t *testing.T) {
	var status EiEventStatus
	err := xml.Unmarshal([]byte("<ei:eventStatus xmlns:ei=\"http://example.com/ei\">invalid</ei:eventStatus>"), &status)
	if err == nil {
		t.Fatal("expected validation error")
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "event_status_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoTopLevelComplexElementUsesElementNameDuringMarshal(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-top-level-complex-element-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:ei="http://example.com/ei"
	targetNamespace="http://example.com/ei"
	elementFormDefault="qualified">
	<xs:complexType name="PayloadType">
		<xs:sequence>
			<xs:element name="value" type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
	<xs:element name="payload" type="ei:PayloadType"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "payload.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "payload.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "payload.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "type EiPayload EiPayloadType")
	assert.Contains(t, code, "func (v EiPayload) MarshalXML")
	assert.Contains(t, code, "start.Name = xml.Name{Space: \"http://example.com/ei\", Local: \"payload\"}")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"testing"
)

func TestTopLevelComplexElementMarshalUsesElementName(t *testing.T) {
	payload := EiPayload{
		Value: "x",
	}
	output, err := xml.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "<payload xmlns=\"http://example.com/ei\"><value xmlns=\"http://example.com/ei\">x</value></payload>" {
		t.Fatalf("unexpected output: %s", output)
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "payload_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoBareElementDefaultsToAnyTypeWithoutRecursion(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-bare-anytype-element-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:tns="http://example.com/schema"
	targetNamespace="http://example.com/schema"
	elementFormDefault="qualified">
	<xs:element name="components" nillable="true"/>
	<xs:complexType name="WrapperType">
		<xs:sequence>
			<xs:element ref="tns:components" minOccurs="0"/>
		</xs:sequence>
	</xs:complexType>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "bare-anytype.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "bare-anytype.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "bare-anytype.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "type TnsComponents string")
	assert.NotContains(t, code, "type TnsComponents TnsComponents")
	assert.Contains(t, code, "TnsComponents *TnsComponents `xml:\"http://example.com/schema components\"`")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"testing"
)

func TestBareElementDefaultsToStringValue(t *testing.T) {
	value := TnsComponents("part")
	output, err := xml.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "<components xmlns=\"http://example.com/schema\">part</components>" {
		t.Fatalf("unexpected output: %s", output)
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "bare_anytype_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoUnionValidationCoversEnumAndPatternMembers(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-union-pattern-validation-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:tns="http://example.com/union"
	targetNamespace="http://example.com/union"
	elementFormDefault="qualified">
	<xs:simpleType name="ExtensionTokenType">
		<xs:restriction base="xs:string">
			<xs:pattern value="x-\S.*"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:simpleType name="SignalNameEnumeratedType">
		<xs:restriction base="xs:string">
			<xs:enumeration value="SIMPLE"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:simpleType name="SignalNameType">
		<xs:union memberTypes="tns:SignalNameEnumeratedType tns:ExtensionTokenType"/>
	</xs:simpleType>
	<xs:element name="signalName" type="tns:SignalNameType"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "union-pattern.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "union-pattern.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "union-pattern.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "regexp.MustCompile(`x-\\S.*`).MatchString(value)")
	assert.Contains(t, code, "if err := validateTnsSignalNameEnumeratedType(value); err == nil {")
	assert.Contains(t, code, "if err := validateTnsExtensionTokenType(value); err == nil {")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"testing"
)

func TestUnionValidationAcceptsEnumMember(t *testing.T) {
	var value TnsSignalName
	if err := xml.Unmarshal([]byte("<signalName>SIMPLE</signalName>"), &value); err != nil {
		t.Fatal(err)
	}
}

func TestUnionValidationAcceptsPatternMember(t *testing.T) {
	var value TnsSignalName
	if err := xml.Unmarshal([]byte("<signalName>x-custom</signalName>"), &value); err != nil {
		t.Fatal(err)
	}
}

func TestUnionValidationRejectsInvalidValue(t *testing.T) {
	var value TnsSignalName
	if err := xml.Unmarshal([]byte("<signalName>invalid</signalName>"), &value); err == nil {
		t.Fatal("expected validation error")
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "union_pattern_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoPatternPreservesBackslashes(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-pattern-backslash-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:tns="http://example.com/pattern"
	targetNamespace="http://example.com/pattern"
	elementFormDefault="qualified">
	<xs:simpleType name="DigitsOnlyType">
		<xs:restriction base="xs:string">
			<xs:pattern value="\d+"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:element name="digits" type="tns:DigitsOnlyType"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "pattern-backslash.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "pattern-backslash.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "pattern-backslash.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "regexp.MustCompile(`\\d+`).MatchString(value)")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"testing"
)

func TestPatternValidationAcceptsDigits(t *testing.T) {
	var value TnsDigits
	if err := xml.Unmarshal([]byte("<digits>123</digits>"), &value); err != nil {
		t.Fatal(err)
	}
}

func TestPatternValidationRejectsNonDigits(t *testing.T) {
	var value TnsDigits
	if err := xml.Unmarshal([]byte("<digits>abc</digits>"), &value); err == nil {
		t.Fatal("expected validation error")
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "pattern_backslash_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoGlobalAttributeInlineEnumCompilesWithoutStructMethods(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-attribute-inline-enum-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:tns="http://example.com/attr"
	targetNamespace="http://example.com/attr"
	elementFormDefault="qualified">
	<xs:attribute name="space" default="preserve">
		<xs:simpleType>
			<xs:restriction base="xs:NCName">
				<xs:enumeration value="default"/>
				<xs:enumeration value="preserve"/>
			</xs:restriction>
		</xs:simpleType>
	</xs:attribute>
	<xs:complexType name="ContainerType">
		<xs:attribute ref="tns:space" use="optional"/>
	</xs:complexType>
	<xs:element name="container" type="tns:ContainerType"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "attribute-inline-enum.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "attribute-inline-enum.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "attribute-inline-enum.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "type TnsSpaceValue string")
	assert.Contains(t, code, "type TnsSpace *TnsSpaceValue")
	assert.NotContains(t, code, "func (v TnsSpace) Validate() error")
	assert.NotContains(t, code, "func (v *TnsSpace) UnmarshalXML")
	assert.NotContains(t, code, "func validateTnsSpaceValue(value reflect.Value, allowValidator bool) error")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoStructValidationSkipsNilOptionalPointers(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-nil-validation-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:tns="http://example.com/nil"
	targetNamespace="http://example.com/nil"
	elementFormDefault="qualified">
	<xs:complexType name="ChildType">
		<xs:sequence>
			<xs:element name="value" type="xs:string" minOccurs="0"/>
		</xs:sequence>
	</xs:complexType>
	<xs:complexType name="ContainerType">
		<xs:sequence>
			<xs:element name="child" type="tns:ChildType" minOccurs="0"/>
		</xs:sequence>
	</xs:complexType>
	<xs:element name="root" type="tns:ContainerType"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "nil-validation.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "nil-validation.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"testing"
)

func TestStructValidationSkipsNilOptionalPointers(t *testing.T) {
	var root TnsContainerType
	if err := xml.Unmarshal([]byte("<tns:root xmlns:tns=\"http://example.com/nil\"></tns:root>"), &root); err != nil {
		t.Fatal(err)
	}
	if root.Child != nil {
		t.Fatalf("expected nil child, got %+v", root.Child)
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "nil_validation_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoStructValidationRejectsNilRequiredReferencePointers(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-required-ref-validation-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:tns="http://example.com/required-ref"
	targetNamespace="http://example.com/required-ref"
	elementFormDefault="qualified">
	<xs:element name="child" type="tns:ChildType"/>
	<xs:complexType name="ChildType">
		<xs:sequence>
			<xs:element name="value" type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
	<xs:complexType name="ContainerType">
		<xs:sequence>
			<xs:element ref="tns:child"/>
		</xs:sequence>
	</xs:complexType>
	<xs:element name="root" type="tns:ContainerType"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "required-ref-validation.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "required-ref-validation.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "required-ref-validation.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "if v.TnsChild == nil {")
	assert.Contains(t, code, "return fmt.Errorf(\"TnsContainerType.TnsChild is required\")")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestStructValidationRejectsNilRequiredReferencePointers(t *testing.T) {
	var root TnsContainerType
	err := xml.Unmarshal([]byte("<tns:root xmlns:tns=\"http://example.com/required-ref\"></tns:root>"), &root)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "TnsContainerType.TnsChild is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "required_ref_validation_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestParseGoStructValidationRejectsMissingRequiredSubstitutionGroup(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "xgen-required-substitution-group-validation-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	inputDir := filepath.Join(tempDir, "xsd")
	outputDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))

	schemaDoc := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:tns="http://example.com/required-substitution"
	targetNamespace="http://example.com/required-substitution"
	elementFormDefault="qualified">
	<xs:element name="animal" type="tns:AnimalType" abstract="true"/>
	<xs:element name="dog" type="tns:DogType" substitutionGroup="tns:animal"/>
	<xs:complexType name="AnimalType">
		<xs:sequence>
			<xs:element name="name" type="xs:string"/>
		</xs:sequence>
	</xs:complexType>
	<xs:complexType name="DogType">
		<xs:complexContent>
			<xs:extension base="tns:AnimalType">
				<xs:sequence>
					<xs:element name="barkVolume" type="xs:int"/>
				</xs:sequence>
			</xs:extension>
		</xs:complexContent>
	</xs:complexType>
	<xs:complexType name="ZooType">
		<xs:sequence>
			<xs:element ref="tns:animal"/>
		</xs:sequence>
	</xs:complexType>
	<xs:element name="zoo" type="tns:ZooType"/>
</xs:schema>`

	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "required-substitution-validation.xsd"), []byte(schemaDoc), 0o644))

	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "required-substitution-validation.xsd"),
		InputDir:            inputDir,
		OutputDir:           outputDir,
		Lang:                "Go",
		Package:             "schema",
		IncludeMap:          make(map[string]bool),
		LocalNameNSMap:      make(map[string]string),
		NSSchemaLocationMap: make(map[string]string),
		ParseFileList:       make(map[string]bool),
		ParseFileMap:        make(map[string][]interface{}),
		ProtoTree:           make([]interface{}, 0),
		RemoteSchema:        make(map[string][]byte),
	})
	require.NoError(t, parser.Parse())

	generated, err := os.ReadFile(filepath.Join(outputDir, "required-substitution-validation.xsd.go"))
	require.NoError(t, err)
	code := string(generated)
	assert.Contains(t, code, "if v.TnsAnimal.Value == nil {")
	assert.Contains(t, code, "return fmt.Errorf(\"TnsZooType.TnsAnimal is required\")")

	goMod := "module schema\n\ngo 1.22\n"
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0o644))
	runtimeTest := `package schema

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestStructValidationRejectsMissingRequiredSubstitutionGroup(t *testing.T) {
	var zoo TnsZooType
	err := xml.Unmarshal([]byte("<tns:zoo xmlns:tns=\"http://example.com/required-substitution\"></tns:zoo>"), &zoo)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "TnsZooType.TnsAnimal is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStructValidationAcceptsPresentRequiredSubstitutionGroup(t *testing.T) {
	var zoo TnsZooType
	input := []byte("<tns:zoo xmlns:tns=\"http://example.com/required-substitution\"><tns:dog><tns:name>Fido</tns:name><tns:barkVolume>7</tns:barkVolume></tns:dog></tns:zoo>")
	if err := xml.Unmarshal(input, &zoo); err != nil {
		t.Fatal(err)
	}
	if zoo.TnsAnimal.Value == nil {
		t.Fatal("expected required substitution group value")
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "required_substitution_validation_runtime_test.go"), []byte(runtimeTest), 0o644))

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}
