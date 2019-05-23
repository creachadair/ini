// Copyright 2019 Michael J. Fromberger. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ini_test

import (
	"strings"
	"testing"

	"bitbucket.org/creachadair/ini"
	"github.com/google/go-cmp/cmp"
)

type result struct {
	Line   int
	Kind   string
	Key    string
	Values []string
}

var tests = []struct {
	desc  string
	input string
	want  []result
}{
	{"empty input", "", nil},
	{"whitespace", "  \n   \t\n", nil},

	{"comment", "\n; blah\n ; blah\n\n", []result{
		{2, "comment", "", nil},
		{3, "comment", "", nil},
	}},

	{"section", "\n[alpha]\n [bravo]\n\n[ charlie   delta\t echo ]\n", []result{
		{2, "section", "alpha", nil},
		{3, "section", "bravo", nil},
		{5, "section", "charlie delta echo", nil}, // normalize whitespace
	}},

	{"bare keys", "\n  \na\nb\n", []result{
		{3, "key/value", "a", []string{""}},
		{4, "key/value", "b", []string{""}},
	}},

	{"single keys", " a = 45 \nb = 29", []result{
		{1, "key/value", "a", []string{"45"}},
		{2, "key/value", "b", []string{"29"}},
	}},

	{"multi-value blank", "a=\n b\n c", []result{
		{1, "key/value", "a", []string{"b", "c"}},
	}},
	{"multi-value nonblank", "a=b\n c\n d", []result{
		{1, "key/value", "a", []string{"b", "c", "d"}},
	}},
	{"multi-value skip", "a=\n b\n\n c\n d\n", []result{
		{1, "key/value", "a", []string{"b", "c", "d"}},
	}},
	{"mixed-value blank", "a=\n b\nc=\nd", []result{
		{1, "key/value", "a", []string{"b"}}, // indented, same key
		{3, "key/value", "c", []string{""}},
		{4, "key/value", "d", []string{""}}, // not indented, new key
	}},
	{"mixed-value indent", "a=\n b\n c=d\n\ne", []result{
		{1, "key/value", "a", []string{"b"}}, // indented, attaches
		{3, "key/value", "c", []string{"d"}}, // indented with =, new key
		{5, "key/value", "e", []string{""}},  // not indented, new key
	}},

	{"normalize keys", " a   long   key = value   village", []result{
		{1, "key/value", "a long key", []string{"value   village"}},
	}},

	{"sample.ini", sampleFile, []result{
		{1, "comment", "", nil},
		{3, "section", "quoted_fields", nil},

		// Note that quotation marks around the field value are preserved.
		{4, "key/value", "required", []string{`"EmailAddr,FirstName,LastName,Mesg"`}},
		{5, "key/value", "csvfile", []string{`"contacts.csv"`}},
	}},

	{"LLVMBuild.txt", llvmBuildText, []result{
		{1, "comment", "", nil}, {2, "comment", "", nil}, {3, "comment", "", nil},
		{4, "comment", "", nil}, {5, "comment", "", nil}, {6, "comment", "", nil},
		{7, "comment", "", nil}, {8, "comment", "", nil}, {9, "comment", "", nil},
		{10, "comment", "", nil}, {11, "comment", "", nil}, {12, "comment", "", nil},
		{13, "comment", "", nil}, {14, "comment", "", nil}, {15, "comment", "", nil},

		{17, "section", "common", nil},

		{18, "key/value", "subdirectories", []string{
			"Analysis", "AsmParser", "Bitcode", "CodeGen", "DebugInfo", "Demangle",
			"ExecutionEngine", "FuzzMutate", "LineEditor", "Linker", "IR", "IRReader",
			"LTO", "MC", "MCA", "Object", "BinaryFormat", "ObjectYAML", "Option",
			"Remarks", "Passes", "ProfileData", "Support", "TableGen", "TextAPI",
			"Target", "Testing", "ToolDrivers", "Transforms", "WindowsManifest", "XRay",
		}},

		{51, "section", "component_0", nil},

		{52, "key/value", "type", []string{"Group"}},
		{53, "key/value", "name", []string{"Libraries"}},
		{54, "key/value", "parent", []string{"$ROOT"}},
	}},
}

func runParser(s string) ([]result, error) {
	var got []result
	push := func(r result) error {
		got = append(got, r)
		return nil
	}

	err := ini.Parse(strings.NewReader(s), ini.Handler{
		Comment: func(loc ini.Location, text string) error {
			return push(result{loc.Line, "comment", "", nil})
		},
		Section: func(loc ini.Location, name string) error {
			return push(result{loc.Line, "section", name, nil})
		},
		KeyValue: func(loc ini.Location, key string, values []string) error {
			return push(result{loc.Line, "key/value", key, values})
		},
	})
	return got, err
}

func TestParse(t *testing.T) {
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got, err := runParser(test.input)
			if err != nil {
				if len(test.input) > 60 {
					test.input = test.input[:60] + "..."
				}
				t.Errorf("Parsing %q failed: %v", test.input, err)
			} else if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Parse results (-want, +got)\n%s", diff)
			}
		})
	}
}

// These must be in sync with the package ini values.
const (
	msgUnclosedHeader = "unclosed section header"
	msgInvalidSection = "invalid section name"
	msgEmptyKey       = "empty key"
)

func TestParseErrors(t *testing.T) {
	tests := []struct {
		input, desc, key string
	}{
		{"[bad", msgUnclosedHeader, "bad"},
		{"[bad name]]", msgInvalidSection, "bad name]"},
		{"[[bad name]", msgInvalidSection, "[bad name"},
		{"= missing key", msgEmptyKey, ""},
		{"  = missing key ", msgEmptyKey, ""},
	}
	for _, test := range tests {
		got, err := runParser(test.input)
		t.Logf("Parse(%q) reports %+v", test.input, err)
		if err == nil {
			t.Errorf("Parse(%q): got %+v, want error", test.input, got)
		} else if e, ok := err.(*ini.SyntaxError); !ok {
			t.Errorf("Parse(%q): got unexpected error: %v", test.input, err)
		} else if e.Desc != test.desc || e.Key != test.key {
			t.Errorf("Parse(%q): got error (%q, %q), want (%q, %q)",
				test.input, e.Desc, e.Key, test.desc, test.key)
		}
	}
}

const llvmBuildText = "" +
	`;===- ./lib/LLVMBuild.txt --------------------------------------*- Conf -*--===;
;
; Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
; See https://llvm.org/LICENSE.txt for license information.
; SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
;
;===------------------------------------------------------------------------===;
;
; This is an LLVMBuild description file for the components in this subdirectory.
;
; For more information on the LLVMBuild system, please see:
;
;   http://llvm.org/docs/LLVMBuild.html
;
;===------------------------------------------------------------------------===;

[common]
subdirectories =
 Analysis
 AsmParser
 Bitcode
 CodeGen
 DebugInfo
 Demangle
 ExecutionEngine
 FuzzMutate
 LineEditor
 Linker
 IR
 IRReader
 LTO
 MC
 MCA
 Object
 BinaryFormat
 ObjectYAML
 Option
 Remarks
 Passes
 ProfileData
 Support
 TableGen
 TextAPI
 Target
 Testing
 ToolDrivers
 Transforms
 WindowsManifest
 XRay

[component_0]
type = Group
name = Libraries
parent = $ROOT
`

const sampleFile = `; Example with quotes and trailing whitespace

[quoted_fields]   
  required = "EmailAddr,FirstName,LastName,Mesg"   
  csvfile = "contacts.csv" 
`
