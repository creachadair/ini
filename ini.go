// Package ini implements a basic INI file parser.
package ini

import (
	"bufio"
	"io"
	"strings"

	"golang.org/x/xerrors"
)

// Handler is a structure containing options and callbacks used by the parser
// to process INI contents. If a callback reports an error, parsing stops and
// that error is returned to the caller of Parse. Any callback that is nil will
// be skipped without error.
type Handler struct {
	// Comment delivers the contents of a comment. The comment text includes the
	// leading delimiter, but leading and trailing whitespace are removed.
	Comment func(loc Location, text string) error

	// Section delivers a section header.
	Section func(loc Location, name string) error

	// KeyValue delivers the values for a single key. The values slice will not
	// be empty, but will contain "" for a key with no value.
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
	Line int // line number, 1-based
}

// Parse scans the INI data from r and invokes the callbacks on h with the
// results. If h reports an error, parsing stops and that error is returned to
// the caller of Parse.
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
				return xerrors.Errorf("line %d: unclosed section header", loc.Line)
			} else if err := emit(); err != nil {
				return err
			}
			name := strings.TrimSpace(clean[1 : len(clean)-1])
			if err := h.section(loc, name); err != nil {
				return err
			}
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
			} else if err := h.keyValue(loc, clean, []string{""}); err != nil {
				return err
			}
			continue
		}

		// At this point we have a key=value pair, which we must accumulate.
		key := strings.TrimSpace(clean[:i])
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
