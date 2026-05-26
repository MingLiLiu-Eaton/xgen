package xgen

import (
	"encoding/xml"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	schema "github.com/xuri/xgen/test/go"
)

// TestGeneratedGo runs through test cases to validate Go generated structs. Each test case
// requires a xml fixture file to unmarshal into the receiving struct. Validate first validates
// that the file can be unmarshaled as the receiving struct and then remarshals the content
// to make sure the marshaling is symmetrical
func TestGeneratedGo(t *testing.T) {
	testCases := []struct {
		// xmlFileName is the path to the xml fixture file to unmarshal into the receiving struct
		xmlFileName string
		// receivingStruct is a pointer to the struct to unmarshal the xml file content into. It should match
		// the type of the top level element present in that file
		receivingStruct interface{}
	}{
		{
			xmlFileName:     "base64.xml",
			receivingStruct: &schema.HereTopLevel{},
		},
		{
			xmlFileName:     "union.xml",
			receivingStruct: &schema.HereUnionTop{},
		},
		{
			xmlFileName:     "union-member-validation.xml",
			receivingStruct: &schema.MemberValidationTop{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.xmlFileName, func(t *testing.T) {
			fullPath := filepath.Join("xmlFixtures", tc.xmlFileName)

			input, err := ioutil.ReadFile(fullPath)
			require.NoError(t, err)

			err = xml.Unmarshal(input, tc.receivingStruct)
			require.NoError(t, err)

			// Validate that decoding resulted in a non-zero value
			assert.NotEmpty(t, tc.receivingStruct)

			remarshaled, err := xml.MarshalIndent(tc.receivingStruct, "", "    ")
			require.NoError(t, err)

			roundTripped := reflect.New(reflect.TypeOf(tc.receivingStruct).Elem()).Interface()
			err = xml.Unmarshal(remarshaled, roundTripped)
			require.NoError(t, err)

			normalizeXMLNameSpaces(tc.receivingStruct)
			normalizeXMLNameSpaces(roundTripped)
			assert.Equal(t, tc.receivingStruct, roundTripped)
		})
	}
}

func normalizeXMLNameSpaces(v interface{}) {
	normalizeReflectXMLNameSpaces(reflect.ValueOf(v))
}

func normalizeReflectXMLNameSpaces(v reflect.Value) {
	if !v.IsValid() {
		return
	}
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		normalizeReflectXMLNameSpaces(v.Elem())
		return
	}

	if v.Kind() == reflect.Struct {
		if v.Type() == reflect.TypeOf(xml.Name{}) {
			name := v.Addr().Interface().(*xml.Name)
			name.Space = ""
			return
		}
		for i := range v.NumField() {
			field := v.Field(i)
			if field.CanAddr() || field.Kind() == reflect.Pointer || field.Kind() == reflect.Interface {
				normalizeReflectXMLNameSpaces(field)
			}
		}
		return
	}

	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		for i := range v.Len() {
			normalizeReflectXMLNameSpaces(v.Index(i))
		}
	}
}

func TestGeneratedGoRejectsInvalidInlineEnumDuringUnmarshal(t *testing.T) {
	input := []byte(`<here:TopLevel xmlns:here="http://example.org/" cost="1.25" LastUpdated="2021-09-14T12:04:09.69" code="not found" identifier="10">
    <nested origin="internet">Destination-Host</nested>
</here:TopLevel>`)
	var actual schema.HereTopLevel
	err := xml.Unmarshal(input, &actual)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HereMyType6CodeAttr")
}

func TestGeneratedGoRejectsInvalidInlineEnumDuringMarshal(t *testing.T) {
	code := schema.HereMyType6CodeAttr("not found")
	value := &schema.HereTopLevel{
		XMLName:         xml.Name{Local: "TopLevel"},
		LastUpdatedAttr: "2021-09-14T12:04:09.69",
		HereMyType6: &schema.HereMyType6{
			CodeAttr: &code,
		},
	}
	_, err := xml.Marshal(value)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HereMyType6CodeAttr")
}

