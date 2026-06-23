package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultAvatarMaxBytes = 256 * 1024
	defaultAvatarMaxDim   = 64
	// avatarRenderVersion is folded into the cache key so that changes to the
	// avatar normalisation/rendering (e.g. the circular mask) invalidate
	// previously cached PNGs instead of serving stale square images.
	avatarRenderVersion = "v2-round"
)

// AvatarImage is a terminal-ready PNG representation of an author avatar.
type AvatarImage struct {
	URL string
	Key string
	PNG []byte
}

// AvatarCache downloads and stores small PNG avatars for terminal rendering.
type AvatarCache struct {
	dir      string
	client   *http.Client
	maxBytes int64
	maxDim   int
}

// NewAvatarCache creates an avatar cache rooted at dir.
func NewAvatarCache(dir string) *AvatarCache {
	return &AvatarCache{
		dir: dir,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		maxBytes: defaultAvatarMaxBytes,
		maxDim:   defaultAvatarMaxDim,
	}
}

// NewDefaultAvatarCache creates the default lazyworktree avatar cache.
func NewDefaultAvatarCache() *AvatarCache {
	base, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(base) == "" {
		base = os.TempDir()
	}
	return NewAvatarCache(filepath.Join(base, "lazyworktree", "avatars"))
}

// Fetch returns cached PNG avatar data, downloading and normalising it when needed.
func (c *AvatarCache) Fetch(ctx context.Context, rawURL string) (*AvatarImage, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("avatar URL is empty")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid avatar URL: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("unsupported avatar URL scheme %q", u.Scheme)
	}

	key := avatarCacheKey(rawURL)
	cachePath := filepath.Join(c.dir, key+".png")
	// #nosec G304 -- cachePath is a SHA-256 derived filename under the avatar cache directory.
	if data, err := os.ReadFile(cachePath); err == nil && len(data) > 0 {
		return &AvatarImage{URL: rawURL, Key: key, PNG: data}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create avatar request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download avatar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download avatar: HTTP %d", resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" &&
		!strings.Contains(contentType, "image/png") &&
		!strings.Contains(contentType, "image/jpeg") &&
		!strings.Contains(contentType, "image/jpg") &&
		!strings.Contains(contentType, "image/gif") &&
		!strings.Contains(contentType, "application/octet-stream") {
		return nil, fmt.Errorf("unsupported avatar content type %q", contentType)
	}

	limited := io.LimitReader(resp.Body, c.maxBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read avatar: %w", err)
	}
	if int64(len(raw)) > c.maxBytes {
		return nil, fmt.Errorf("avatar exceeds %d bytes", c.maxBytes)
	}

	pngData, err := normaliseAvatarPNG(raw, c.maxDim)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.dir, 0o750); err != nil {
		return nil, fmt.Errorf("create avatar cache: %w", err)
	}
	if err := os.WriteFile(cachePath, pngData, 0o644); err != nil {
		return nil, fmt.Errorf("write avatar cache: %w", err)
	}
	return &AvatarImage{URL: rawURL, Key: key, PNG: pngData}, nil
}

func avatarCacheKey(rawURL string) string {
	sum := sha256.Sum256([]byte(avatarRenderVersion + "\x00" + rawURL))
	return hex.EncodeToString(sum[:])
}

func normaliseAvatarPNG(raw []byte, maxDim int) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode avatar: %w", err)
	}
	switch format {
	case "png", "jpeg", "gif":
	default:
		return nil, fmt.Errorf("unsupported avatar image format %q", format)
	}
	if maxDim > 0 {
		img = resizeNearest(img, maxDim)
	}
	img = circleMask(img)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("encode avatar PNG: %w", err)
	}
	return out.Bytes(), nil
}

// circleMask returns an NRGBA copy of src with a circular alpha mask applied:
// pixels outside the inscribed circle become fully transparent and the edge is
// anti-aliased, yielding a stylish round avatar with transparent corners.
func circleMask(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}

	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	cx := float64(w) / 2
	cy := float64(h) / 2
	radius := math.Min(cx, cy)

	for y := range h {
		for x := range w {
			c := color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			// Distance from pixel centre to circle centre.
			dx := (float64(x) + 0.5) - cx
			dy := (float64(y) + 0.5) - cy
			dist := math.Hypot(dx, dy)

			// Anti-alias a 1px band at the circle edge.
			coverage := radius - dist + 0.5
			switch {
			case coverage >= 1:
				coverage = 1
			case coverage <= 0:
				coverage = 0
			}

			c.A = uint8(float64(c.A) * coverage)
			dst.SetNRGBA(x, y, c)
		}
	}
	return dst
}

func resizeNearest(src image.Image, maxDim int) image.Image {
	b := src.Bounds()
	width := b.Dx()
	height := b.Dy()
	if width <= 0 || height <= 0 || (width <= maxDim && height <= maxDim) {
		return src
	}

	dstW, dstH := maxDim, maxDim
	if width >= height {
		dstH = max(1, height*maxDim/width)
	} else {
		dstW = max(1, width*maxDim/height)
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := range dstH {
		srcY := b.Min.Y + y*height/dstH
		for x := range dstW {
			srcX := b.Min.X + x*width/dstW
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func init() {
	image.RegisterFormat("jpeg", "\xff\xd8", jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("png", "\x89PNG\r\n\x1a\n", png.Decode, png.DecodeConfig)
	image.RegisterFormat("gif", "GIF8?a", gif.Decode, gif.DecodeConfig)
}
