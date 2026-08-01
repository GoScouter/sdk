package style

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const indentWidth = 4

// Render turns an arbitrary JSON report into an indented, styled block of
// terminal text. It is the fallback view for modules that have no opinion on
// how their results should look: keys become labels, nested objects become
// headings, and array elements become bullets.
func Render(raw json.RawMessage) string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var b strings.Builder
	if err := value(&b, dec, "", 0); err != nil {
		return Failuref("scan: unreadable report: %v", err) + "\r\n"
	}

	return b.String()
}

func value(b *strings.Builder, dec *json.Decoder, label string, depth int) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}

	if d, ok := tok.(json.Delim); ok {
		switch d {
		case '{':
			return object(b, dec, label, depth)
		case '[':
			return array(b, dec, label, depth)
		default:
			return fmt.Errorf("unexpected %q", d)
		}
	}

	field(b, depth, label, scalar(tok))
	return nil
}

func object(b *strings.Builder, dec *json.Decoder, label string, depth int) error {
	inner := depth
	if label != "" {
		heading(b, depth, label)
		inner++
	}

	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return err
		}

		name, ok := key.(string)
		if !ok {
			return fmt.Errorf("object key is %T, not a string", key)
		}

		if err := value(b, dec, name, inner); err != nil {
			return err
		}
	}

	_, err := dec.Token() // closing brace
	return err
}

func array(b *strings.Builder, dec *json.Decoder, label string, depth int) error {
	titled := false
	i := 0

	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}

		d, nested := tok.(json.Delim)
		if !nested {
			if !titled {
				heading(b, depth, label)
				titled = true
			}

			bullet(b, depth+1, scalar(tok))
			i++
			continue
		}

		element := fmt.Sprintf("%s[%d]", label, i)
		b.WriteString("\r\n")

		switch d {
		case '{':
			err = object(b, dec, element, depth)
		case '[':
			err = array(b, dec, element, depth)
		default:
			err = fmt.Errorf("unexpected %q", d)
		}
		if err != nil {
			return err
		}

		i++
	}

	if i == 0 {
		heading(b, depth, label)
	}

	_, err := dec.Token() // closing bracket
	return err
}

func indent(depth int) string {
	return strings.Repeat(" ", depth*indentWidth)
}

func heading(b *strings.Builder, depth int, label string) {
	b.WriteString(indent(depth) + Found(Bold(Cyan(label))) + "\r\n")
}

func field(b *strings.Builder, depth int, label, val string) {
	line := Gray(label + ":")
	if val != "" {
		line += " " + White(val)
	}

	b.WriteString(indent(depth) + Found(line) + "\r\n")
}

func bullet(b *strings.Builder, depth int, val string) {
	b.WriteString(indent(depth) + Gray("- ") + White(val) + "\r\n")
}

func scalar(tok json.Token) string {
	switch v := tok.(type) {
	case nil:
		return "null"
	case string:
		return v
	case json.Number:
		return v.String()
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
