// Copyright 2020 - 2024 The xgen Authors. All rights reserved. Use of this
// source code is governed by a BSD-style license that can be found in the
// LICENSE file.
//
// Package xgen written in pure Go providing a set of functions that allow you
// to parse XSD (XML schema files). This library needs Go version 1.10 or
// later.

package xgen

import (
	"fmt"
	"go/format"
	"os"
	"reflect"
	"sort"
	"strings"
)

// CodeGenerator holds code generator overrides and runtime data that are used
// when generate code from proto tree.
type CodeGenerator struct {
	Lang              string
	File              string
	Field             string
	Package           string
	RootImportPath    string
	ImportTime        bool // For Go language
	ImportEncodingXML bool // For Go language
	TargetNamespace   string
	LocalNameNSMap    map[string]string
	GoImports         map[string]string
	ProtoTree         []interface{}
	StructAST         map[string]string
	Hook              Hook
}

var goBuildinType = map[string]bool{
	"xml.Name":      true,
	"byte":          true,
	"[]byte":        true,
	"bool":          true,
	"[]bool":        true,
	"complex64":     true,
	"complex128":    true,
	"float32":       true,
	"float64":       true,
	"int":           true,
	"int8":          true,
	"int16":         true,
	"int32":         true,
	"int64":         true,
	"interface":     true,
	"[]interface{}": true,
	"string":        true,
	"[]string":      true,
	"time.Time":     true,
	"uint":          true,
	"uint8":         true,
	"uint16":        true,
	"uint32":        true,
	"uint64":        true,
}

// GenGo generate Go programming language source code for XML schema
// definition files.
func (gen *CodeGenerator) GenGo() error {
	err := error(nil)
	fieldNameCount = make(map[string]int)
	gen.GoImports = make(map[string]string)
	for _, ele := range gen.ProtoTree {
		if ele == nil {
			continue
		}

		next := true
		protoName := reflect.TypeOf(ele).String()[6:]
		if gen.Hook != nil {
			next, err = gen.Hook.OnGenerate(gen, protoName, ele)
			if err != nil {
				return err
			}

			// skip to next element (in tree)
			if !next {
				continue
			}
		}

		funcName := fmt.Sprintf("Go%s", protoName)
		callFuncByName(gen, funcName, []reflect.Value{reflect.ValueOf(ele)})
	}
	f, err := os.Create(gen.FileWithExtension(".go"))
	if err != nil {
		return err
	}
	defer f.Close()
	var importLines []string
	if gen.ImportTime {
		importLines = append(importLines, "\t\"time\"")
	}
	if gen.ImportEncodingXML {
		importLines = append(importLines, "\t\"encoding/xml\"")
	}
	importPaths := make([]string, 0, len(gen.GoImports))
	for importPath := range gen.GoImports {
		importPaths = append(importPaths, importPath)
	}
	sort.Strings(importPaths)
	for _, importPath := range importPaths {
		importLines = append(importLines, fmt.Sprintf("\t%s %q", gen.GoImports[importPath], importPath))
	}
	importPackage := ""
	if len(importLines) > 0 {
		importPackage = fmt.Sprintf("import (\n%s\n)", strings.Join(importLines, "\n"))
	}
	packageName := gen.Package
	if packageName == "" {
		packageName = goPackageName("")
	}
	source, err := format.Source([]byte(fmt.Sprintf("%s\n\npackage %s\n%s%s", copyright, packageName, importPackage, gen.Field)))
	if err != nil {
		f.WriteString(fmt.Sprintf("package %s\n%s%s", packageName, importPackage, gen.Field))
		return err
	}
	f.Write(source)
	return err
}

func splitter(r rune) bool {
	return strings.ContainsRune(":.-_", r)
}

func genGoFieldName(name string, unique bool) (fieldName string) {
	for _, str := range strings.FieldsFunc(name, splitter) {
		fieldName += MakeFirstUpperCase(str)
	}

	if unique {
		fieldNameCount[fieldName]++
		if count := fieldNameCount[fieldName]; count != 1 {
			fieldName = fmt.Sprintf("%s%d", fieldName, count)
		}
	}
	return
}

func genGoTypeName(name string) string {
	name = trimNSPrefix(name)
	if _, ok := goBuildinType[name]; ok {
		return name
	}
	var fieldType string
	for _, str := range strings.FieldsFunc(name, splitter) {
		fieldType += MakeFirstUpperCase(str)
	}
	return fieldType
}

func (gen *CodeGenerator) goFieldType(name string) string {
	if _, ok := goBuildinType[name]; ok {
		return name
	}
	if prefix := getNSPrefix(name); prefix != "" {
		namespace := gen.LocalNameNSMap[prefix]
		if namespace != "" && namespace != gen.TargetNamespace {
			alias := gen.goImportAlias(namespace)
			fieldType := genGoTypeName(name)
			if fieldType != "" {
				return "*" + alias + "." + fieldType
			}
			return "interface{}"
		}
		name = trimNSPrefix(name)
	}
	fieldType := genGoTypeName(name)
	if fieldType != "" {
		return "*" + fieldType
	}
	return "interface{}"
}

