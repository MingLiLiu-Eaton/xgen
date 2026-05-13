// Copyright 2020 - 2024 The xgen Authors. All rights reserved. Use of this
// source code is governed by a BSD-style license that can be found in the
// LICENSE file.
//
// Package xgen written in pure Go providing a set of functions that allow you
// to parse XSD (XML schema files). This library needs Go version 1.10 or
// later.

package xgen

import "encoding/xml"

// OnSimpleType handles parsing event on the simpleType start elements. The
// simpleType element defines a simple type and specifies the constraints and
// information about the values of attributes or text-only elements.
func (opt *Options) OnSimpleType(ele xml.StartElement, protoTree []interface{}) (err error) {
	if opt.SimpleType.Len() == 0 {
		opt.SimpleType.Push(&SimpleType{})
	}
	if opt.CurrentEle == "attributeGroup" {
		// return
	}
	for _, attr := range ele.Attr {
		if attr.Name.Local == "name" {
			opt.CurrentEle = opt.InElement
			opt.SimpleType.Peek().(*SimpleType).Name = attr.Value
		}
	}
	if opt.SimpleType.Peek() != nil && opt.SimpleType.Peek().(*SimpleType).Name == "" && (opt.Attribute.Len() > 0 || opt.Element.Len() > 0) {
		opt.SimpleType.Peek().(*SimpleType).Anonymous = true
	}
	return
}

// EndSimpleType handles parsing event on the simpleType end elements.
func (opt *Options) EndSimpleType(ele xml.EndElement, protoTree []interface{}) (err error) {
	if opt.attachInlineSimpleType() {
		return
	}
	if ele.Name.Local == opt.CurrentEle && opt.ComplexType.Len() == 1 {
		opt.ProtoTree = append(opt.ProtoTree, opt.ComplexType.Pop())
		opt.CurrentEle = ""
	}

	if ele.Name.Local == opt.CurrentEle && !opt.InUnion {
		opt.ProtoTree = append(opt.ProtoTree, opt.SimpleType.Pop())
		opt.CurrentEle = ""
	}
	return
}

func (opt *Options) finalizeInlineSimpleTypeTarget() (err error) {
	if opt.SimpleType.Len() == 0 || opt.SimpleType.Peek() == nil {
		return nil
	}
	valueType, err := opt.GetValueType(opt.SimpleType.Peek().(*SimpleType).Base, opt.ProtoTree)
	if err != nil {
		return err
	}
	if opt.Attribute.Len() > 0 && opt.Attribute.Peek() != nil {
		opt.Attribute.Peek().(*Attribute).Type = valueType
		opt.CurrentEle = ""
		return nil
	}
	if opt.Element.Len() > 0 && opt.Element.Peek() != nil {
		opt.Element.Peek().(*Element).Type = valueType
		opt.updateCurrentElement()
		opt.CurrentEle = ""
	}
	return nil
}

func (opt *Options) attachInlineSimpleType() bool {
	if opt.SimpleType.Len() == 0 || opt.SimpleType.Peek() == nil {
		return false
	}
	if opt.Attribute.Len() > 0 && opt.Attribute.Peek() != nil {
		simpleType := opt.SimpleType.Pop().(*SimpleType)
		if opt.Attribute.Peek().(*Attribute).Type == "" {
			opt.Attribute.Peek().(*Attribute).Type = simpleType.Base
		}
		opt.Attribute.Peek().(*Attribute).InlineSimpleType = simpleType
		return true
	}
	if opt.Element.Len() > 0 && opt.Element.Peek() != nil {
		simpleType := opt.SimpleType.Pop().(*SimpleType)
		if opt.Element.Peek().(*Element).Type == "" {
			opt.Element.Peek().(*Element).Type = simpleType.Base
		}
		opt.Element.Peek().(*Element).InlineSimpleType = simpleType
		opt.updateCurrentElement()
		return true
	}
	return false
}

func (opt *Options) updateCurrentElement() {
	if opt.Element.Len() == 0 || opt.Element.Peek() == nil || opt.ComplexType.Len() == 0 || opt.ComplexType.Peek() == nil {
		return
	}
	elements := opt.ComplexType.Peek().(*ComplexType).Elements
	if len(elements) == 0 {
		return
	}
	elements[len(elements)-1] = *opt.Element.Peek().(*Element)
	opt.ComplexType.Peek().(*ComplexType).Elements = elements
}
