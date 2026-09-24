package icon_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/url"
	"testing"

	"github.com/gabe-santos/yogurt/internal/icon"
)

// fakeFetcher answers Fetch from an in-memory map, so tests exercise Choose
// with no real network access.
type fakeFetcher struct {
	bodies map[string][]byte
	fail   map[string]bool
}

func (f *fakeFetcher) Fetch(_ context.Context, rawURL string) ([]byte, error) {
	if f.fail[rawURL] {
		return nil, errors.New("fetch failed")
	}
	body, ok := f.bodies[rawURL]
	if !ok {
		return nil, errors.New("not found")
	}
	return body, nil
}

func mustBase(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse base: %v", err)
	}
	return u
}

// pngBytes renders a solid-colour square PNG of the given edge length, the
// smallest realistic stand-in for a publisher's raster icon.
func pngBytes(t *testing.T, edge int, c color.Color) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, edge, edge))
	for y := range edge {
		for x := range edge {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode fixture png: %v", err)
	}
	return buf.Bytes()
}

func TestChoosePrefersSVGOverEverything(t *testing.T) {
	html := []byte(`<html><head>
		<link rel="icon" type="image/svg+xml" href="/icon.svg">
		<link rel="icon" type="image/png" sizes="32x32" href="/icon-32.png">
	</head></html>`)
	fetcher := &fakeFetcher{bodies: map[string][]byte{
		"https://example.com/icon.svg":    []byte(`<svg></svg>`),
		"https://example.com/icon-32.png": pngBytes(t, 32, color.White),
	}}

	got, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/"))
	if !ok {
		t.Fatal("expected an icon")
	}
	if got.MediaType != "image/svg+xml" {
		t.Errorf("media type = %q, want image/svg+xml", got.MediaType)
	}
	if string(got.Data) != `<svg></svg>` {
		t.Errorf("SVG was not stored byte-for-byte: %q", got.Data)
	}
}

func TestChoosePrefersSmallestExactRasterOverLarger(t *testing.T) {
	html := []byte(`<html><head>
		<link rel="icon" type="image/png" sizes="192x192" href="/icon-192.png">
		<link rel="icon" type="image/png" sizes="32x32" href="/icon-32.png">
	</head></html>`)
	fetcher := &fakeFetcher{bodies: map[string][]byte{
		"https://example.com/icon-192.png": pngBytes(t, 192, color.RGBA{R: 255, A: 255}),
		"https://example.com/icon-32.png":  pngBytes(t, 32, color.RGBA{G: 255, A: 255}),
	}}

	got, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/"))
	if !ok {
		t.Fatal("expected an icon")
	}
	img, _, err := image.Decode(bytes.NewReader(got.Data))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 32 || b.Dy() != 32 {
		t.Errorf("bounds = %v, want 32x32 (the already-32 candidate, not the downscaled 192)", b)
	}
	r, g, _, _ := img.At(0, 0).RGBA()
	if r != 0 || g == 0 {
		t.Errorf("chose the 192px candidate instead of the 32px one")
	}
}

func TestChoosePrefersThirtyTwoOverASmallerDeclaredSize(t *testing.T) {
	// A very common pairing: a site declares both a 16x16 and a 32x32 icon.
	// Choosing the 16 would ship a blurry icon on exactly the high-density
	// displays the 32px target exists for.
	html := []byte(`<html><head>
		<link rel="icon" type="image/png" sizes="16x16" href="/icon-16.png">
		<link rel="icon" type="image/png" sizes="32x32" href="/icon-32.png">
	</head></html>`)
	fetcher := &fakeFetcher{bodies: map[string][]byte{
		"https://example.com/icon-16.png": pngBytes(t, 16, color.RGBA{R: 255, A: 255}),
		"https://example.com/icon-32.png": pngBytes(t, 32, color.RGBA{G: 255, A: 255}),
	}}

	got, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/"))
	if !ok {
		t.Fatal("expected an icon")
	}
	img, _, err := image.Decode(bytes.NewReader(got.Data))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 32 || b.Dy() != 32 {
		t.Errorf("bounds = %v, want the 32px candidate, not the smaller declared one", b)
	}
	r, g, _, _ := img.At(0, 0).RGBA()
	if r != 0 || g == 0 {
		t.Errorf("chose the 16px candidate instead of the 32px one")
	}
}

func TestChooseFallsBackToICO(t *testing.T) {
	html := []byte(`<html><head>
		<link rel="shortcut icon" href="/favicon.ico">
	</head></html>`)
	fetcher := &fakeFetcher{bodies: map[string][]byte{
		"https://example.com/favicon.ico": []byte("\x00\x00fake-ico-bytes"),
	}}

	got, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/"))
	if !ok {
		t.Fatal("expected an icon")
	}
	if got.MediaType != "image/x-icon" {
		t.Errorf("media type = %q, want image/x-icon", got.MediaType)
	}
	if string(got.Data) != "\x00\x00fake-ico-bytes" {
		t.Errorf("ICO was not stored byte-for-byte")
	}
}