func (gen *CodeGenerator) goImportAlias(namespace string) string {
	if namespace == "" || namespace == gen.TargetNamespace {
		return ""
	}
	alias := goNamespacePackageName(gen.RootImportPath, namespace)
	gen.GoImports[goImportPathForNamespace(gen.RootImportPath, namespace)] = alias
	return alias
}

// GoSimpleType generates code for simple type XML schema in Go language
// syntax.
func (gen *CodeGenerator) GoSimpleType(v *SimpleType) {
	if v.List {
		if _, ok := gen.StructAST[v.Name]; !ok {
			fieldType := gen.goFieldType(getBasefromSimpleType(v.Base, gen.ProtoTree))
			if fieldType == "time.Time" {
				gen.ImportTime = true
			}
			content := fmt.Sprintf(" []%s\n", strings.TrimPrefix(fieldType, "*"))
			gen.StructAST[v.Name] = content
			fieldName := genGoFieldName(v.Name, true)

			output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
			if gen.Hook != nil {
				gen.Hook.OnAddContent(gen, &output)
			}
			gen.Field += output
			return
		}
	}
	if v.Union && len(v.MemberTypes) > 0 {
		if _, ok := gen.StructAST[v.Name]; !ok {
			content := " struct {\n"
			fieldName := genGoFieldName(v.Name, true)
			if fieldName != v.Name {
				gen.ImportEncodingXML = true
				content += fmt.Sprintf("\tXMLName\txml.Name\t`xml:\"%s\"`\n", v.Name)
			}
			for _, member := range toSortedPairs(v.MemberTypes) {
				memberName := member.key
				memberType := member.value

				if memberType == "" { // fix order issue
					memberType = getBasefromSimpleType(memberName, gen.ProtoTree)
				}
				content += fmt.Sprintf("\t%s\t%s\n", genGoFieldName(memberName, false), gen.goFieldType(memberType))
			}
			content += "}\n"
			gen.StructAST[v.Name] = content

			output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
			if gen.Hook != nil {
				gen.Hook.OnAddContent(gen, &output)
			}
			gen.Field += output
		}
		return
	}
	if _, ok := gen.StructAST[v.Name]; !ok {
		content := fmt.Sprintf(" %s\n", gen.goFieldType(getBasefromSimpleType(v.Base, gen.ProtoTree)))
		gen.StructAST[v.Name] = content
		fieldName := genGoFieldName(v.Name, true)

		output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
		if gen.Hook != nil {
			gen.Hook.OnAddContent(gen, &output)
		}
		gen.Field += output
	}
}

// GoComplexType generates code for complex type XML schema in Go language
// syntax.
func (gen *CodeGenerator) GoComplexType(v *ComplexType) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		content := " struct {\n"
		fieldName := genGoFieldName(v.Name, true)
		if fieldName != v.Name {
			gen.ImportEncodingXML = true
			content += fmt.Sprintf("\tXMLName\txml.Name\t`xml:\"%s\"`\n", v.Name)
		}
		for _, attrGroup := range v.AttributeGroup {
			fieldType := getBasefromSimpleType(attrGroup.Ref, gen.ProtoTree)
			if fieldType == "time.Time" {
				gen.ImportTime = true
			}
			content += fmt.Sprintf("\t%s\t%s\n", genGoFieldName(attrGroup.Name, false), gen.goFieldType(fieldType))
		}

		for _, attribute := range v.Attributes {
			fieldType := gen.goFieldType(getBasefromSimpleType(attribute.Type, gen.ProtoTree))
			var optional string
			if attribute.Optional {
				if !strings.HasPrefix(fieldType, `*`) {
					fieldType = "*" + fieldType
				} else {
					optional = `,omitempty`
				}
			}
			if fieldType == "time.Time" {
				gen.ImportTime = true
			}
			content += fmt.Sprintf("\t%sAttr\t%s\t`xml:\"%s,attr%s\"`\n", genGoFieldName(attribute.Name, false), fieldType, attribute.Name, optional)
		}
		for _, group := range v.Groups {
			fieldType := gen.goFieldType(getBasefromSimpleType(group.Ref, gen.ProtoTree))
			if group.Plural {
				fieldType = "[]" + fieldType
			}
			content += fmt.Sprintf("\t%s\t%s\n", genGoFieldName(group.Name, false), fieldType)
		}

		for _, element := range v.Elements {
			fieldType := gen.goFieldType(getBasefromSimpleType(element.Type, gen.ProtoTree))

			if element.Plural {
				fieldType = "[]" + fieldType
			}
			var optional string
			if element.Optional {
				if !element.Plural && !strings.HasPrefix(fieldType, `*`) {
					fieldType = "*" + fieldType
				}
			}
			if fieldType == "time.Time" {
				gen.ImportTime = true
			}
			content += fmt.Sprintf("\t%s\t%s\t`xml:\"%s%s\"`\n", genGoFieldName(element.Name, false), fieldType, element.Name, optional)
		}
		if len(v.Base) > 0 {
			// If the type is a built-in type, generate a Value field as chardata.
			// If it's not built-in one, embed the base type in the struct for the child type
			// to effectively inherit all of the base type's fields
			if isGoBuiltInType(v.Base) {
				content += fmt.Sprintf("\tValue\t%s\t`xml:\",chardata\"`\n", gen.goFieldType(v.Base))
			} else {
				content += fmt.Sprintf("\t%s\n", gen.goFieldType(v.Base))
			}
		}
		content += "}\n"
		gen.StructAST[v.Name] = content

		output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
		if gen.Hook != nil {
			gen.Hook.OnAddContent(gen, &output)
		}
		gen.Field += output
	}
}

