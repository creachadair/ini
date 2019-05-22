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
	{"mixed-value blank", "a=\n b\nc=\nd", []result{
		{1, "key/value", "a", []string{"b"}}, // indented, same key
		{3, "key/value", "c", []string{""}},
		{4, "key/value", "d", []string{""}}, // not indented, separate key
	}},

	{"normalize keys", " a   long   key = value   village", []result{
		{1, "key/value", "a long key", []string{"value   village"}},
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

func TestParse(t *testing.T) {
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var got []result
			push := func(r result) error {
				got = append(got, r)
				return nil
			}

			if err := ini.Parse(strings.NewReader(test.input), ini.Handler{
				Comment: func(loc ini.Location, text string) error {
					return push(result{loc.Line, "comment", "", nil})
				},
				Section: func(loc ini.Location, name string) error {
					return push(result{loc.Line, "section", name, nil})
				},
				KeyValue: func(loc ini.Location, key string, values []string) error {
					return push(result{loc.Line, "key/value", key, values})
				},
			}); err != nil {
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
