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
	"strconv"
)

// OnAny handles parsing event on any wildcard elements.
func (opt *Options) OnAny(ele xml.StartElement, protoTree []interface{}) (err error) {
	e := Element{Name: "Any", Type: "anyType", Wildcard: true}
	for _, attr := range ele.Attr {
		if attr.Name.Local == "maxOccurs" {
			var maxOccurs int
			if maxOccurs, err = strconv.Atoi(attr.Value); attr.Value != "unbounded" && err != nil {
				return
			}
			if attr.Value == "unbounded" || maxOccurs > 1 {
				e.Plural, err = true, nil
			}
		}
		if attr.Name.Local == "minOccurs" {
			var minOccurs int
			if minOccurs, err = strconv.Atoi(attr.Value); err != nil {
				return
			}
			if minOccurs == 0 {
				e.Optional = true
			}
		}
	}

	if len(opt.InPluralSequence) > 0 && opt.InPluralSequence[len(opt.InPluralSequence)-1] {
		e.Plural = true
	}
	if opt.Choice.Len() > 0 {
		e.Optional = true
		e.Plural = e.Plural || opt.Choice.Peek().(*Choice).Plural
	}

	if opt.ComplexType.Len() > 0 {
		element, i := findElement(&e, opt.ComplexType.Peek().(*ComplexType).Elements)
		if element != nil && element.Type == e.Type {
			element.Plural = element.Plural || e.Plural
			element.Optional = element.Optional && e.Optional
			opt.ComplexType.Peek().(*ComplexType).Elements[i] = *element
		} else {
			opt.ComplexType.Peek().(*ComplexType).Elements = append(opt.ComplexType.Peek().(*ComplexType).Elements, e)
		}
		return
	}

	if opt.InGroup > 0 {
		if opt.Group.Len() > 0 {
			opt.Group.Peek().(*Group).Elements = append(opt.Group.Peek().(*Group).Elements, e)
		}
	}
	return
}

// EndAny handles parsing event on any wildcard end elements.
func (opt *Options) EndAny(ele xml.EndElement, protoTree []interface{}) (err error) {
	return nil
}