func isGoBuiltInType(typeName string) bool {
	_, builtIn := goBuildinType[typeName]
	return builtIn
}

// GoGroup generates code for group XML schema in Go language syntax.
func (gen *CodeGenerator) GoGroup(v *Group) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		content := " struct {\n"
		fieldName := genGoFieldName(v.Name, true)
		if fieldName != v.Name {
			gen.ImportEncodingXML = true
			content += fmt.Sprintf("\tXMLName\txml.Name\t`xml:\"%s\"`\n", v.Name)
		}
		for _, element := range v.Elements {
			var plural string
			if element.Plural {
				plural = "[]"
			}
			content += fmt.Sprintf("\t%s\t%s%s\n", genGoFieldName(element.Name, false), plural, gen.goFieldType(getBasefromSimpleType(element.Type, gen.ProtoTree)))
		}

		for _, group := range v.Groups {
			var plural string
			if group.Plural {
				plural = "[]"
			}
			content += fmt.Sprintf("\t%s\t%s%s\n", genGoFieldName(group.Name, false), plural, gen.goFieldType(getBasefromSimpleType(group.Ref, gen.ProtoTree)))
		}

		content += "}\n"
		gen.StructAST[v.Name] = content

		output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
		if gen.Hook != nil {
			gen.Hook.OnAddContent(gen, &output)
		}
		gen.Field += output
	}
}

// GoAttributeGroup generates code for attribute group XML schema in Go language
// syntax.
func (gen *CodeGenerator) GoAttributeGroup(v *AttributeGroup) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		content := " struct {\n"
		fieldName := genGoFieldName(v.Name, true)
		if fieldName != v.Name {
			gen.ImportEncodingXML = true
			content += fmt.Sprintf("\tXMLName\txml.Name\t`xml:\"%s\"`\n", v.Name)
		}
		for _, attribute := range v.Attributes {
			var optional string
			if attribute.Optional {
				optional = `,omitempty`
			}
			content += fmt.Sprintf("\t%sAttr\t%s\t`xml:\"%s,attr%s\"`\n", genGoFieldName(attribute.Name, false), gen.goFieldType(getBasefromSimpleType(attribute.Type, gen.ProtoTree)), attribute.Name, optional)
		}
		content += "}\n"
		gen.StructAST[v.Name] = content

		output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
		if gen.Hook != nil {
			gen.Hook.OnAddContent(gen, &output)
		}
		gen.Field += output
	}
}

// GoElement generates code for element XML schema in Go language syntax.
func (gen *CodeGenerator) GoElement(v *Element) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		var plural string
		if v.Plural {
			plural = "[]"
		}
		content := fmt.Sprintf("\t%s%s\n", plural, gen.goFieldType(getBasefromSimpleType(v.Type, gen.ProtoTree)))
		gen.StructAST[v.Name] = content
		fieldName := genGoFieldName(v.Name, false)

		output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
		if gen.Hook != nil {
			gen.Hook.OnAddContent(gen, &output)
		}
		gen.Field += output
	}
}

// GoAttribute generates code for attribute XML schema in Go language syntax.
func (gen *CodeGenerator) GoAttribute(v *Attribute) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		var plural string
		if v.Plural {
			plural = "[]"
		}
		content := fmt.Sprintf("\t%s%s\n", plural, gen.goFieldType(getBasefromSimpleType(v.Type, gen.ProtoTree)))
		gen.StructAST[v.Name] = content
		fieldName := genGoFieldName(v.Name, true)

		output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
		if gen.Hook != nil {
			gen.Hook.OnAddContent(gen, &output)
		}
		gen.Field += output
	}
}

func (gen *CodeGenerator) FileWithExtension(extension string) string {
	if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}
	if strings.HasSuffix(gen.File, extension) {
		return gen.File
	}
	return gen.File + extension
}
