package worktreecolor

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
)

// namedColors maps supported named colours (lowercase) to RGB.
var namedColors = map[string]color.RGBA{
	"red":            {R: 255, G: 0, B: 0, A: 255},
	"green":          {R: 0, G: 128, B: 0, A: 255},
	"blue":           {R: 0, G: 0, B: 255, A: 255},
	"yellow":         {R: 255, G: 255, B: 0, A: 255},
	"cyan":           {R: 0, G: 255, B: 255, A: 255},
	"magenta":        {R: 255, G: 0, B: 255, A: 255},
	"orange":         {R: 255, G: 165, B: 0, A: 255},
	"white":          {R: 255, G: 255, B: 255, A: 255},
	"black":          {R: 0, G: 0, B: 0, A: 255},
	"gray":           {R: 128, G: 128, B: 128, A: 255},
	"grey":           {R: 128, G: 128, B: 128, A: 255},
	"light red":      {R: 255, G: 102, B: 102, A: 255},
	"light green":    {R: 144, G: 238, B: 144, A: 255},
	"light blue":     {R: 173, G: 216, B: 230, A: 255},
	"light yellow":   {R: 255, G: 255, B: 224, A: 255},
	"light cyan":     {R: 224, G: 255, B: 255, A: 255},
	"light magenta":  {R: 255, G: 182, B: 193, A: 255},
	"dark red":       {R: 139, G: 0, B: 0, A: 255},
	"dark green":     {R: 0, G: 100, B: 0, A: 255},
	"dark blue":      {R: 0, G: 0, B: 139, A: 255},
	"dark yellow":    {R: 184, G: 134, B: 11, A: 255},
	"dark cyan":      {R: 0, G: 139, B: 139, A: 255},
	"dark magenta":   {R: 139, G: 0, B: 139, A: 255},
	"pink":           {R: 255, G: 192, B: 203, A: 255},
	"purple":         {R: 128, G: 0, B: 128, A: 255},
	"brown":          {R: 165, G: 42, B: 42, A: 255},
	"navy":           {R: 0, G: 0, B: 128, A: 255},
	"teal":           {R: 0, G: 128, B: 128, A: 255},
	"olive":          {R: 128, G: 128, B: 0, A: 255},
	"coral":          {R: 255, G: 127, B: 80, A: 255},
	"gold":           {R: 255, G: 215, B: 0, A: 255},
	"indigo":         {R: 75, G: 0, B: 130, A: 255},
	"violet":         {R: 238, G: 130, B: 238, A: 255},
	"salmon":         {R: 250, G: 128, B: 114, A: 255},
	"turquoise":      {R: 64, G: 224, B: 208, A: 255},
	"tan":            {R: 210, G: 180, B: 140, A: 255},
	"plum":           {R: 221, G: 160, B: 221, A: 255},
	"orchid":         {R: 218, G: 112, B: 214, A: 255},
	"crimson":        {R: 220, G: 20, B: 60, A: 255},
	"firebrick":      {R: 178, G: 34, B: 34, A: 255},
	"tomato":         {R: 255, G: 99, B: 71, A: 255},
	"chocolate":      {R: 210, G: 105, B: 30, A: 255},
	"sienna":         {R: 160, G: 82, B: 45, A: 255},
	"peru":           {R: 205, G: 133, B: 63, A: 255},
	"darkorange":     {R: 255, G: 140, B: 0, A: 255},
	"orangered":      {R: 255, G: 69, B: 0, A: 255},
	"lime":           {R: 0, G: 255, B: 0, A: 255},
	"springgreen":    {R: 0, G: 255, B: 127, A: 255},
	"mediumseagreen": {R: 60, G: 179, B: 113, A: 255},
	"seagreen":       {R: 46, G: 139, B: 87, A: 255},
	"dodgerblue":     {R: 30, G: 144, B: 255, A: 255},
	"slateblue":      {R: 106, G: 90, B: 205, A: 255},
	"mediumpurple":   {R: 147, G: 112, B: 219, A: 255},
	"mediumorchid":   {R: 186, G: 85, B: 211, A: 255},
	"hotpink":        {R: 255, G: 105, B: 180, A: 255},
	"deeppink":       {R: 255, G: 20, B: 147, A: 255},
	"aquamarine":     {R: 127, G: 255, B: 212, A: 255},
	"palegreen":      {R: 152, G: 251, B: 152, A: 255},
	"khaki":          {R: 240, G: 230, B: 140, A: 255},
	"lavender":       {R: 230, G: 230, B: 250, A: 255},
	"beige":          {R: 245, G: 245, B: 220, A: 255},
	"wheat":          {R: 245, G: 222, B: 179, A: 255},
	"mistyrose":      {R: 255, G: 228, B: 225, A: 255},
	"azure":          {R: 240, G: 255, B: 255, A: 255},
	"aliceblue":      {R: 240, G: 248, B: 255, A: 255},
	"ghostwhite":     {R: 248, G: 248, B: 255, A: 255},
	"white smoke":    {R: 245, G: 245, B: 245, A: 255},
	"whitesmoke":     {R: 245, G: 245, B: 245, A: 255},
	"gainsboro":      {R: 220, G: 220, B: 220, A: 255},
	"lightgray":      {R: 211, G: 211, B: 211, A: 255},
	"lightgrey":      {R: 211, G: 211, B: 211, A: 255},
	"silver":         {R: 192, G: 192, B: 192, A: 255},
	"darkgray":       {R: 169, G: 169, B: 169, A: 255},
	"darkgrey":       {R: 169, G: 169, B: 169, A: 255},
	"dimgray":        {R: 105, G: 105, B: 105, A: 255},
	"dimgrey":        {R: 105, G: 105, B: 105, A: 255},
}

