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
	"regexp"
)

// OnPattern handles parsing event on the pattern start elements.
func (opt *Options) OnPattern(ele xml.StartElement, protoTree []interface{}) (err error) {
	for _, attr := range ele.Attr {
		if attr.Name.Local != "value" || opt.SimpleType.Peek() == nil {
			continue
		}
		opt.SimpleType.Peek().(*SimpleType).Restriction.Pattern, err = regexp.Compile(attr.Value)
		if err != nil {
			return err
		}
	}
	return nil
}

// EndPattern handles parsing event on the pattern end elements. Pattern
// defines the exact sequence of characters that are acceptable.
func (opt *Options) EndPattern(ele xml.EndElement, protoTree []interface{}) (err error) {
	return opt.finalizeInlineSimpleTypeTarget()
}