func TestGeneratedGoRejectsInvalidUnionDuringUnmarshal(t *testing.T) {
	input := []byte(`<here:UnionTop xmlns:here="http://example.org/union" attr="maybe">
    <value>true</value>
</here:UnionTop>`)
	var actual schema.HereUnionTop
	err := xml.Unmarshal(input, &actual)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HereUnionValue")
}

func TestGeneratedGoRejectsInvalidUnionDuringMarshal(t *testing.T) {
	attr := schema.HereUnionValue("maybe")
	value := schema.HereUnionValue("true")
	actual := &schema.HereUnionTop{
		XMLName:  xml.Name{Local: "UnionTop"},
		AttrAttr: &attr,
		Value:    &value,
	}
	_, err := xml.Marshal(actual)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HereUnionValue")
}

func TestGeneratedGoRejectsInvalidUnionMemberValidationDuringUnmarshal(t *testing.T) {
	input := []byte(`<here:ValidationTop xmlns:here="http://example.org/validation" attr="blue">
    <value>true</value>
</here:ValidationTop>`)
	var actual schema.MemberValidationTop
	err := xml.Unmarshal(input, &actual)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MemberUnionValue")
}

func TestGeneratedGoRejectsInvalidUnionMemberValidationDuringMarshal(t *testing.T) {
	attr := schema.MemberUnionValue("blue")
	value := schema.MemberUnionValue("green")
	actual := &schema.MemberValidationTop{
		XMLName:  xml.Name{Local: "ValidationTop"},
		AttrAttr: &attr,
		Value:    &value,
	}
	_, err := xml.Marshal(actual)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MemberUnionValue")
}

func TestGenGoAddsValidationForRestrictedSimpleType(t *testing.T) {
	tempDir := t.TempDir()
	gen := &CodeGenerator{
		File:            filepath.Join(tempDir, "enum"),
		Package:         "schema",
		TargetNamespace: "http://example.org/",
		NamespacePrefix: map[string]string{
			"http://example.org/": "here",
		},
		ReferencedNames: map[string]bool{},
		LocalNameNSMap:  map[string]string{},
		ParseFileMap:    map[string][]interface{}{},
		ProtoTree: []interface{}{
			&SimpleType{
				Name: "status",
				Base: "string",
				Restriction: Restriction{
					Enum: []string{"open", "closed"},
				},
			},
		},
		StructAST: map[string]string{},
	}

	require.NoError(t, gen.GenGo())
	output, err := ioutil.ReadFile(filepath.Join(tempDir, "enum.go"))
	require.NoError(t, err)

	typeName := gen.goTypeIdentifier(gen.TargetNamespace, "status")
	source := string(output)
	assert.Contains(t, source, "type "+typeName+" string")
	assert.Contains(t, source, "func (v "+typeName+") Validate() error")
	assert.Contains(t, source, "func (v "+typeName+") MarshalText() ([]byte, error)")
	assert.Contains(t, source, "func (v *"+typeName+") UnmarshalText(text []byte) error")
}

func TestGenGoAddsTextMarshalingForUnionSimpleType(t *testing.T) {
	tempDir := t.TempDir()
	gen := &CodeGenerator{
		File:            filepath.Join(tempDir, "union"),
		Package:         "schema",
		TargetNamespace: "http://example.org/",
		NamespacePrefix: map[string]string{
			"http://example.org/": "here",
		},
		ReferencedNames: map[string]bool{},
		LocalNameNSMap:  map[string]string{},
		ParseFileMap:    map[string][]interface{}{},
		ProtoTree: []interface{}{
			&SimpleType{
				Name:        "unionValue",
				Union:       true,
				MemberTypes: map[string]string{"boolean": "bool", "int": "int"},
			},
		},
		StructAST: map[string]string{},
	}

	require.NoError(t, gen.GenGo())
	output, err := ioutil.ReadFile(filepath.Join(tempDir, "union.go"))
	require.NoError(t, err)

	typeName := gen.goTypeIdentifier(gen.TargetNamespace, "unionValue")
	source := string(output)
	assert.Contains(t, source, "type "+typeName+" string")
	assert.Contains(t, source, "func (v "+typeName+") Validate() error")
	assert.Contains(t, source, "func (v "+typeName+") MarshalText() ([]byte, error)")
	assert.Contains(t, source, "func (v *"+typeName+") UnmarshalText(text []byte) error")
}

