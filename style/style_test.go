package style

import (
	"strings"
	"testing"
)

func withEnabled(t *testing.T, want bool, fn func()) {
	t.Helper()
	prev := enabled
	enabled = want
	defer func() { enabled = prev }()
	fn()
}

func TestDisabledIsPlain(t *testing.T) {
	withEnabled(t, false, func() {
		got := Red("boom")
		if got != "boom" {
			t.Fatalf("Red with styling off = %q, want plain %q", got, "boom")
		}
		if strings.Contains(Prompt(), "\033") {
			t.Fatalf("Prompt emitted escape codes while styling disabled: %q", Prompt())
		}
	})
}

func TestEnabledWraps(t *testing.T) {
	withEnabled(t, true, func() {
		got := Red("boom")
		if !strings.HasPrefix(got, CodeRed) || !strings.HasSuffix(got, Reset) {
			t.Fatalf("Red = %q, want wrapped in color + reset", got)
		}
	})
}

func TestSemanticPrefixes(t *testing.T) {
	withEnabled(t, false, func() {
		cases := map[string]string{
			"✗ ":   Error("x"),
			"✓ ":   Success("x"),
			"» ":   Info("x"),
			"[+] ": Found("x"),
			"[-] ": Failure("x"),
			"[!] ": Alert("x"),
		}
		for prefix, out := range cases {
			if !strings.HasPrefix(out, prefix) {
				t.Errorf("output %q missing prefix %q", out, prefix)
			}
		}
	})
}

func TestWidthStripsANSI(t *testing.T) {
	if got := Width("\x1b[31mabc\x1b[0m"); got != 3 {
		t.Fatalf("Width of styled %q = %d, want 3", "abc", got)
	}
	if got := Width("héllo"); got != 5 {
		t.Fatalf("Width of multibyte string = %d, want 5", got)
	}
}

func TestSectionBracketsTheTitle(t *testing.T) {
	withEnabled(t, false, func() {
		body := "  A      1.2.3.4"
		out := Section("DNS", body)

		lines := strings.Split(strings.TrimSpace(out), "\r\n")
		if lines[0] != "[DNS]" {
			t.Fatalf("first line = %q, want a bracketed title", lines[0])
		}
		if lines[1] != body {
			t.Fatalf("body line = %q, want it rendered verbatim", lines[1])
		}
	})
}

func TestSectionPadsWithBlankLines(t *testing.T) {
	withEnabled(t, false, func() {
		out := Section("DNS")

		if !strings.HasPrefix(out, "\r\n") || !strings.HasSuffix(out, "\r\n\r\n") {
			t.Fatalf("Section = %q, want it padded away from the surrounding output", out)
		}
	})
}

func TestFieldPadsLabel(t *testing.T) {
	withEnabled(t, false, func() {
		if got := Field("A", 6, "1.2.3.4"); got != "  A      1.2.3.4" {
			t.Fatalf("Field = %q, want the label padded into a column", got)
		}
	})
}

func TestRenderScalarsKeepTheirJSONForm(t *testing.T) {
	withEnabled(t, false, func() {
		got := Render([]byte(`{"host":"a.io","port":443,"up":true,"note":null}`))
		want := "[+] host: a.io\r\n[+] port: 443\r\n[+] up: true\r\n[+] note: null\r\n"

		if got != want {
			t.Fatalf("Render = %q, want %q", got, want)
		}
	})
}

func TestRenderIndentsNestedObjectsAndBullets(t *testing.T) {
	withEnabled(t, false, func() {
		got := Render([]byte(`{"dns":{"a":["1.1.1.1","8.8.8.8"]}}`))
		want := "[+] dns\r\n" +
			"    [+] a\r\n" +
			"        - 1.1.1.1\r\n" +
			"        - 8.8.8.8\r\n"

		if got != want {
			t.Fatalf("Render = %q, want each level indented one step further:\n%q", got, want)
		}
	})
}

func TestRenderNumbersLargerThanFloatPrecision(t *testing.T) {
	withEnabled(t, false, func() {
		// Decoding into any would round this through float64; UseNumber must not.
		got := Render([]byte(`{"id":12345678901234567890}`))

		if !strings.Contains(got, "12345678901234567890") {
			t.Fatalf("Render = %q, want the number carried through verbatim", got)
		}
	})
}

func TestRenderIndexesArrayElements(t *testing.T) {
	withEnabled(t, false, func() {
		got := Render([]byte(`{"hosts":[{"name":"x"},{"name":"y"}]}`))

		for _, want := range []string{"[+] hosts[0]", "    [+] name: x", "[+] hosts[1]", "    [+] name: y"} {
			if !strings.Contains(got, want) {
				t.Errorf("Render = %q, missing %q", got, want)
			}
		}
	})
}

func TestRenderEmptyArrayKeepsItsHeading(t *testing.T) {
	withEnabled(t, false, func() {
		if got := Render([]byte(`{"empty":[]}`)); got != "[+] empty\r\n" {
			t.Fatalf("Render = %q, want a bare heading for the empty array", got)
		}
	})
}

func TestRenderReportsTruncatedJSON(t *testing.T) {
	withEnabled(t, false, func() {
		got := Render([]byte(`{"broken":`))

		if !strings.HasPrefix(got, "[-] ") || !strings.HasSuffix(got, "\r\n") {
			t.Fatalf("Render = %q, want a failure line for unreadable input", got)
		}
	})
}
