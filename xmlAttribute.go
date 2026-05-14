// Copyright 2020 - 2024 The xgen Authors. All rights reserved. Use of this
// source code is governed by a BSD-style license that can be found in the
// LICENSE file.
//
// Package xgen written in pure Go providing a set of functions that allow you
// to parse XSD (XML schema files). This library needs Go version 1.10 or
// later.

package xgen

import "encoding/xml"

// OnAttribute handles parsing event on the attribute start elements. All
// attributes are declared as simple types.
func (opt *Options) OnAttribute(ele xml.StartElement, protoTree []interface{}) (err error) {
	attribute := Attribute{
		Optional: true,
	}
	localAttribute := opt.ComplexType.Len() > 0 || opt.AttributeGroup.Len() > 0
	explicitUnqualified := false
	for _, attr := range ele.Attr {
		if attr.Name.Local == "ref" {
			attribute.Name = attr.Value
			attribute.Namespace = opt.parseNS(attr.Value)
			if attribute.Namespace == "" {
				attribute.Namespace = opt.TargetNamespace
			}
			attribute.Type, err = opt.GetValueType(attr.Value, protoTree)
			if err != nil {
				return
			}
		}
		if attr.Name.Local == "name" {
			attribute.Name = attr.Value
		}
		if attr.Name.Local == "form" {
			if attr.Value == "qualified" {
				attribute.Namespace = opt.TargetNamespace
			}
			if attr.Value == "unqualified" {
				explicitUnqualified = true
				attribute.Namespace = ""
			}
		}
		if attr.Name.Local == "type" {
			attribute.Type, err = opt.GetValueType(attr.Value, protoTree)
			if err != nil {
				return
			}
		}
		if attr.Name.Local == "use" {
			if attr.Value == "required" {
				attribute.Optional = false
			}
		}
	}
	if attribute.Name != "" && attribute.Namespace == "" && (!localAttribute || (!explicitUnqualified && opt.AttributeFormQualified)) {
		attribute.Namespace = opt.TargetNamespace
	}
	opt.Attribute.Push(&attribute)
	return
}

// EndAttribute handles parsing event on the attribute end elements.
func (opt *Options) EndAttribute(ele xml.EndElement, protoTree []interface{}) (err error) {
	if opt.Attribute.Len() == 0 {
		return
	}
	if opt.AttributeGroup.Len() > 0 {
		opt.AttributeGroup.Peek().(*AttributeGroup).Attributes = append(opt.AttributeGroup.Peek().(*AttributeGroup).Attributes, *opt.Attribute.Pop().(*Attribute))
		return
	}
	if opt.ComplexType.Len() > 0 {
		opt.ComplexType.Peek().(*ComplexType).Attributes = append(opt.ComplexType.Peek().(*ComplexType).Attributes, *opt.Attribute.Pop().(*Attribute))
		return
	}
	if opt.ComplexType.Len() == 0 {
		opt.ProtoTree = append(opt.ProtoTree, opt.Attribute.Pop())
	}
	return
}