func TestGenGoUnionReusesNamedMemberValidation(t *testing.T) {
	tempDir := t.TempDir()
	gen := &CodeGenerator{
		File:            filepath.Join(tempDir, "union-member"),
		Package:         "schema",
		TargetNamespace: "http://example.org/",
		NamespacePrefix: map[string]string{
			"http://example.org/": "here",
		},
		ReferencedNames: map[string]bool{},
		LocalNameNSMap:  map[string]string{},
		ParseFileMap:    map[string][]interface{}{},
		ProtoTree: []interface{}{
			&SimpleType{
				Name: "color",
				Base: "string",
				Restriction: Restriction{
					Enum: []string{"red", "green"},
				},
			},
			&SimpleType{
				Name:        "unionValue",
				Union:       true,
				MemberTypes: map[string]string{"boolean": "bool", "color": "string"},
			},
		},
		StructAST: map[string]string{},
	}

	require.NoError(t, gen.GenGo())
	output, err := ioutil.ReadFile(filepath.Join(tempDir, "union-member.go"))
	require.NoError(t, err)

	source := string(output)
	assert.Contains(t, source, "if err := validateHereColor(value); err == nil {")
}

func TestGenGoAddsValidationForPatternRestrictedSimpleType(t *testing.T) {
	tempDir := t.TempDir()
	gen := &CodeGenerator{
		File:            filepath.Join(tempDir, "pattern"),
		Package:         "schema",
		TargetNamespace: "http://example.org/",
		NamespacePrefix: map[string]string{
			"http://example.org/": "here",
		},
		ReferencedNames: map[string]bool{},
		LocalNameNSMap:  map[string]string{},
		ParseFileMap:    map[string][]interface{}{},
		ProtoTree: []interface{}{
			&SimpleType{
				Name: "extensionToken",
				Base: "string",
				Restriction: Restriction{
					Pattern: regexp.MustCompile(`x-\S.*`),
				},
			},
		},
		StructAST: map[string]string{},
	}

	require.NoError(t, gen.GenGo())
	output, err := ioutil.ReadFile(filepath.Join(tempDir, "pattern.go"))
	require.NoError(t, err)

	typeName := gen.goTypeIdentifier(gen.TargetNamespace, "extensionToken")
	source := string(output)
	assert.Contains(t, source, "import (\n\t\"fmt\"\n\t\"regexp\"\n)")
	assert.Contains(t, source, "type "+typeName+" string")
	assert.Contains(t, source, "regexp.MustCompile(`x-\\S.*`).MatchString(value)")
	assert.Contains(t, source, "func (v "+typeName+") Validate() error")
}

func TestGenGoPreservesPatternBackslashes(t *testing.T) {
	tempDir := t.TempDir()
	gen := &CodeGenerator{
		File:            filepath.Join(tempDir, "pattern-digits"),
		Package:         "schema",
		TargetNamespace: "http://example.org/",
		NamespacePrefix: map[string]string{
			"http://example.org/": "here",
		},
		ReferencedNames: map[string]bool{},
		LocalNameNSMap:  map[string]string{},
		ParseFileMap:    map[string][]interface{}{},
		ProtoTree: []interface{}{
			&SimpleType{
				Name: "digitsOnly",
				Base: "string",
				Restriction: Restriction{
					Pattern: regexp.MustCompile(`\d+`),
				},
			},
		},
		StructAST: map[string]string{},
	}

	require.NoError(t, gen.GenGo())
	output, err := ioutil.ReadFile(filepath.Join(tempDir, "pattern-digits.go"))
	require.NoError(t, err)

	assert.Contains(t, string(output), "regexp.MustCompile(`\\d+`).MatchString(value)")
}

