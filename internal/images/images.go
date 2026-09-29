// Package images handles upload decoding and thumbnail generation.
// Thumbnails are capped at 480px on the long edge and encoded as JPEG
// so card grids never download full-size originals.
package images

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif" // first frame
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
)

// Limits.
const (
	MaxUploadBytes = 12 << 20 // 12MB
	MaxDimension   = 8000     // decode guard
	ThumbEdge      = 480      // thumbnail long edge
	ThumbQuality   = 80
)

// Processed is a decoded upload ready for storage.
type Processed struct {
	ContentType string // canonical stored type ("image/jpeg" or original for gif/png passthrough)
	Width       int
	Height      int
	Blob        []byte // original bytes
	Thumb       []byte // JPEG thumbnail bytes
	ThumbWidth  int
	ThumbHeight int
}

// Process decodes raw upload bytes, validates them, and builds a JPEG
// thumbnail. GIF/PNG originals are kept byte-identical; thumbnails are
// always JPEG for size.
func Process(raw []byte, claimedType string) (Processed, error) {
	if len(raw) == 0 || len(raw) > MaxUploadBytes {
		return Processed{}, fmt.Errorf("image must be 1B..12MB")
	}
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return Processed{}, fmt.Errorf("unsupported image (jpeg/png/gif only): %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || w > MaxDimension || h > MaxDimension {
		return Processed{}, fmt.Errorf("bad image dimensions %dx%d", w, h)
	}
	tw, th := thumbSize(w, h, ThumbEdge)
	thumb := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.CatmullRom.Scale(thumb, thumb.Bounds(), img, b, draw.Over, nil)
	var tbuf bytes.Buffer
	if err := jpeg.Encode(&tbuf, thumb, &jpeg.Options{Quality: ThumbQuality}); err != nil {
		return Processed{}, err
	}
	ct := claimedType
	switch format {
	case "jpeg":
		ct = "image/jpeg"
	case "png":
		ct = "image/png"
	case "gif":
		ct = "image/gif"
	}
	return Processed{
		ContentType: ct, Width: w, Height: h, Blob: raw,
		Thumb: tbuf.Bytes(), ThumbWidth: tw, ThumbHeight: th,
	}, nil
}

func thumbSize(w, h, edge int) (int, int) {
	if w <= edge && h <= edge {
		return w, h
	}
	if w >= h {
		return edge, h * edge / w
	}
	return w * edge / h, edge
}
