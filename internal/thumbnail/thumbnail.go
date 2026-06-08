// Package thumbnail generates and caches 300px-wide JPEG thumbnails.
package thumbnail

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

const thumbWidth = 300

// Path returns the on-disk path where the thumbnail for the given source
// filename is cached, rooted at mediaDir.
func Path(mediaDir, filename string) string {
	return filepath.Join(mediaDir, "thumbnails", filename+".jpg")
}

// Ensure generates a JPEG thumbnail for srcPath if the cached file at
// dstPath does not already exist. The thumbnail is scaled to thumbWidth
// pixels wide, preserving aspect ratio.
func Ensure(srcPath, dstPath string) error {
	if _, err := os.Stat(dstPath); err == nil {
		return nil
	}
	return Generate(srcPath, dstPath)
}

// Generate unconditionally (re)generates the thumbnail at dstPath from srcPath.
func Generate(srcPath, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create thumbnail dir: %w", err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	thumb := scale(img, thumbWidth)

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create thumbnail: %w", err)
	}
	defer dst.Close()

	if err := jpeg.Encode(dst, thumb, &jpeg.Options{Quality: 85}); err != nil {
		os.Remove(dstPath)
		return fmt.Errorf("encode thumbnail: %w", err)
	}

	return nil
}

func scale(src image.Image, width int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if srcW <= width {
		return src
	}

	height := (srcH * width) / srcW
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}
