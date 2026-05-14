// Copyright 2020 - 2024 The xgen Authors. All rights reserved. Use of this
// source code is governed by a BSD-style license that can be found in the
// LICENSE file.
//
// Package xgen written in pure Go providing a set of functions that allow you
// to parse XSD (XML schema files). This library needs Go version 1.10 or
// later.

package xgen

import "encoding/xml"

// OnComplexType handles parsing event on the complex start elements. A
// complex element contains other elements and/or attributes.
func (opt *Options) OnComplexType(ele xml.StartElement, protoTree []interface{}) (err error) {
	if opt.ComplexType.Len() > 0 {
		e := opt.Element.Pop().(*Element)
		opt.ComplexType.Push(&ComplexType{
			Doc:       e.Doc,
			Name:      e.Name,
			Namespace: e.Namespace,
		})
	}

	if opt.ComplexType.Len() == 0 {
		c := ComplexType{}
		opt.CurrentEle = opt.InElement
		inheritedName := false
		for _, attr := range ele.Attr {
			if attr.Name.Local == "name" {
				c.Name = attr.Value
				c.Namespace = opt.TargetNamespace
			}
		}
		if opt.Element.Len() > 0 {
			e := opt.Element.Pop().(*Element)
			c.Doc = e.Doc
			c.Namespace = e.Namespace
			if c.Name == "" {
				c.Name = e.Name
				inheritedName = true
			}
		}
		c.Anonymous = inheritedName
		opt.ComplexType.Push(&c)
	}
	return
}

// EndComplexType handles parsing event on the complex end elements.
func (opt *Options) EndComplexType(ele xml.EndElement, protoTree []interface{}) (err error) {
	opt.ProtoTree = append(opt.ProtoTree, opt.ComplexType.Pop())
	opt.CurrentEle = ""
	return
}
