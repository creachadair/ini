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

// Package ini implements a basic INI file parser.
package ini

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Handler is a structure containing options and callbacks used by the parser
// to process INI contents. If a callback reports an error, parsing stops and
// that error is returned to the caller of Parse. Any callback that is nil will
// be skipped without error.
type Handler struct {
	// Comment delivers the contents of a comment. The comment text includes the
	// leading delimiter, but leading and trailing whitespace are removed.
	Comment func(loc Location, text string) error

	// Section delivers a section header. Whitespace in name is normalized.  The
	// loc.Section field contains the name of the most recent section label
	// prior to this one.
	Section func(loc Location, name string) error

	// KeyValue delivers the values for a single key. Whitespace in the key name
	// is normalized. The values slice will not be empty, but will contain ""
	// for a key with only one empty value.
	KeyValue func(loc Location, key string, values []string) error
}

func (h Handler) comment(loc Location, text string) error {
	if h.Comment != nil {
		return h.Comment(loc, text)
	}
	return nil
}

func (h Handler) section(loc Location, name string) error {
	if h.Section != nil {
		return h.Section(loc, name)
	}
	return nil
}

func (h Handler) keyValue(loc Location, key string, values []string) error {
	if h.KeyValue != nil {
		return h.KeyValue(loc, key, values)
	}
	return nil
}

// A Location describes the physical location of an input element.
type Location struct {
	Line    int    // line number, 1-based
	Section string // most recent section name (or "")
}

// SyntaxError is the concrete type of error values denoting syntax problems
// with INI input.
type SyntaxError struct {
	Location        // where the error occurred
	Desc     string // general description of the error
	Key      string // if applicable, the key or name affected
}

func (s *SyntaxError) Error() string {
	msg := fmt.Sprintf("line %d: %s", s.Location.Line, s.Desc)
	if s.Key != "" {
		msg += ": " + s.Key
	}
	return msg
}

func syntaxError(loc Location, msg, key string) error {
	return &SyntaxError{Location: loc, Desc: msg, Key: key}
}

const (
	msgUnclosedHeader = "unclosed section header"
	msgInvalidSection = "invalid section name"
	msgEmptyKey       = "empty key"
)

// Parse scans the INI data from r and invokes the callbacks on h with the
// results. If h reports an error, parsing stops and that error is returned to
// the caller of Parse. Errors in syntax have concrete type *SyntaxError, and
// may be asserted to that type to recover location and name details.
//
// The INI syntax supported by Parse ignores blank lines and removes leading
// and trailing whitespace from keys, section names, and values. Whole-line
// comments are prefixed with a semicolon:
//
//	; this is a comment
//
// Section headers are enclosed in square brackets, and permit horizontal
// whitespace:
//
//	[section header]
//
// Key-value pairs allow whitespace where sensible:
//
//	key1=first value
//	key2 = second value
//
// Keys and section names may contain whitespace, which is normalized.  Each
// run of whitespace inside the key or section name is replaced by one Unicode
// space (32) character.  Whitespace is not normalized within values:
//
//	; "a long key" has value "value   village"
//	a    long     key = value   village
//
// Keys may have multiple values, indicated by indentation:
//
//	; letter has values alpha, bravo, charlie
//	letter = alpha
//	    bravo
//	    charlie
//
//	; number has values 1, 2, 3
//	number =
//	    1
//	    2
//	    3
//
// A bare key that is not indented is assigned an empty value, so the following
// are equivalent:
//
//	foo
//	foo=
//	foo =
//
// Note that these rules imply you cannot have a multi-valued key with an empty
// string as one of its values.
//
// Parse does not check for duplication among section headers or keys; the
// caller is responsible for any validation that is required.
// Line continuations with trailing backslashes are not currently supported.
// String quotation is not currently supported.
func Parse(r io.Reader, h Handler) error {
	buf := bufio.NewScanner(r)
	var loc Location // current physical input location

	var keyLoc Location // location of curKey
	var curKey string   // current key being processed
	var values []string // values for curKey

	emit := func() error {
		defer func() { curKey = ""; values = nil }()
		if curKey == "" {
			return nil
		}
		return h.keyValue(keyLoc, curKey, values)
	}

	for buf.Scan() {
		loc.Line++
		text := buf.Text()
		clean := strings.TrimSpace(text)
		if clean == "" {
			continue // skip blank lines
		}
		isIndented := text != "" && (text[0] == ' ' || text[0] == '\t')

		if strings.HasPrefix(clean, ";") {
			if err := emit(); err != nil {
				return err
			} else if err := h.comment(loc, text); err != nil {
				return err
			}
			continue
		}

		if clean[0] == '[' {
			if clean[len(clean)-1] != ']' {
				return syntaxError(loc, msgUnclosedHeader, clean[1:])
			}
			name := cleanKey(clean[1 : len(clean)-1])
			if name == "" || strings.ContainsAny(name, "[]") {
				return syntaxError(loc, msgInvalidSection, name)
			} else if err := emit(); err != nil {
				return err
			} else if err := h.section(loc, name); err != nil {
				return err
			}
			loc.Section = name
			continue
		}

		i := strings.Index(clean, "=")
		if i < 0 {
			// If a bare key is indented, it may be the value for a previous key.
			if isIndented && curKey != "" {
				if len(values) == 1 && values[0] == "" {
					values[0] = clean
				} else {
					values = append(values, clean)
				}
				continue
			}

			// If it is not indented, or there isn't a previous key waiting for
			// more values, this is a new key with no value. Because there is no
			// equal sign to support continuations, this key cannot have more than
			// one value of its own so we bypass accumulation
			if err := emit(); err != nil {
				return err
			} else if err := h.keyValue(loc, cleanKey(clean), []string{""}); err != nil {
				return err
			}
			continue
		}

		// At this point we have a key=value pair, which we must accumulate.
		key := cleanKey(clean[:i])
		if key == "" {
			return syntaxError(loc, msgEmptyKey, "")
		}
		value := strings.TrimSpace(clean[i+1:])
		if key != curKey {
			if err := emit(); err != nil {
				return err
			}
			keyLoc = loc
			curKey = key
		}
		values = append(values, value)
	}
	if err := buf.Err(); err != nil {
		return err
	}
	return emit() // emit any leftover key/values
}

func cleanKey(key string) string {
	return strings.Join(strings.Fields(key), " ")
}
