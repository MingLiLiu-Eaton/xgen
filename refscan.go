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
	"io"
	"os"
	"path/filepath"

	"golang.org/x/net/html/charset"
)

func collectQualifiedElementRefs(path string) (map[string]bool, error) {
	refs := make(map[string]bool)
	files, err := GetFileList(path)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if filepath.Ext(file) != ".xsd" {
			continue
		}
		if err := collectQualifiedElementRefsFromFile(file, refs); err != nil {
			return nil, err
		}
	}
	return refs, nil
}

func collectQualifiedElementRefsFromFile(path string, refs map[string]bool) error {
	xmlFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer xmlFile.Close()

	decoder := xml.NewDecoder(xmlFile)
	decoder.CharsetReader = charset.NewReaderLabel
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "element" {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "ref" && getNSPrefix(attr.Value) != "" {
				refs[attr.Value] = true
			}
		}
	}
}
