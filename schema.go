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
	"strings"
)

func (opt *Options) prepareLocalNameNSMap(element xml.StartElement) {
	for _, ele := range element.Attr {
		if ele.Name.Space == "xmlns" {
			opt.LocalNameNSMap[ele.Name.Local] = ele.Value
		}
	}
}

func (opt *Options) prepareNSSchemaLocationMap(element xml.StartElement) {
	var currentNS string
	for _, ele := range element.Attr {
		if ele.Name.Local == "namespace" {
			currentNS = ele.Value
		}
		if ele.Name.Local == "schemaLocation" {
			if _, ok := opt.NSSchemaLocationMap[currentNS]; ok {
				continue
			}
			if isValidURL(ele.Value) {
				continue
				// TODO: fetch remote schema
				// var err error
				// if opt.RemoteSchema[ele.Value], err = fetchSchema(ele.Value); err != nil {
				// 	continue
				// }
			}
			opt.NSSchemaLocationMap[currentNS] = ele.Value
		}
	}
}

func (opt *Options) parseNS(str string) (ns string) {
	return opt.LocalNameNSMap[getNSPrefix(str)]
}

func (opt *Options) prepareNamespacePrefixMap() {
	if opt.NamespacePrefixMap == nil {
		opt.NamespacePrefixMap = make(map[string]string)
	}
	for prefix, namespace := range opt.LocalNameNSMap {
		opt.registerNamespacePrefix(namespace, prefix)
	}
	opt.registerNamespacePrefix(opt.TargetNamespace, "")
}

func (opt *Options) registerNamespacePrefix(namespace, declaredPrefix string) {
	if namespace == "" {
		return
	}
	if _, ok := opt.NamespacePrefixMap[namespace]; ok {
		return
	}
	candidate := normalizeNamespacePrefixCandidate(declaredPrefix)
	if candidate == "" {
		candidate = normalizeNamespacePrefixCandidate(namespaceTypeLabel(namespace))
	}
	if candidate == "" {
		candidate = "ns"
	}
	if !namespacePrefixInUse(opt.NamespacePrefixMap, candidate, namespace) {
		opt.NamespacePrefixMap[namespace] = candidate
		return
	}
	label := normalizeNamespacePrefixCandidate(namespaceTypeLabel(namespace))
	if label != "" && !strings.EqualFold(candidate, label) {
		combined := normalizeNamespacePrefixCandidate(candidate + "_" + label)
		if combined != "" && !namespacePrefixInUse(opt.NamespacePrefixMap, combined, namespace) {
			opt.NamespacePrefixMap[namespace] = combined
			return
		}
	}
	for suffix := 2; ; suffix++ {
		next := fmt.Sprintf("%s_%d", candidate, suffix)
		if !namespacePrefixInUse(opt.NamespacePrefixMap, next, namespace) {
			opt.NamespacePrefixMap[namespace] = next
			return
		}
	}
}
