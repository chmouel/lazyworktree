package services

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestAvatarCacheFetchDownloadsAndCachesPNG(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(testPNG(t))
	}))
	defer server.Close()

	cache := NewAvatarCache(t.TempDir())
	avatar, err := cache.Fetch(context.Background(), server.URL+"/avatar.png")
	if err != nil {
		t.Fatalf("fetch avatar: %v", err)
	}
	if avatar == nil || len(avatar.PNG) == 0 {
		t.Fatal("expected PNG avatar data")
	}
	if requests != 1 {
		t.Fatalf("expected one HTTP request, got %d", requests)
	}

	avatar, err = cache.Fetch(context.Background(), server.URL+"/avatar.png")
	if err != nil {
		t.Fatalf("fetch cached avatar: %v", err)
	}
	if avatar == nil || len(avatar.PNG) == 0 {
		t.Fatal("expected cached PNG avatar data")
	}
	if requests != 1 {
		t.Fatalf("expected cache hit without another request, got %d requests", requests)
	}
}

func TestAvatarCacheFetchRejectsUnsupportedContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not an image"))
	}))
	defer server.Close()

	cache := NewAvatarCache(t.TempDir())
	_, err := cache.Fetch(context.Background(), server.URL+"/avatar.txt")
	if err == nil {
		t.Fatal("expected unsupported content type error")
	}
}

func TestAvatarCacheFetchRejectsLargeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(bytes.Repeat([]byte("x"), 16))
	}))
	defer server.Close()

	cache := NewAvatarCache(t.TempDir())
	cache.maxBytes = 8
	_, err := cache.Fetch(context.Background(), server.URL+"/avatar.png")
	if err == nil {
		t.Fatal("expected size limit error")
	}
}

func TestAvatarCacheFetchReadsExistingCache(t *testing.T) {
	cache := NewAvatarCache(t.TempDir())
	rawURL := "https://example.com/alice.png"
	key := avatarCacheKey(rawURL)
	if err := os.MkdirAll(cache.dir, 0o750); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cache.dir, key+".png"), testPNG(t), 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	avatar, err := cache.Fetch(context.Background(), rawURL)
	if err != nil {
		t.Fatalf("fetch cached avatar: %v", err)
	}
	if avatar.Key != key {
		t.Fatalf("expected key %q, got %q", key, avatar.Key)
	}
}

func TestCircleMaskMakesCornersTransparent(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			src.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}

	masked := circleMask(src)

	// Corners fall outside the inscribed circle and must be fully transparent.
	corners := [][2]int{{0, 0}, {63, 0}, {0, 63}, {63, 63}}
	for _, c := range corners {
		if _, _, _, a := masked.At(c[0], c[1]).RGBA(); a != 0 {
			t.Fatalf("corner (%d,%d) expected transparent, got alpha %d", c[0], c[1], a>>8)
		}
	}

	// Centre lies inside the circle and must remain fully opaque.
	if _, _, _, a := masked.At(32, 32).RGBA(); a>>8 != 255 {
		t.Fatalf("centre expected opaque, got alpha %d", a>>8)
	}
}

func TestNormaliseAvatarPNGAppliesCircleMask(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			src.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode src png: %v", err)
	}

	out, err := normaliseAvatarPNG(buf.Bytes(), defaultAvatarMaxDim)
	if err != nil {
		t.Fatalf("normalise avatar: %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode normalised png: %v", err)
	}
	b := decoded.Bounds()
	if _, _, _, a := decoded.At(b.Min.X, b.Min.Y).RGBA(); a != 0 {
		t.Fatalf("top-left corner expected transparent after normalisation, got alpha %d", a>>8)
	}
}