// CuratedColor pairs a colour name with a human-readable description.
type CuratedColor struct {
	Name        string
	Description string
}

var curatedColors = []CuratedColor{
	{Name: "red", Description: "Bright red"},
	{Name: "green", Description: "Medium green"},
	{Name: "blue", Description: "Pure blue"},
	{Name: "yellow", Description: "Bright yellow"},
	{Name: "cyan", Description: "Bright blue-green"},
	{Name: "magenta", Description: "Bright pink-purple"},
	{Name: "orange", Description: "Warm orange"},
	{Name: "white", Description: "Pure white"},
	{Name: "black", Description: "Pure black"},
	{Name: "gray", Description: "Neutral mid-grey"},
	{Name: "pink", Description: "Soft pink"},
	{Name: "purple", Description: "Deep purple"},
	{Name: "brown", Description: "Earthy brown"},
	{Name: "navy", Description: "Dark blue"},
	{Name: "teal", Description: "Blue-green"},
	{Name: "olive", Description: "Dark yellowish-green"},
	{Name: "coral", Description: "Warm pinkish-orange"},
	{Name: "gold", Description: "Rich golden yellow"},
	{Name: "indigo", Description: "Deep blue-violet"},
	{Name: "violet", Description: "Soft violet"},
	{Name: "salmon", Description: "Soft pinkish-orange"},
	{Name: "turquoise", Description: "Vivid blue-green"},
	{Name: "lime", Description: "Bright green"},
	{Name: "light red", Description: "Soft red"},
	{Name: "light green", Description: "Pale green"},
	{Name: "light blue", Description: "Pale blue"},
	{Name: "light yellow", Description: "Pale yellow"},
	{Name: "dark red", Description: "Deep maroon"},
	{Name: "dark green", Description: "Forest green"},
	{Name: "dark blue", Description: "Deep blue"},
	{Name: "crimson", Description: "Deep dark red"},
	{Name: "tomato", Description: "Warm red-orange"},
	{Name: "orangered", Description: "Strong red-orange"},
	{Name: "darkorange", Description: "Deep orange"},
	{Name: "springgreen", Description: "Vivid green"},
	{Name: "seagreen", Description: "Muted ocean green"},
	{Name: "dodgerblue", Description: "Vivid sky blue"},
	{Name: "slateblue", Description: "Muted blue-violet"},
	{Name: "mediumpurple", Description: "Soft purple"},
	{Name: "hotpink", Description: "Vivid pink"},
	{Name: "deeppink", Description: "Intense pink"},
	{Name: "aquamarine", Description: "Light blue-green"},
	{Name: "palegreen", Description: "Very light green"},
	{Name: "khaki", Description: "Warm sandy yellow"},
	{Name: "lavender", Description: "Soft blue-purple"},
	{Name: "wheat", Description: "Warm pale tan"},
	{Name: "silver", Description: "Light grey"},
}

var curatedNameSet = buildCuratedNameSet()

// CuratedColors returns a copy of the curated colours shown in the picker.
func CuratedColors() []CuratedColor {
	out := make([]CuratedColor, len(curatedColors))
	copy(out, curatedColors)
	return out
}

// CuratedNames returns a copy of the curated colour names.
func CuratedNames() []string {
	out := make([]string, len(curatedColors))
	for i, c := range curatedColors {
		out[i] = c.Name
	}
	return out
}

func buildCuratedNameSet() map[string]struct{} {
	curated := make(map[string]struct{}, len(curatedColors))
	for _, c := range curatedColors {
		curated[c.Name] = struct{}{}
	}
	return curated
}

// Normalize trims a stored colour string and treats "none" as empty.
func Normalize(s string) string {
	trimmed := strings.TrimSpace(s)
	if strings.EqualFold(trimmed, "none") {
		return ""
	}
	return trimmed
}

// IsCuratedValue reports whether the value can be selected directly from the picker.
func IsCuratedValue(s string) bool {
	s = Normalize(s)
	if s == "" {
		return false
	}
	_, ok := curatedNameSet[strings.ToLower(s)]
	return ok
}

// IsValid reports whether the value is a supported non-empty worktree colour.
func IsValid(s string) bool {
	return Resolve(s) != nil
}

var resolveCache sync.Map // string → resolvedEntry

type resolvedEntry struct{ c color.Color }

// Resolve converts a stored colour string (hex, 256 index, or supported name) to a color.Color.
// It returns nil for empty, "none", or invalid values.
func Resolve(s string) color.Color {
	s = Normalize(s)
	if s == "" {
		return nil
	}
	if v, ok := resolveCache.Load(s); ok {
		return v.(resolvedEntry).c
	}
	c := resolve(s)
	resolveCache.Store(s, resolvedEntry{c: c})
	return c
}

func resolve(s string) color.Color {
	if validateHex(s) {
		hex := s[1:]
		if len(hex) == 3 {
			hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
		}
		return lipgloss.Color("#" + hex)
	}
	if n, ok := parsePaletteIndex(s); ok {
		return lipgloss.Color(strconv.Itoa(n))
	}
	if c, ok := namedColors[strings.ToLower(s)]; ok {
		return lipgloss.Color(rgbHex(c))
	}
	return nil
}

func parsePaletteIndex(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 255 {
		return 0, false
	}
	return n, true
}

func validateHex(s string) bool {
	if s == "" || len(s) < 4 || s[0] != '#' {
		return false
	}
	hex := s[1:]
	if len(hex) != 3 && len(hex) != 6 {
		return false
	}
	return isHexDigits(hex)
}

func isHexDigits(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func rgbHex(c color.RGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}
