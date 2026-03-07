package worktreecolor

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestNormalize(t *testing.T) {
	if got := Normalize("  red "); got != "red" {
		t.Fatalf("expected red, got %q", got)
	}
	if got := Normalize("None"); got != "" {
		t.Fatalf("expected empty for None, got %q", got)
	}
}

func TestResolveEmptyAndNone(t *testing.T) {
	if c := Resolve(""); c != nil {
		t.Fatalf("expected nil for empty, got %v", c)
	}
	if c := Resolve("   "); c != nil {
		t.Fatalf("expected nil for whitespace, got %v", c)
	}
	if c := Resolve("none"); c != nil {
		t.Fatalf("expected nil for none, got %v", c)
	}
}

func TestResolveHex(t *testing.T) {
	c := Resolve("#ff0000")
	if c == nil {
		t.Fatal("expected non-nil for #ff0000")
	}
	r, g, b, _ := c.RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 {
		t.Fatalf("expected red, got R=%d G=%d B=%d", r>>8, g>>8, b>>8)
	}

	c = Resolve("#f00")
	if c == nil {
		t.Fatal("expected non-nil for #f00")
	}
	r, g, b, _ = c.RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 {
		t.Fatalf("expected red for #f00, got R=%d G=%d B=%d", r>>8, g>>8, b>>8)
	}
}

func TestResolvePaletteIndex(t *testing.T) {
	c := Resolve("0")
	if c == nil {
		t.Fatal("expected non-nil for 0")
	}
	r, g, b, _ := c.RGBA()
	if r != 0 || g != 0 || b != 0 {
		t.Fatalf("expected black for 0, got R=%d G=%d B=%d", r>>8, g>>8, b>>8)
	}

	c = Resolve("255")
	if c == nil {
		t.Fatal("expected non-nil for 255")
	}
	r, g, b, _ = c.RGBA()
	if r>>8 < 200 || g>>8 < 200 || b>>8 < 200 {
		t.Fatalf("expected bright grey for 255, got R=%d G=%d B=%d", r>>8, g>>8, b>>8)
	}
}

func TestResolveNamedColour(t *testing.T) {
	c := Resolve("red")
	if c == nil {
		t.Fatal("expected non-nil for red")
	}
	r, g, b, _ := c.RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 {
		t.Fatalf("expected red, got R=%d G=%d B=%d", r>>8, g>>8, b>>8)
	}

	if c := Resolve("Light Blue"); c == nil {
		t.Fatal("expected non-nil for Light Blue")
	}
}

func TestResolveNamedColourRendersANSISequence(t *testing.T) {
	rendered := lipgloss.NewStyle().Foreground(Resolve("coral")).Render("hello")
	if !strings.HasPrefix(rendered, "\x1b[") {
		t.Fatalf("expected ANSI prefix, got %q", rendered)
	}
}

func TestIsValid(t *testing.T) {
	if !IsValid("#abc") {
		t.Fatal("expected #abc to be valid")
	}
	if !IsValid("214") {
		t.Fatal("expected 214 to be valid")
	}
	if !IsValid("red") {
		t.Fatal("expected red to be valid")
	}
	if IsValid("") {
		t.Fatal("expected empty to be invalid")
	}
	if IsValid("notacolor") {
		t.Fatal("expected unknown name to be invalid")
	}
}

func TestIsCuratedValue(t *testing.T) {
	if !IsCuratedValue("red") {
		t.Fatal("expected red to be curated")
	}
	if IsCuratedValue("214") {
		t.Fatal("expected palette index 214 to be custom, not curated")
	}
	if IsCuratedValue("#ff0000") {
		t.Fatal("expected hex to be custom")
	}
}

func TestCuratedNamesReturnsCopy(t *testing.T) {
	names := CuratedNames()
	if len(names) == 0 {
		t.Fatal("expected curated names")
	}
	names[0] = "mutated"
	if CuratedNames()[0] == "mutated" {
		t.Fatal("expected CuratedNames to return a copy")
	}
}