func TestChooseFallsBackToOpenGraphImage(t *testing.T) {
	html := []byte(`<html><head>
		<meta property="og:image" content="/social.png">
	</head></html>`)
	fetcher := &fakeFetcher{bodies: map[string][]byte{
		"https://example.com/social.png": pngBytes(t, 64, color.Black),
	}}

	got, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/"))
	if !ok {
		t.Fatal("expected an icon")
	}
	img, _, err := image.Decode(bytes.NewReader(got.Data))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 32 || b.Dy() != 32 {
		t.Errorf("bounds = %v, want downscaled to 32x32", b)
	}
}

func TestChooseDownscalesLargeRaster(t *testing.T) {
	html := []byte(`<html><head><link rel="icon" href="/icon.png"></head></html>`)
	fetcher := &fakeFetcher{bodies: map[string][]byte{
		"https://example.com/icon.png": pngBytes(t, 170, color.RGBA{B: 255, A: 255}),
	}}

	got, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/"))
	if !ok {
		t.Fatal("expected an icon")
	}
	if len(got.Data) >= len(pngBytes(t, 170, color.RGBA{B: 255, A: 255})) {
		t.Error("downscaled icon is not smaller than the original")
	}
	img, _, err := image.Decode(bytes.NewReader(got.Data))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 32 || b.Dy() != 32 {
		t.Errorf("bounds = %v, want 32x32", b)
	}
}

func TestChooseKeepsSmallRasterUnmodified(t *testing.T) {
	original := pngBytes(t, 16, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	html := []byte(`<html><head><link rel="icon" href="/icon.png"></head></html>`)
	fetcher := &fakeFetcher{bodies: map[string][]byte{
		"https://example.com/icon.png": original,
	}}

	got, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/"))
	if !ok {
		t.Fatal("expected an icon")
	}
	if !bytes.Equal(got.Data, original) {
		t.Error("a raster already at or under 32px should be stored unmodified")
	}
}

func TestChooseNeverGuessesConventionalPaths(t *testing.T) {
	html := []byte(`<html><head></head><body>no icon links here</body></html>`)
	fetcher := &fakeFetcher{bodies: map[string][]byte{
		// If Choose ever guessed /favicon.ico, this would satisfy it. It must
		// not: the site declared nothing.
		"https://example.com/favicon.ico": []byte("\x00\x00should never be fetched"),
	}}

	if _, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/")); ok {
		t.Fatal("expected no icon: conventional path guessing must never be attempted")
	}
}

func TestChooseNoIconLinksReportsNoIcon(t *testing.T) {
	html := []byte(`<html><head><title>No icons here</title></head></html>`)
	fetcher := &fakeFetcher{}

	if _, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/")); ok {
		t.Fatal("expected no icon")
	}
}

func TestChooseMalformedHTMLReportsNoIcon(t *testing.T) {
	html := []byte(`<html><head><link rel="icon" href="/icon.png"`) // truncated, unclosed tag
	fetcher := &fakeFetcher{}

	if _, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/")); ok {
		t.Fatal("expected no icon from malformed HTML")
	}
}

func TestChooseUndecodableImageFallsThrough(t *testing.T) {
	html := []byte(`<html><head>
		<link rel="icon" type="image/png" href="/broken.png">
		<meta property="og:image" content="/social.png">
	</head></html>`)
	fetcher := &fakeFetcher{bodies: map[string][]byte{
		"https://example.com/broken.png": []byte("this is not a real image"),
		"https://example.com/social.png": pngBytes(t, 32, color.White),
	}}

	got, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/"))
	if !ok {
		t.Fatal("expected the undecodable candidate to be skipped, not to fail the whole choice")
	}
	if got.MediaType != "image/png" {
		t.Errorf("media type = %q, want image/png", got.MediaType)
	}
}

func TestChooseUnreachableCandidateFallsThrough(t *testing.T) {
	html := []byte(`<html><head>
		<link rel="icon" type="image/png" href="/unreachable.png">
		<link rel="shortcut icon" href="/favicon.ico">
	</head></html>`)
	fetcher := &fakeFetcher{
		bodies: map[string][]byte{"https://example.com/favicon.ico": []byte("ico-bytes")},
		fail:   map[string]bool{"https://example.com/unreachable.png": true},
	}

	got, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/"))
	if !ok {
		t.Fatal("expected fallback to the ICO after the raster failed to fetch")
	}
	if got.MediaType != "image/x-icon" {
		t.Errorf("media type = %q, want image/x-icon", got.MediaType)
	}
}

func TestChooseRefusesOversizedBody(t *testing.T) {
	oversized := make([]byte, icon.MaxBodySize+1)
	html := []byte(`<html><head><link rel="icon" href="/huge.png"></head></html>`)
	fetcher := &fakeFetcher{bodies: map[string][]byte{"https://example.com/huge.png": oversized}}

	if _, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/")); ok {
		t.Fatal("expected an oversized body to be refused")
	}
}

func TestChooseResolvesRelativeHrefsAgainstSiteURL(t *testing.T) {
	html := []byte(`<html><head><link rel="icon" href="assets/icon.png"></head></html>`)
	fetcher := &fakeFetcher{bodies: map[string][]byte{
		"https://example.com/site/assets/icon.png": pngBytes(t, 32, color.White),
	}}

	got, ok := icon.Choose(t.Context(), fetcher, html, mustBase(t, "https://example.com/site/"))
	if !ok {
		t.Fatal("expected the relative href to resolve against the site URL")
	}
	if got.MediaType != "image/png" {
		t.Errorf("media type = %q, want image/png", got.MediaType)
	}
}
