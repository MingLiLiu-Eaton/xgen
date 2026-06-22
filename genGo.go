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
	"strconv"
	"strings"
)

// CodeGenerator holds code generator overrides and runtime data that are used
// when generate code from proto tree.
type CodeGenerator struct {
	Lang                           string
	File                           string
	Field                          string
	Package                        string
	ImportFmt                      bool // For Go language
	ImportReflect                  bool // For Go language
	ImportRegexp                   bool // For Go language
	ImportStrconv                  bool // For Go language
	ImportTime                     bool // For Go language
	ImportEncodingXML              bool // For Go language
	TargetNamespace                string
	NamespacePrefix                map[string]string
	ReferencedNames                map[string]bool
	LocalNameNSMap                 map[string]string
	ParseFileMap                   map[string][]interface{}
	ProtoTree                      []interface{}
	StructAST                      map[string]string
	Hook                           Hook
	ValidationHelperEmitted        bool
	SubstitutionGroupHelperEmitted bool
}

type goComplexElementField struct {
	FieldName           string
	FieldType           string
	Plural              bool
	Element             Element
	SubstitutionMembers []*Element
}

type goComplexStructField struct {
	FieldName string
	FieldType string
	XMLTag    string
	HasXMLTag bool
}

type goComplexFlattenedBase struct {
	TypeName string
	Fields   []goComplexStructField
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
	if gen.ImportReflect {
		importLines = append(importLines, "\t\"reflect\"")
	}
	if gen.ImportRegexp {
		importLines = append(importLines, "\t\"regexp\"")
	}
	if gen.ImportStrconv {
		importLines = append(importLines, "\t\"strconv\"")
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
	fieldName := gen.goElementTypeNameFor(gen.TargetNamespace, name)
	fieldNameCount[fieldName]++
	if count := fieldNameCount[fieldName]; count != 1 {
		fieldName = fmt.Sprintf("%s%d", fieldName, count)
	}
	return fieldName
}

func (gen *CodeGenerator) goElementTypeNameFor(namespace, name string) string {
	fieldName := gen.goTypeIdentifier(namespace, name)
	if fieldName == "" {
		fieldName = genGoTypeName(name)
	}
	if gen.hasLocalNamedTypeConflict(fieldName, name) {
		fieldName += "Element"
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

func (gen *CodeGenerator) goXMLTag(namespace, name string) string {
	localName := trimNSPrefix(name)
	if namespace == "" {
		return localName
	}
	return fmt.Sprintf("%s %s", namespace, localName)
}

func (gen *CodeGenerator) goXMLTagWithOptions(namespace, name string, options ...string) string {
	tag := gen.goXMLTag(namespace, name)
	if len(options) == 0 {
		return tag
	}
	return tag + strings.Join(options, "")
}

func (gen *CodeGenerator) goXMLName(name, namespace string, anonymous bool) string {
	if namespace == "" && (anonymous || getNSPrefix(name) != "") {
		namespace = gen.goTypeNamespace(name)
	}
	return gen.goXMLTag(namespace, name)
}

func (gen *CodeGenerator) goXMLStartNameLiteral(namespace, name string) string {
	localName := trimNSPrefix(name)
	if namespace == "" {
		return fmt.Sprintf("xml.Name{Local: %q}", localName)
	}
	return fmt.Sprintf("xml.Name{Space: %q, Local: %q}", namespace, localName)
}

func (gen *CodeGenerator) goElementXMLNameLiteral(element *Element) string {
	return gen.goXMLStartNameLiteral(element.Namespace, element.Name)
}

func (gen *CodeGenerator) goAllElements() []*Element {
	elements := make([]*Element, 0)
	for _, tree := range append([][]interface{}{gen.ProtoTree}, gen.goParsedTrees()...) {
		for _, ele := range tree {
			if v, ok := ele.(*Element); ok {
				elements = append(elements, v)
			}
		}
	}
	return elements
}

func (gen *CodeGenerator) goParsedTrees() [][]interface{} {
	paths := make([]string, 0, len(gen.ParseFileMap))
	for path := range gen.ParseFileMap {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	values := make([][]interface{}, 0, len(paths))
	for _, path := range paths {
		values = append(values, gen.ParseFileMap[path])
	}
	return values
}

func (gen *CodeGenerator) goElementNamespace(element *Element) string {
	if element.Namespace != "" {
		return element.Namespace
	}
	if namespace := gen.goTypeNamespace(element.Name); namespace != "" {
		return namespace
	}
	return gen.TargetNamespace
}

func (gen *CodeGenerator) goSubstitutionGroupNamespace(member *Element) string {
	if prefix := getNSPrefix(member.SubstitutionGroup); prefix != "" {
		if namespace := gen.LocalNameNSMap[prefix]; namespace != "" {
			return namespace
		}
	}
	return gen.goElementNamespace(member)
}

func (gen *CodeGenerator) goFindElement(namespace, name string) *Element {
	localName := trimNSPrefix(name)
	for _, element := range gen.goAllElements() {
		if element.Name == localName && gen.goElementNamespace(element) == namespace {
			return element
		}
	}
	return nil
}

func (gen *CodeGenerator) goSubstitutionGroupMembers(head *Element) []*Element {
	headName := trimNSPrefix(head.Name)
	headNamespace := gen.goElementNamespace(head)
	members := make([]*Element, 0)
	seen := make(map[string]bool)
	for _, element := range gen.goAllElements() {
		if element.SubstitutionGroup == "" {
			continue
		}
		if trimNSPrefix(element.SubstitutionGroup) != headName || gen.goSubstitutionGroupNamespace(element) != headNamespace {
			continue
		}
		key := gen.goElementNamespace(element) + "\x00" + element.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		members = append(members, element)
	}
	sort.Slice(members, func(i, j int) bool {
		left := gen.goElementNamespace(members[i]) + "\x00" + members[i].Name
		right := gen.goElementNamespace(members[j]) + "\x00" + members[j].Name
		return left < right
	})
	return members
}

func (gen *CodeGenerator) goConcreteSubstitutionGroupMembers(head *Element) []*Element {
	concrete := make([]*Element, 0)
	seen := make(map[string]bool)
	var collect func(*Element)
	collect = func(current *Element) {
		for _, member := range gen.goSubstitutionGroupMembers(current) {
			memberKey := gen.goElementNamespace(member) + "\x00" + member.Name
			if seen[memberKey] {
				continue
			}
			seen[memberKey] = true
			children := gen.goSubstitutionGroupMembers(member)
			if len(children) == 0 {
				concrete = append(concrete, member)
				continue
			}
			collect(member)
		}
	}
	collect(head)
	sort.Slice(concrete, func(i, j int) bool {
		left := gen.goElementNamespace(concrete[i]) + "\x00" + concrete[i].Name
		right := gen.goElementNamespace(concrete[j]) + "\x00" + concrete[j].Name
		return left < right
	})
	return concrete
}

func (gen *CodeGenerator) goSubstitutionGroupHeads(member *Element) []*Element {
	heads := make([]*Element, 0)
	seen := make(map[string]bool)
	current := member
	for current != nil && current.SubstitutionGroup != "" {
		headName := trimNSPrefix(current.SubstitutionGroup)
		headNamespace := gen.goSubstitutionGroupNamespace(current)
		key := headNamespace + "\x00" + headName
		if seen[key] {
			break
		}
		seen[key] = true
		head := gen.goFindElement(headNamespace, headName)
		if head == nil {
			head = &Element{Name: headName, Namespace: headNamespace}
		}
		heads = append(heads, head)
		current = head
	}
	return heads
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

func (gen *CodeGenerator) goResolvedBaseType(name string) string {
	if baseType := getBasefromSimpleType(name, gen.ProtoTree); baseType != name {
		return baseType
	}
	for _, tree := range gen.ParseFileMap {
		if baseType := getBasefromSimpleType(name, tree); baseType != name {
			return baseType
		}
	}
	return name
}

func (gen *CodeGenerator) goValueFieldType(name string) string {
	if simpleType := gen.goFindSimpleType(name); gen.goSimpleTypeHasValidation(simpleType) {
		return gen.goFieldType(name)
	}
	return gen.goFieldType(gen.goResolvedBaseType(name))
}

func (gen *CodeGenerator) goValueFieldTypeInNamespace(name, namespace string) string {
	if getNSPrefix(name) != "" || isGoBuiltInType(name) {
		return gen.goValueFieldType(name)
	}
	if simpleType := gen.goFindSimpleType(name); gen.goSimpleTypeHasValidation(simpleType) {
		return gen.goFieldTypeInNamespace(name, namespace)
	}
	baseType := gen.goResolvedBaseType(name)
	if getNSPrefix(baseType) != "" || isGoBuiltInType(baseType) {
		return gen.goFieldType(baseType)
	}
	return gen.goFieldTypeInNamespace(baseType, namespace)
}

func (gen *CodeGenerator) goFieldTypeInNamespace(name, namespace string) string {
	if _, ok := goBuildinType[name]; ok {
		return name
	}
	fieldType := gen.goTypeIdentifier(namespace, name)
	if fieldType != "" {
		return "*" + fieldType
	}
	return "interface{}"
}

func (gen *CodeGenerator) goSimpleContentFieldType(name string) string {
	if simpleType := gen.goFindSimpleType(name); gen.goSimpleTypeHasValidation(simpleType) {
		return gen.goReferenceType(name)
	}
	return gen.goResolvedBaseType(name)
}

func findComplexTypeInTree(name string, protoTree []interface{}) *ComplexType {
	localName := trimNSPrefix(name)
	for _, ele := range protoTree {
		if v, ok := ele.(*ComplexType); ok && v.Name == localName {
			return v
		}
	}
	return nil
}

func (gen *CodeGenerator) goFindComplexType(name string) *ComplexType {
	if complexType := findComplexTypeInTree(name, gen.ProtoTree); complexType != nil {
		return complexType
	}
	for _, tree := range gen.ParseFileMap {
		if complexType := findComplexTypeInTree(name, tree); complexType != nil {
			return complexType
		}
	}
	return nil
}

func (gen *CodeGenerator) goComplexTypeOwnFields(v *ComplexType, typeName string) []goComplexStructField {
	namespace := v.Namespace
	if namespace == "" {
		namespace = gen.goTypeNamespace(v.Name)
	}
	fields := make([]goComplexStructField, 0)
	for _, attribute := range v.Attributes {
		fieldType := gen.goValueFieldTypeInNamespace(attribute.Type, namespace)
		if helperName := gen.goEnsureInlineSimpleType(typeName, genGoFieldName(attribute.Name, false)+"Attr", attribute.InlineSimpleType); helperName != "" {
			fieldType = gen.goFieldType(helperName)
		}
		if attribute.Optional && !strings.HasPrefix(fieldType, "*") {
			fieldType = "*" + fieldType
		}
		fieldName := genGoFieldName(attribute.Name, false) + "Attr"
		fields = append(fields, goComplexStructField{
			FieldName: fieldName,
			FieldType: fieldType,
			XMLTag:    gen.goXMLTagWithOptions(attribute.Namespace, attribute.Name, ",attr"),
			HasXMLTag: true,
		})
	}
	for _, element := range v.Elements {
		fieldName := genGoFieldName(element.Name, false)
		fieldType := gen.goValueFieldTypeInNamespace(element.Type, namespace)
		if helperName := gen.goEnsureInlineSimpleType(typeName, fieldName, element.InlineSimpleType); helperName != "" {
			fieldType = gen.goFieldType(helperName)
		}
		if len(gen.goConcreteSubstitutionGroupMembers(&element)) > 0 {
			fieldType = typeName + fieldName + "SubstitutionGroup"
		} else {
			if element.Plural {
				fieldType = "[]" + fieldType
			}
			if element.Optional && !element.Plural && !strings.HasPrefix(fieldType, "*") {
				fieldType = "*" + fieldType
			}
		}
		fields = append(fields, goComplexStructField{
			FieldName: fieldName,
			FieldType: fieldType,
			XMLTag:    gen.goXMLTagWithOptions(element.Namespace, element.Name),
			HasXMLTag: true,
		})
	}
	return fields
}

func (gen *CodeGenerator) goFlattenedComplexBaseFields(baseType string) []goComplexFlattenedBase {
	complexType := gen.goFindComplexType(baseType)
	if complexType == nil {
		return nil
	}
	namespace := complexType.Namespace
	if namespace == "" {
		namespace = gen.goTypeNamespace(complexType.Name)
	}
	typeName := gen.goTypeIdentifier(namespace, complexType.Name)
	fields := gen.goFlattenedComplexBaseFields(complexType.Base)
	fields = append(fields, goComplexFlattenedBase{
		TypeName: typeName,
		Fields:   gen.goComplexTypeOwnFields(complexType, typeName),
	})
	return fields
}

func (gen *CodeGenerator) goHasNamedType(name string) bool {
	return gen.goFindSimpleType(name) != nil || gen.goFindComplexType(name) != nil
}

func (gen *CodeGenerator) goElementBaseType(name string) string {
	if _, ok := goBuildinType[name]; ok {
		return name
	}
	if simpleType := gen.goFindSimpleType(name); simpleType != nil {
		return gen.goReferenceType(name)
	}
	baseType := gen.goResolvedBaseType(name)
	if _, ok := goBuildinType[baseType]; ok {
		return baseType
	}
	return gen.goReferenceType(name)
}

func goRequiredPointerFieldCheck(typeName, fieldName string) string {
	return fmt.Sprintf("\tif v.%s == nil {\n\t\treturn fmt.Errorf(%q)\n\t}", fieldName, typeName+"."+fieldName+" is required")
}

func goRequiredSubstitutionFieldCheck(typeName, fieldName string) string {
	return fmt.Sprintf("\tif v.%s.Value == nil {\n\t\treturn fmt.Errorf(%q)\n\t}", fieldName, typeName+"."+fieldName+" is required")
}

func (gen *CodeGenerator) goStructValidationCore(typeName string, requiredChecks []string) string {
	gen.ImportReflect = true
	validateBody := fmt.Sprintf("\treturn %s(reflect.ValueOf(v), false)", "validateStructFields"+typeName)
	if len(requiredChecks) > 0 {
		gen.ImportFmt = true
		validateBody = strings.Join(requiredChecks, "\n") + "\n" + validateBody
	}
	helperName := "validateStructFields" + typeName
	return fmt.Sprintf(`
func %s(value reflect.Value, allowValidator bool) error {
	if !value.IsValid() {
		return nil
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil
		}
		return %s(value.Elem(), allowValidator)
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		return %s(value.Elem(), true)
	}
	if allowValidator && value.CanInterface() {
		if validator, ok := value.Interface().(interface{ Validate() error }); ok {
			return validator.Validate()
		}
	}
	switch value.Kind() {
	case reflect.Struct:
		for idx := 0; idx < value.NumField(); idx++ {
			if value.Type().Field(idx).Name == "XMLName" {
				continue
			}
			if err := %s(value.Field(idx), true); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for idx := 0; idx < value.Len(); idx++ {
			if err := %s(value.Index(idx), true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v %s) Validate() error {
	%s
}
`, helperName, helperName, helperName, helperName, helperName, typeName, validateBody)
}

func (gen *CodeGenerator) goStructValidationOnlyMethods(typeName string, requiredChecks []string) string {
	return gen.goStructValidationCore(typeName, requiredChecks)
}

func (gen *CodeGenerator) goStructValidationMethods(typeName string, requiredChecks []string, embeddedPointerFields ...string) string {
	gen.ImportEncodingXML = true
	decodeAliasTypes := ""
	decodeShadowFields := ""
	decodeInitializers := ""
	decodeAssignments := ""
	decodeMainAssignment := fmt.Sprintf("\t*v = %s(value)\n", typeName)
	decodeValueType := "alias"
	for _, fieldName := range embeddedPointerFields {
		aliasName := fieldName + "Alias"
		decodeAliasTypes += fmt.Sprintf("\ttype %s %s\n", aliasName, fieldName)
		decodeShadowFields += fmt.Sprintf("\t\t*%s\n", aliasName)
		decodeInitializers += fmt.Sprintf("\tvalue.%s = &%s{}\n", aliasName, aliasName)
		decodeAssignments += fmt.Sprintf("\t%sValue := %s(*value.%s)\n\tv.%s = &%sValue\n", fieldName, fieldName, aliasName, fieldName, fieldName)
	}
	if len(embeddedPointerFields) > 0 {
		decodeValueType = "struct {\n\t\talias\n" + decodeShadowFields + "\t}"
		decodeMainAssignment = fmt.Sprintf("\t*v = %s(value.alias)\n", typeName)
	}
	return gen.goStructValidationCore(typeName, requiredChecks) + fmt.Sprintf(`
func (v *%s) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type alias %s
%s	var value %s
%s
	if err := d.DecodeElement(&value, &start); err != nil {
		return err
	}
%s%s
	return v.Validate()
}
`, typeName, typeName, decodeAliasTypes, decodeValueType, decodeInitializers, decodeMainAssignment, decodeAssignments)
}

func (gen *CodeGenerator) goSubstitutionFieldWrapper(typeName string, field goComplexElementField) string {
	gen.ImportEncodingXML = true
	gen.ImportFmt = true
	headType := typeName + field.FieldName + "SubstitutionGroupMember"
	markerName := "is" + headType
	content := fmt.Sprintf("type %s interface {\n\t%s()\n}\n", headType, markerName)
	for _, member := range field.SubstitutionMembers {
		memberType := gen.goElementTypeNameFor(gen.goElementNamespace(member), member.Name)
		content += fmt.Sprintf("\nfunc (*%s) %s() {}\n", memberType, markerName)
	}
	if field.Plural {
		typeName = typeName + field.FieldName + "SubstitutionGroup"
		content += fmt.Sprintf("type %s []%s\n", typeName, headType)
		content += fmt.Sprintf(`
func (v *%s) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	switch start.Name {
`, typeName)
		for _, member := range field.SubstitutionMembers {
			memberType := gen.goElementTypeNameFor(gen.goElementNamespace(member), member.Name)
			content += fmt.Sprintf("\tcase %s:\n\t\tvar value %s\n\t\tif err := d.DecodeElement(&value, &start); err != nil {\n\t\t\treturn err\n\t\t}\n\t\t*v = append(*v, &value)\n\t\treturn nil\n", gen.goElementXMLNameLiteral(member), memberType)
		}
		content += fmt.Sprintf(`	default:
		return fmt.Errorf("unsupported substitution group member %%s for %s", start.Name.Local)
	}
}

func (v %s) Values() []%s {
	return []%s(v)
}

func (v *%s) Append(values ...%s) {
	*v = append(*v, values...)
}

func (v %s) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	for _, item := range v {
		if item == nil {
			continue
		}
		if err := e.Encode(item); err != nil {
			return err
		}
	}
	return nil
}
`, field.FieldName, typeName, headType, headType, typeName, headType, typeName)
		return content
	}
	typeName = typeName + field.FieldName + "SubstitutionGroup"
	content += fmt.Sprintf("type %s struct {\n\tValue %s\n}\n", typeName, headType)
	content += fmt.Sprintf(`
func (v *%s) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	switch start.Name {
`, typeName)
	for _, member := range field.SubstitutionMembers {
		memberType := gen.goElementTypeNameFor(gen.goElementNamespace(member), member.Name)
		content += fmt.Sprintf("\tcase %s:\n\t\tvar value %s\n\t\tif err := d.DecodeElement(&value, &start); err != nil {\n\t\t\treturn err\n\t\t}\n\t\tv.Value = &value\n\t\treturn nil\n", gen.goElementXMLNameLiteral(member), memberType)
	}
	content += fmt.Sprintf(`	default:
		return fmt.Errorf("unsupported substitution group member %%s for %s", start.Name.Local)
	}
}

func (v %s) Get() %s {
	return v.Value
}

func (v *%s) Set(value %s) {
	v.Value = value
}

func (v %s) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if v.Value == nil {
		return nil
	}
	return e.Encode(v.Value)
}
`, field.FieldName, typeName, headType, typeName, headType, typeName)
	return content
}

func (gen *CodeGenerator) goComplexExtensionMethods(typeName string, baseRef string, baseType string, requiredChecks []string, fields []goComplexStructField) string {
	gen.ImportEncodingXML = true
	gen.ImportReflect = true
	gen.ImportFmt = gen.ImportFmt || len(requiredChecks) > 0
	methods := gen.goStructValidationOnlyMethods(typeName, requiredChecks)
	baseName := strings.TrimPrefix(baseType, "*")
	inheritedBases := gen.goFlattenedComplexBaseFields(baseRef)
	methods += fmt.Sprintf(`
func (v %s) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	value := struct {
`, typeName)
	for _, field := range fields {
		if field.HasXMLTag {
			methods += fmt.Sprintf("\t\t%s\t%s\t`xml:\"%s\"`\n", field.FieldName, field.FieldType, field.XMLTag)
		} else {
			methods += fmt.Sprintf("\t\t%s\t%s\n", field.FieldName, field.FieldType)
		}
	}
	for _, base := range inheritedBases {
		for _, field := range base.Fields {
			if field.HasXMLTag {
				methods += fmt.Sprintf("\t\t%s\t%s\t`xml:\"%s\"`\n", field.FieldName, field.FieldType, field.XMLTag)
			} else {
				methods += fmt.Sprintf("\t\t%s\t%s\n", field.FieldName, field.FieldType)
			}
		}
	}
	methods += "\t}{}\n"
	for _, field := range fields {
		methods += fmt.Sprintf("\tvalue.%s = v.%s\n", field.FieldName, field.FieldName)
	}
	for _, base := range inheritedBases {
		methods += fmt.Sprintf("\tif v.%s != nil {\n", base.TypeName)
		for _, field := range base.Fields {
			methods += fmt.Sprintf("\t\tvalue.%s = v.%s.%s\n", field.FieldName, base.TypeName, field.FieldName)
		}
		methods += "\t}\n"
	}
	methods += `	return e.EncodeElement(value, start)
}

func (v *` + typeName + `) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var value struct {
`
	for _, field := range fields {
		if field.HasXMLTag {
			methods += fmt.Sprintf("\t\t%s\t%s\t`xml:\"%s\"`\n", field.FieldName, field.FieldType, field.XMLTag)
		} else {
			methods += fmt.Sprintf("\t\t%s\t%s\n", field.FieldName, field.FieldType)
		}
	}
	for _, base := range inheritedBases {
		for _, field := range base.Fields {
			if field.HasXMLTag {
				methods += fmt.Sprintf("\t\t%s\t%s\t`xml:\"%s\"`\n", field.FieldName, field.FieldType, field.XMLTag)
			} else {
				methods += fmt.Sprintf("\t\t%s\t%s\n", field.FieldName, field.FieldType)
			}
		}
	}
	methods += `	}
	if err := d.DecodeElement(&value, &start); err != nil {
		return err
	}
`
	for _, field := range fields {
		methods += fmt.Sprintf("\tv.%s = value.%s\n", field.FieldName, field.FieldName)
	}
	for _, base := range inheritedBases {
		methods += fmt.Sprintf("\t%sValue := &%s{}\n", base.TypeName, base.TypeName)
		for _, field := range base.Fields {
			methods += fmt.Sprintf("\t%sValue.%s = value.%s\n", base.TypeName, field.FieldName, field.FieldName)
		}
	}
	if len(inheritedBases) > 0 {
		for idx := 0; idx < len(inheritedBases)-1; idx++ {
			methods += fmt.Sprintf("\t%sValue.%s = %sValue\n", inheritedBases[idx+1].TypeName, inheritedBases[idx].TypeName, inheritedBases[idx].TypeName)
		}
		methods += fmt.Sprintf("\tv.%s = %sValue\n", baseName, inheritedBases[len(inheritedBases)-1].TypeName)
	}
	methods += `	return v.Validate()
}
`
	return methods
}

func (gen *CodeGenerator) goElementMethods(typeName, baseType string, element *Element) string {
	gen.ImportEncodingXML = true
	if baseType == "time.Time" {
		gen.ImportTime = true
	}
	validateMethod := ""
	if gen.goTypeHasValidate(baseType) {
		validateMethod = fmt.Sprintf(`
func (v %s) Validate() error {
	return %s(v).Validate()
}
`, typeName, baseType)
	}
	return fmt.Sprintf(`
func (v %s) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = %s
	return e.EncodeElement(%s(v), start)
}
%s

func (v *%s) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var value %s
	if err := d.DecodeElement(&value, &start); err != nil {
		return err
	}
	*v = %s(value)
	return nil
}
`, typeName, gen.goXMLStartNameLiteral(element.Namespace, element.Name), baseType, validateMethod, typeName, baseType, typeName)
}

func (gen *CodeGenerator) goTypeHasValidate(name string) bool {
	if name == "" || isGoBuiltInType(strings.TrimPrefix(name, "[]")) || strings.HasPrefix(name, "[]") {
		return false
	}
	if simpleType := gen.goFindSimpleType(name); simpleType != nil {
		return gen.goSimpleTypeHasValidation(simpleType)
	}
	return gen.goFindComplexType(name) != nil
}

func (gen *CodeGenerator) goSubstitutionGroupInterface(typeName string, element *Element) string {
	methodName := gen.goSubstitutionGroupMarkerMethod(typeName)
	return fmt.Sprintf("type %s interface {\n\t%s()\n}\n", typeName, methodName)
}

func (gen *CodeGenerator) goSubstitutionGroupMarkerMethod(typeName string) string {
	return "is" + typeName
}

func (gen *CodeGenerator) goSubstitutionGroupMemberMethods(typeName string, element *Element) string {
	headElements := gen.goSubstitutionGroupHeads(element)
	if len(headElements) == 0 {
		return ""
	}
	var methods strings.Builder
	seen := make(map[string]bool)
	for _, head := range headElements {
		headType := gen.goElementTypeNameFor(gen.goElementNamespace(head), head.Name)
		marker := gen.goSubstitutionGroupMarkerMethod(headType)
		if seen[marker] {
			continue
		}
		seen[marker] = true
		methods.WriteString(fmt.Sprintf("\nfunc (*%s) %s() {}\n", typeName, marker))
	}
	return methods.String()
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
			fieldName := gen.goDeclarationName(v.Name, true)
			content := " string\n"
			gen.StructAST[v.Name] = content

			output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
			if gen.Hook != nil {
				gen.Hook.OnAddContent(gen, &output)
			}
			gen.Field += output
			gen.Field += gen.goUnionValidationMethods(fieldName, v.MemberTypes)
		}
		return
	}
	if _, ok := gen.StructAST[v.Name]; !ok {
		baseType := gen.goResolvedBaseType(v.Base)
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
		requiredChecks := make([]string, 0)
		pointerFieldInitializers := make([]string, 0)
		structFields := make([]goComplexStructField, 0)
		complexExtensionBase := ""
		if shouldAddGoXMLName(v.Name, v.Anonymous) {
			gen.ImportEncodingXML = true
			content += fmt.Sprintf("\tXMLName\txml.Name\t`xml:\"%s\"`\n", gen.goXMLName(v.Name, v.Namespace, v.Anonymous))
		}
		for _, attrGroup := range v.AttributeGroup {
			fieldType := getBasefromSimpleType(attrGroup.Ref, gen.ProtoTree)
			if fieldType == "time.Time" {
				gen.ImportTime = true
			}
			content += fmt.Sprintf("\t%s\t%s\n", genGoFieldName(attrGroup.Name, false), gen.goFieldType(fieldType))
		}

		for _, attribute := range v.Attributes {
			fieldType := gen.goValueFieldType(attribute.Type)
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
			if !attribute.Optional && strings.HasPrefix(fieldType, "*") {
				requiredChecks = append(requiredChecks, goRequiredPointerFieldCheck(fieldName, genGoFieldName(attribute.Name, false)+"Attr"))
			}
			attributeFieldName := genGoFieldName(attribute.Name, false) + "Attr"
			attributeTag := gen.goXMLTagWithOptions(attribute.Namespace, attribute.Name, ",attr", optional)
			content += fmt.Sprintf("\t%s\t%s\t`xml:\"%s\"`\n", attributeFieldName, fieldType, attributeTag)
			structFields = append(structFields, goComplexStructField{FieldName: attributeFieldName, FieldType: fieldType, XMLTag: attributeTag, HasXMLTag: true})
		}
		for _, group := range v.Groups {
			fieldType := gen.goValueFieldType(group.Ref)
			if group.Plural {
				fieldType = "[]" + fieldType
			}
			groupFieldName := genGoFieldName(group.Name, false)
			content += fmt.Sprintf("\t%s\t%s\n", groupFieldName, fieldType)
			structFields = append(structFields, goComplexStructField{FieldName: groupFieldName, FieldType: fieldType})
		}

		substitutionFields := make([]goComplexElementField, 0)
		for _, element := range v.Elements {
			elementFieldName := genGoFieldName(element.Name, false)
			fieldType := gen.goValueFieldType(element.Type)
			if helperName := gen.goEnsureInlineSimpleType(fieldName, elementFieldName, element.InlineSimpleType); helperName != "" {
				fieldType = gen.goFieldType(helperName)
			}
			members := gen.goConcreteSubstitutionGroupMembers(&element)
			if len(members) > 0 {
				fieldType = fieldName + elementFieldName + "SubstitutionGroup"
				content += fmt.Sprintf("\t%s\t%s\t`xml:\",any\"`\n", elementFieldName, fieldType)
				structFields = append(structFields, goComplexStructField{FieldName: elementFieldName, FieldType: fieldType, XMLTag: ",any", HasXMLTag: true})
				substitutionFields = append(substitutionFields, goComplexElementField{
					FieldName:           elementFieldName,
					FieldType:           fieldType,
					Plural:              element.Plural,
					Element:             element,
					SubstitutionMembers: members,
				})
				if !element.Optional && !element.Plural {
					requiredChecks = append(requiredChecks, goRequiredSubstitutionFieldCheck(fieldName, elementFieldName))
				}
				continue
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
			if !element.Optional && !element.Plural && strings.HasPrefix(fieldType, "*") {
				requiredChecks = append(requiredChecks, goRequiredPointerFieldCheck(fieldName, elementFieldName))
			}
			elementTag := gen.goXMLTagWithOptions(element.Namespace, element.Name, optional)
			content += fmt.Sprintf("\t%s\t%s\t`xml:\"%s\"`\n", elementFieldName, fieldType, elementTag)
			structFields = append(structFields, goComplexStructField{FieldName: elementFieldName, FieldType: fieldType, XMLTag: elementTag, HasXMLTag: true})
		}
		if len(v.Base) > 0 {
			// If the type is a built-in type, generate a Value field as chardata.
			// If the type is a simple type, keep it as chardata so text unmarshalling
			// still triggers its validation methods.
			// If it's not built-in one, embed the base type in the struct for the child type
			// to effectively inherit all of the base type's fields
			if simpleType := gen.goFindSimpleType(v.Base); simpleType != nil {
				fieldType := gen.goSimpleContentFieldType(v.Base)
				if fieldType == "time.Time" {
					gen.ImportTime = true
				}
				content += fmt.Sprintf("\tValue\t%s\t`xml:\",chardata\"`\n", fieldType)
			} else if isGoBuiltInType(v.Base) {
				content += fmt.Sprintf("\tValue\t%s\t`xml:\",chardata\"`\n", gen.goFieldType(v.Base))
			} else {
				baseFieldType := gen.goFieldType(v.Base)
				content += fmt.Sprintf("\t%s\n", baseFieldType)
				if strings.HasPrefix(baseFieldType, "*") {
					pointerFieldInitializers = append(pointerFieldInitializers, strings.TrimPrefix(baseFieldType, "*"))
				}
				complexExtensionBase = baseFieldType
			}
		}
		content += "}\n"
		gen.StructAST[v.Name] = content

		output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
		if gen.Hook != nil {
			gen.Hook.OnAddContent(gen, &output)
		}
		gen.Field += output
		for _, field := range substitutionFields {
			gen.Field += gen.goSubstitutionFieldWrapper(fieldName, field)
		}
		if complexExtensionBase != "" {
			gen.Field += gen.goComplexExtensionMethods(fieldName, v.Base, complexExtensionBase, requiredChecks, structFields)
		} else {
			gen.Field += gen.goStructValidationMethods(fieldName, requiredChecks, pointerFieldInitializers...)
		}
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

func goRegexpLiteral(pattern string) string {
	if !strings.Contains(pattern, "`") {
		return "`" + pattern + "`"
	}
	return strconv.Quote(pattern)
}

func (gen *CodeGenerator) goSimpleTypeValidationMethods(typeName, baseType string, restriction Restriction) string {
	if (len(restriction.Enum) == 0 && restriction.Pattern == nil) || !goSupportsSimpleTypeValidation(baseType) {
		return ""
	}
	if len(restriction.Enum) > 0 && restriction.Pattern == nil {
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
			gen.ImportFmt = true
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
	validatorName := "validate" + typeName
	validationChecks := make([]string, 0, 2)
	if len(restriction.Enum) > 0 {
		gen.ImportFmt = true
		caseValues := make([]string, 0, len(restriction.Enum))
		for _, enum := range restriction.Enum {
			caseValues = append(caseValues, fmt.Sprintf("%q", enum))
		}
		errorFormat := fmt.Sprintf("%s must be one of [%s], got %%q", typeName, strings.Join(restriction.Enum, ", "))
		validationChecks = append(validationChecks, fmt.Sprintf("\tswitch value {\n\tcase %s:\n\tdefault:\n\t\treturn fmt.Errorf(%q, value)\n\t}", strings.Join(caseValues, ", "), errorFormat))
	}
	if restriction.Pattern != nil {
		gen.ImportFmt = true
		gen.ImportRegexp = true
		pattern := restriction.Pattern.String()
		validationChecks = append(validationChecks, fmt.Sprintf("\tif !regexp.MustCompile(%s).MatchString(value) {\n\t\treturn fmt.Errorf(%q, value)\n\t}", goRegexpLiteral(pattern), fmt.Sprintf("%s must match pattern %s, got %%q", typeName, pattern)))
	}
	textExpr := goSimpleTypeTextExpr(baseType, "v")
	unmarshalAssignment := fmt.Sprintf("\t*v = %s(value)\n", typeName)
	if baseType != "string" {
		unmarshalAssignment = fmt.Sprintf("\tvar parsed %s\n\tif _, err := fmt.Sscan(value, &parsed); err != nil {\n\t\treturn err\n\t}\n\t*v = %s(parsed)\n", baseType, typeName)
		gen.ImportFmt = true
	}
	return fmt.Sprintf(`
func %s(value string) error {
%s
	return nil
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
`, validatorName, strings.Join(validationChecks, "\n"), typeName, validatorName, textExpr, typeName, textExpr, validatorName, typeName, validatorName, unmarshalAssignment)
}

func findSimpleTypeInTree(name string, protoTree []interface{}) *SimpleType {
	localName := trimNSPrefix(name)
	for _, ele := range protoTree {
		if v, ok := ele.(*SimpleType); ok && v.Name == localName {
			return v
		}
	}
	return nil
}

func (gen *CodeGenerator) goFindSimpleType(name string) *SimpleType {
	if simpleType := findSimpleTypeInTree(name, gen.ProtoTree); simpleType != nil {
		return simpleType
	}
	for _, tree := range gen.ParseFileMap {
		if simpleType := findSimpleTypeInTree(name, tree); simpleType != nil {
			return simpleType
		}
	}
	return nil
}

func (gen *CodeGenerator) goSimpleTypeHasValidation(simpleType *SimpleType) bool {
	if simpleType == nil {
		return false
	}
	if simpleType.Union && len(simpleType.MemberTypes) > 0 {
		return true
	}
	baseType := gen.goResolvedBaseType(simpleType.Base)
	return (len(simpleType.Restriction.Enum) > 0 || simpleType.Restriction.Pattern != nil) && goSupportsSimpleTypeValidation(baseType)
}

func (gen *CodeGenerator) goSimpleTypeValidatorName(name string) string {
	return "validate" + gen.goTypeIdentifier(gen.goTypeNamespace(name), name)
}

func goSupportsUnionValidation(memberType string) bool {
	switch memberType {
	case "string", "bool", "float32", "float64", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return true
	default:
		return false
	}
}

func (gen *CodeGenerator) goUnionValidationMethods(typeName string, memberTypes map[string]string) string {
	if len(memberTypes) == 0 {
		return ""
	}
	memberNames := make([]string, 0, len(memberTypes))
	validationChecks := make([]string, 0, len(memberTypes))
	alwaysValid := false
	for _, member := range toSortedPairs(memberTypes) {
		memberName := member.key
		memberType := member.value
		if memberType == "" {
			memberType = gen.goResolvedBaseType(memberName)
		}
		memberNames = append(memberNames, memberName)
		if memberSimpleType := gen.goFindSimpleType(memberName); gen.goSimpleTypeHasValidation(memberSimpleType) {
			validationChecks = append(validationChecks, fmt.Sprintf("\tif err := %s(value); err == nil {\n\t\treturn nil\n\t}", gen.goSimpleTypeValidatorName(memberName)))
			continue
		}
		if !goSupportsUnionValidation(memberType) {
			continue
		}
		if memberType == "string" {
			alwaysValid = true
			continue
		}
		gen.ImportStrconv = true
		switch memberType {
		case "bool":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseBool(value); err == nil {\n\t\treturn nil\n\t}")
		case "float32":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseFloat(value, 32); err == nil {\n\t\treturn nil\n\t}")
		case "float64":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseFloat(value, 64); err == nil {\n\t\treturn nil\n\t}")
		case "int":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseInt(value, 10, 0); err == nil {\n\t\treturn nil\n\t}")
		case "int8":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseInt(value, 10, 8); err == nil {\n\t\treturn nil\n\t}")
		case "int16":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseInt(value, 10, 16); err == nil {\n\t\treturn nil\n\t}")
		case "int32":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseInt(value, 10, 32); err == nil {\n\t\treturn nil\n\t}")
		case "int64":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseInt(value, 10, 64); err == nil {\n\t\treturn nil\n\t}")
		case "uint":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseUint(value, 10, 0); err == nil {\n\t\treturn nil\n\t}")
		case "uint8":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseUint(value, 10, 8); err == nil {\n\t\treturn nil\n\t}")
		case "uint16":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseUint(value, 10, 16); err == nil {\n\t\treturn nil\n\t}")
		case "uint32":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseUint(value, 10, 32); err == nil {\n\t\treturn nil\n\t}")
		case "uint64":
			validationChecks = append(validationChecks, "\tif _, err := strconv.ParseUint(value, 10, 64); err == nil {\n\t\treturn nil\n\t}")
		}
	}
	validatorBody := strings.Join(validationChecks, "\n")
	if alwaysValid || len(validationChecks) == 0 {
		validatorBody = "\treturn nil"
	} else {
		gen.ImportFmt = true
		validatorBody += fmt.Sprintf("\n\treturn fmt.Errorf(%q, value)", fmt.Sprintf("%s must match one of [%s], got %%q", typeName, strings.Join(memberNames, ", ")))
	}
	return fmt.Sprintf(`
func validate%s(value string) error {
%s
}

func (v %s) Validate() error {
	return validate%s(string(v))
}

func (v %s) MarshalText() ([]byte, error) {
	value := string(v)
	if err := validate%s(value); err != nil {
		return nil, err
	}
	return []byte(value), nil
}

func (v *%s) UnmarshalText(text []byte) error {
	value := string(text)
	if err := validate%s(value); err != nil {
		return err
	}
	*v = %s(value)
	return nil
}
`, typeName, validatorBody, typeName, typeName, typeName, typeName, typeName, typeName, typeName)
}

func (gen *CodeGenerator) goEnsureInlineSimpleType(ownerTypeName, fieldName string, simpleType *SimpleType) string {
	if simpleType == nil {
		return ""
	}
	if !simpleType.Union {
		if !gen.goSimpleTypeHasValidation(simpleType) {
			return ""
		}
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
		requiredChecks := make([]string, 0)
		if shouldAddGoXMLName(v.Name, false) {
			gen.ImportEncodingXML = true
			content += fmt.Sprintf("\tXMLName\txml.Name\t`xml:\"%s\"`\n", gen.goXMLName(v.Name, gen.goTypeNamespace(v.Name), false))
		}
		for _, element := range v.Elements {
			var plural string
			if element.Plural {
				plural = "[]"
			}
			fieldType := gen.goValueFieldType(element.Type)
			if !element.Optional && !element.Plural && strings.HasPrefix(fieldType, "*") {
				requiredChecks = append(requiredChecks, goRequiredPointerFieldCheck(fieldName, genGoFieldName(element.Name, false)))
			}
			content += fmt.Sprintf("\t%s\t%s%s\n", genGoFieldName(element.Name, false), plural, fieldType)
		}

		for _, group := range v.Groups {
			var plural string
			if group.Plural {
				plural = "[]"
			}
			content += fmt.Sprintf("\t%s\t%s%s\n", genGoFieldName(group.Name, false), plural, gen.goValueFieldType(group.Ref))
		}

		content += "}\n"
		gen.StructAST[v.Name] = content

		output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
		if gen.Hook != nil {
			gen.Hook.OnAddContent(gen, &output)
		}
		gen.Field += output
		gen.Field += gen.goStructValidationMethods(fieldName, requiredChecks)
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
			content += fmt.Sprintf("\tXMLName\txml.Name\t`xml:\"%s\"`\n", gen.goXMLName(v.Name, gen.goTypeNamespace(v.Name), false))
		}
		for _, attribute := range v.Attributes {
			var optional string
			if attribute.Optional {
				optional = `,omitempty`
			}
			content += fmt.Sprintf("\t%sAttr\t%s\t`xml:\"%s\"`\n", genGoFieldName(attribute.Name, false), gen.goValueFieldType(attribute.Type), gen.goXMLTagWithOptions(attribute.Namespace, attribute.Name, ",attr", optional))
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
		if len(gen.goSubstitutionGroupMembers(v)) > 0 {
			content := fmt.Sprintf(" interface {\n\t%s()\n}\n", gen.goSubstitutionGroupMarkerMethod(fieldName))
			gen.StructAST[v.Name] = content

			output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
			if gen.Hook != nil {
				gen.Hook.OnAddContent(gen, &output)
			}
			gen.Field += output
			return
		}
		fieldType := gen.goElementBaseType(v.Type)
		if helperName := gen.goEnsureInlineSimpleType(fieldName, "Value", v.InlineSimpleType); helperName != "" {
			fieldType = helperName
		}
		if v.InlineSimpleType == nil && v.SubstitutionGroup == "" && fieldType == fieldName && !gen.goHasNamedType(v.Type) {
			fieldType, _ = getBuildInTypeByLang("anyType", "Go")
		}
		declaredType := fieldType
		if v.Plural {
			declaredType = "[]" + declaredType
		}
		content := fmt.Sprintf(" %s\n", declaredType)
		gen.StructAST[v.Name] = content

		output := fmt.Sprintf("%stype %s%s", genFieldComment(fieldName, v.Doc, "//"), fieldName, gen.StructAST[v.Name])
		if gen.Hook != nil {
			gen.Hook.OnAddContent(gen, &output)
		}
		gen.Field += output
		gen.Field += gen.goElementMethods(fieldName, declaredType, v)
		gen.Field += gen.goSubstitutionGroupMemberMethods(fieldName, v)
	}
}

// GoAttribute generates code for attribute XML schema in Go language syntax.
func (gen *CodeGenerator) GoAttribute(v *Attribute) {
	if _, ok := gen.StructAST[v.Name]; !ok {
		fieldName := gen.goDeclarationName(v.Name, true)
		fieldType := gen.goValueFieldType(v.Type)
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