func TestGenGoUnionReusesPatternMemberValidation(t *testing.T) {
	tempDir := t.TempDir()
	gen := &CodeGenerator{
		File:            filepath.Join(tempDir, "union-pattern"),
		Package:         "schema",
		TargetNamespace: "http://example.org/",
		NamespacePrefix: map[string]string{
			"http://example.org/": "here",
		},
		ReferencedNames: map[string]bool{},
		LocalNameNSMap:  map[string]string{},
		ParseFileMap:    map[string][]interface{}{},
		ProtoTree: []interface{}{
			&SimpleType{
				Name: "extensionToken",
				Base: "string",
				Restriction: Restriction{
					Pattern: regexp.MustCompile(`x-\S.*`),
				},
			},
			&SimpleType{
				Name: "signalName",
				Base: "string",
				Restriction: Restriction{
					Enum: []string{"SIMPLE"},
				},
			},
			&SimpleType{
				Name:        "unionValue",
				Union:       true,
				MemberTypes: map[string]string{"signalName": "string", "extensionToken": "string"},
			},
		},
		StructAST: map[string]string{},
	}

	require.NoError(t, gen.GenGo())
	output, err := ioutil.ReadFile(filepath.Join(tempDir, "union-pattern.go"))
	require.NoError(t, err)

	source := string(output)
	assert.Contains(t, source, "if err := validateHereSignalName(value); err == nil {")
	assert.Contains(t, source, "if err := validateHereExtensionToken(value); err == nil {")
}

func TestParseGoUnionFixture(t *testing.T) {
	tempDir := t.TempDir()
	inputDir := "testdata"
	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "union.xsd"),
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

	actualGenerated, err := ioutil.ReadFile(filepath.Join(tempDir, "union.xsd.go"))
	require.NoError(t, err)
	assert.NotEmpty(t, actualGenerated)
}

func TestParseGoUnionMemberValidationFixture(t *testing.T) {
	tempDir := t.TempDir()
	inputDir := "testdata"
	parser := NewParser(&Options{
		FilePath:            filepath.Join(inputDir, "union-member-validation.xsd"),
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

	actualGenerated, err := ioutil.ReadFile(filepath.Join(tempDir, "union-member-validation.xsd.go"))
	require.NoError(t, err)
	assert.NotEmpty(t, actualGenerated)
}

func TestToTitle(t *testing.T) {
	test := func(expected, actual string) {
		assert.Equal(t, expected, ToTitle(actual))
	}

	test("", "")
	test("A", "a")
	test("Ab", "ab")
	test("A b", "a b")
	test("Ab cd", "ab cd")

	// Test Сyrillic (`привет мир` → `hello world`)
	test("Привет", "привет")
	test("Привет мир", "привет мир")
}

func TestCodeGeneratorFileWithExtension(t *testing.T) {
	testCases := []struct {
		description string
		filename    string
		extension   string
		expected    string
	}{
		{
			description: "filename without extension and extension without period should add extension",
			filename:    "foo",
			extension:   "java",
			expected:    "foo.java",
		},
		{
			description: "filename without extension and extension with period should add extension",
			filename:    "foo",
			extension:   ".java",
			expected:    "foo.java",
		},
		{
			description: "filename with extension already should not add extension",
			filename:    "foo.java",
			extension:   ".java",
			expected:    "foo.java",
		},
		{
			description: "filename with different extension should add extension",
			filename:    "foo.bar",
			extension:   ".java",
			expected:    "foo.bar.java",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			gen := CodeGenerator{
				File: tc.filename,
			}
			actual := gen.FileWithExtension(tc.extension)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
