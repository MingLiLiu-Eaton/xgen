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
	"strings"
)

// CodeGenerator holds code generator overrides and runtime data that are used
// when generate code from proto tree.
type CodeGenerator struct {
	Lang              string
	File              string
	Field             string
	Package           string
	ImportFmt         bool // For Go language
	ImportTime        bool // For Go language
	ImportEncodingXML bool // For Go language
	TargetNamespace   string
	NamespacePrefix   map[string]string
	ReferencedNames   map[string]bool
	LocalNameNSMap    map[string]string
	ParseFileMap      map[string][]interface{}
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
	if gen.ImportFmt {
		importLines = append(importLines, "\t\"fmt\"")
	}
	if gen.ImportTime {
		importLines = append(importLines, "\t\"time\"")
	}
	if gen.ImportEncodingXML {
		importLines = append(importLines, "\t\"encoding/xml\"")
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

func (gen *CodeGenerator) goTypeNamespace(name string) string {
	if prefix := getNSPrefix(name); prefix != "" {
		if namespace := gen.LocalNameNSMap[prefix]; namespace != "" {
			return namespace
		}
	}
	return gen.TargetNamespace
}

func (gen *CodeGenerator) goNamespacePrefix(namespace string) string {
	prefix := normalizeNamespacePrefixCandidate(gen.NamespacePrefix[namespace])
	if prefix == "" {
		prefix = normalizeNamespacePrefixCandidate(namespaceTypeLabel(namespace))
	}
	return genGoTypeName(prefix)
}

func (gen *CodeGenerator) goTypeIdentifier(namespace, name string) string {
	if _, ok := goBuildinType[name]; ok {
		return name
	}
	fieldType := genGoTypeName(name)
	if fieldType == "" {
		return ""
	}
	if prefix := gen.goNamespacePrefix(namespace); prefix != "" && !strings.HasPrefix(strings.ToLower(fieldType), strings.ToLower(prefix)) {
		return prefix + fieldType
	}
	return fieldType
}

func (gen *CodeGenerator) goDeclarationName(name string, unique bool) string {
	fieldName := gen.goTypeIdentifier(gen.TargetNamespace, name)
	if fieldName == "" {
		fieldName = genGoTypeName(name)
	}
	if unique {
		fieldNameCount[fieldName]++
		if count := fieldNameCount[fieldName]; count != 1 {
			fieldName = fmt.Sprintf("%s%d", fieldName, count)
		}
	}
	return fieldName
}

func (gen *CodeGenerator) goElementDeclarationName(name string) string {
	fieldName := gen.goTypeIdentifier(gen.TargetNamespace, name)
	if fieldName == "" {
		fieldName = genGoTypeName(name)
	}
	if gen.hasLocalNamedTypeConflict(fieldName, name) {
		fieldName += "Element"
	}
	fieldNameCount[fieldName]++
	if count := fieldNameCount[fieldName]; count != 1 {
		fieldName = fmt.Sprintf("%s%d", fieldName, count)
	}
	return fieldName
}

func (gen *CodeGenerator) hasLocalNamedTypeConflict(goName, originalName string) bool {
	for _, ele := range gen.ProtoTree {
		switch v := ele.(type) {
		case *SimpleType:
			if v.Name != originalName && gen.goTypeIdentifier(gen.TargetNamespace, v.Name) == goName {
				return true
			}
		case *ComplexType:
			if v.Name != originalName && gen.goTypeIdentifier(gen.TargetNamespace, v.Name) == goName {
				return true
			}
		case *Group:
			if v.Name != originalName && gen.goTypeIdentifier(gen.TargetNamespace, v.Name) == goName {
				return true
			}
		case *AttributeGroup:
			if v.Name != originalName && gen.goTypeIdentifier(gen.TargetNamespace, v.Name) == goName {
				return true
			}
		}
	}
	return false
}

func shouldAddGoXMLName(name string, anonymous bool) bool {
	return anonymous || strings.IndexFunc(name, splitter) >= 0
}

func (gen *CodeGenerator) goXMLName(name string, anonymous bool) string {
	if !anonymous {
		return name
	}
	prefix := normalizeNamespacePrefixCandidate(gen.NamespacePrefix[gen.TargetNamespace])
	if prefix == "" {
		return name
	}
	qualifiedName := fmt.Sprintf("%s:%s", prefix, name)
	if gen.ReferencedNames[qualifiedName] {
		return qualifiedName
	}
	if gen.hasQualifiedElementReference(qualifiedName) {
		return qualifiedName
	}
	return name
}

func (gen *CodeGenerator) hasQualifiedElementReference(name string) bool {
	if hasQualifiedElementReferenceInTree(name, gen.ProtoTree) {
		return true
	}
	for _, tree := range gen.ParseFileMap {
		if hasQualifiedElementReferenceInTree(name, tree) {
			return true
		}
	}
	return false
}

func hasQualifiedElementReferenceInTree(name string, protoTree []interface{}) bool {
	for _, ele := range protoTree {
		switch v := ele.(type) {
		case *ComplexType:
			for _, element := range v.Elements {
				if element.Name == name {
					return true
				}
			}
		case *Group:
			for _, element := range v.Elements {
				if element.Name == name {
					return true
				}
			}
		}
	}
	return false
}

func (gen *CodeGenerator) goReferenceType(name string) string {
	return gen.goTypeIdentifier(gen.goTypeNamespace(name), name)
}

func (gen *CodeGenerator) goFieldType(name string) string {
	if _, ok := goBuildinType[name]; ok {
		return name
	}
	fieldType := gen.goReferenceType(name)
	if fieldType != "" {
		return "*" + fieldType
	}
	return "interface{}"
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
			fieldName := gen.goDeclarationName(v.Name, true)

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
			fieldName := gen.goDeclarationName(v.Name, true)
			if shouldAddGoXMLName(v.Name, v.Anonymous) {
				gen.ImportEncodingXML = true
				content += fmt.Sprintf("\tXMLName\txml.Name\t`xml:\"%s\"`\n", gen.goXMLName(v.Name, v.Anonymous))
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
		baseType := getBasefromSimpleType(v.Base, gen.ProtoTree)
		content := fmt.Sprintf(" %s\n", gen.goFieldType(baseType))
		gen.StructAST[v.Name] = content
		fieldName := gen.goDeclarationName(v.Name, true)

		output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
		if gen.Hook != nil {
			gen.Hook.OnAddContent(gen, &output)
		}
		gen.Field += output
		gen.Field += gen.goSimpleTypeValidationMethods(fieldName, baseType, v.Restriction)
	}
}

// GoComplexType generates code for complex type XML schema in Go language
// syntax.
func (gen *CodeGenerator) GoComplexType(v *ComplexType) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		content := " struct {\n"
		fieldName := gen.goDeclarationName(v.Name, true)
		if shouldAddGoXMLName(v.Name, v.Anonymous) {
			gen.ImportEncodingXML = true
			content += fmt.Sprintf("\tXMLName\txml.Name\t`xml:\"%s\"`\n", gen.goXMLName(v.Name, v.Anonymous))
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
			if helperName := gen.goEnsureInlineSimpleType(fieldName, genGoFieldName(attribute.Name, false)+"Attr", attribute.InlineSimpleType); helperName != "" {
				fieldType = gen.goFieldType(helperName)
			}
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
			if helperName := gen.goEnsureInlineSimpleType(fieldName, genGoFieldName(element.Name, false), element.InlineSimpleType); helperName != "" {
				fieldType = gen.goFieldType(helperName)
			}

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

func goSupportsSimpleTypeValidation(baseType string) bool {
	switch baseType {
	case "string", "bool", "float32", "float64", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return true
	default:
		return false
	}
}

func goSimpleTypeTextExpr(baseType, valueExpr string) string {
	if baseType == "string" {
		return fmt.Sprintf("string(%s)", valueExpr)
	}
	return fmt.Sprintf("fmt.Sprint(%s(%s))", baseType, valueExpr)
}

func (gen *CodeGenerator) goSimpleTypeValidationMethods(typeName, baseType string, restriction Restriction) string {
	if len(restriction.Enum) == 0 || !goSupportsSimpleTypeValidation(baseType) {
		return ""
	}
	gen.ImportFmt = true
	validatorName := "validate" + typeName
	caseValues := make([]string, 0, len(restriction.Enum))
	for _, enum := range restriction.Enum {
		caseValues = append(caseValues, fmt.Sprintf("%q", enum))
	}
	errorFormat := fmt.Sprintf("%s must be one of [%s], got %%q", typeName, strings.Join(restriction.Enum, ", "))
	textExpr := goSimpleTypeTextExpr(baseType, "v")
	unmarshalAssignment := fmt.Sprintf("\t*v = %s(value)\n", typeName)
	if baseType != "string" {
		unmarshalAssignment = fmt.Sprintf("\tvar parsed %s\n\tif _, err := fmt.Sscan(value, &parsed); err != nil {\n\t\treturn err\n\t}\n\t*v = %s(parsed)\n", baseType, typeName)
	}
	return fmt.Sprintf(`
func %s(value string) error {
	switch value {
	case %s:
		return nil
	default:
		return fmt.Errorf(%q, value)
	}
}

func (v %s) Validate() error {
	return %s(%s)
}

func (v %s) MarshalText() ([]byte, error) {
	value := %s
	if err := %s(value); err != nil {
		return nil, err
	}
	return []byte(value), nil
}

func (v *%s) UnmarshalText(text []byte) error {
	value := string(text)
	if err := %s(value); err != nil {
		return err
	}
%s	return nil
}
`, validatorName, strings.Join(caseValues, ", "), errorFormat, typeName, validatorName, textExpr, typeName, textExpr, validatorName, typeName, validatorName, unmarshalAssignment)
}

func (gen *CodeGenerator) goEnsureInlineSimpleType(ownerTypeName, fieldName string, simpleType *SimpleType) string {
	if simpleType == nil {
		return ""
	}
	baseType := getBasefromSimpleType(simpleType.Base, gen.ProtoTree)
	if len(simpleType.Restriction.Enum) == 0 || !goSupportsSimpleTypeValidation(baseType) {
		return ""
	}
	helperName := ownerTypeName + fieldName
	if _, ok := gen.StructAST[helperName]; ok {
		return helperName
	}
	inlineType := *simpleType
	inlineType.Name = helperName
	inlineType.Anonymous = false
	gen.GoSimpleType(&inlineType)
	return helperName
}

// GoGroup generates code for group XML schema in Go language syntax.
func (gen *CodeGenerator) GoGroup(v *Group) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		content := " struct {\n"
		fieldName := gen.goDeclarationName(v.Name, true)
		if shouldAddGoXMLName(v.Name, false) {
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
		fieldName := gen.goDeclarationName(v.Name, true)
		if shouldAddGoXMLName(v.Name, false) {
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
		fieldName := gen.goElementDeclarationName(v.Name)
		fieldType := gen.goFieldType(getBasefromSimpleType(v.Type, gen.ProtoTree))
		if helperName := gen.goEnsureInlineSimpleType(fieldName, "Value", v.InlineSimpleType); helperName != "" {
			fieldType = gen.goFieldType(helperName)
		}
		var plural string
		if v.Plural {
			plural = "[]"
		}
		content := fmt.Sprintf("\t%s%s\n", plural, fieldType)
		gen.StructAST[v.Name] = content

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
		fieldName := gen.goDeclarationName(v.Name, true)
		fieldType := gen.goFieldType(getBasefromSimpleType(v.Type, gen.ProtoTree))
		if helperName := gen.goEnsureInlineSimpleType(fieldName, "Value", v.InlineSimpleType); helperName != "" {
			fieldType = gen.goFieldType(helperName)
		}
		var plural string
		if v.Plural {
			plural = "[]"
		}
		content := fmt.Sprintf("\t%s%s\n", plural, fieldType)
		gen.StructAST[v.Name] = content

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
