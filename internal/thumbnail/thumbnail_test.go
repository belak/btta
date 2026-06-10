package thumbnail

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 0, 255})
		}
	}
	f, err := os.Create(path)
	assert.NoError(t, err)
	defer f.Close()
	assert.NoError(t, png.Encode(f, img))
}

func decodeConfig(t *testing.T, path string) image.Config {
	t.Helper()
	f, err := os.Open(path)
	assert.NoError(t, err)
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	assert.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	return cfg
}

func TestGenerateScaling(t *testing.T) {
	tests := []struct {
		name         string
		srcW, srcH   int
		wantW, wantH int
	}{
		{"downscales wide image preserving aspect", 600, 300, thumbWidth, 150},
		{"does not upscale narrow image", 100, 50, 100, 50},
		{"scales to exact thumbWidth", 900, 300, thumbWidth, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "src.png")
			writePNG(t, src, tt.srcW, tt.srcH)
			dst := filepath.Join(dir, "out.jpg")

			assert.NoError(t, Generate(src, dst))

			cfg := decodeConfig(t, dst)
			assert.Equal(t, tt.wantW, cfg.Width)
			assert.Equal(t, tt.wantH, cfg.Height)
		})
	}
}

func TestEnsureSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.png")
	writePNG(t, src, 600, 300)
	dst := filepath.Join(dir, "out.jpg")

	// A pre-existing thumbnail must not be regenerated.
	assert.NoError(t, os.WriteFile(dst, []byte("sentinel"), 0o644))
	assert.NoError(t, Ensure(src, dst))

	got, err := os.ReadFile(dst)
	assert.NoError(t, err)
	assert.Equal(t, "sentinel", string(got))
}

func TestGenerateBadSourceLeavesNoFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "notimage.txt")
	assert.NoError(t, os.WriteFile(src, []byte("not an image"), 0o644))
	dst := filepath.Join(dir, "out.jpg")

	assert.Error(t, Generate(src, dst))
	_, err := os.Stat(dst)
	assert.True(t, os.IsNotExist(err))
}
