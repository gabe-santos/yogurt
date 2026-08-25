package icon

import (
	"image"
	"image/color"
)

// downscale resizes img to fit within an edge x edge box, preserving aspect
// ratio, using box-filtered averaging: each destination pixel is the mean of
// the source pixels it covers, which is cheap and avoids the ringing a naive
// nearest-neighbour resize would give a mostly-flat icon. It stays pure Go
// with no cgo, per ADR-0006.
func downscale(img image.Image, edge int) image.Image {
	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return image.NewNRGBA(image.Rect(0, 0, edge, edge))
	}

	dstW, dstH := edge, edge
	if srcW > srcH {
		dstH = max(1, edge*srcH/srcW)
	} else if srcH > srcW {
		dstW = max(1, edge*srcW/srcH)
	}

	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	for y := range dstH {
		srcY0 := bounds.Min.Y + y*srcH/dstH
		srcY1 := bounds.Min.Y + (y+1)*srcH/dstH
		if srcY1 <= srcY0 {
			srcY1 = srcY0 + 1
		}
		for x := range dstW {
			srcX0 := bounds.Min.X + x*srcW/dstW
			srcX1 := bounds.Min.X + (x+1)*srcW/dstW
			if srcX1 <= srcX0 {
				srcX1 = srcX0 + 1
			}
			dst.SetNRGBA(x, y, averageBox(img, srcX0, srcY0, srcX1, srcY1))
		}
	}
	return dst
}

// averageBox returns the mean colour of img over [x0,x1) x [y0,y1), as a
// straight-alpha NRGBA. img.At returns alpha-premultiplied 16-bit channels;
// premultiplied channels are averaged first and the result is unpremultiplied
// once, rather than averaging already-unpremultiplied colours, so a
// half-transparent pixel does not bleed its hidden colour into the mean.
func averageBox(img image.Image, x0, y0, x1, y1 int) color.NRGBA {
	var r, g, b, a, n uint32
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			pr, pg, pb, pa := img.At(x, y).RGBA()
			r += pr
			g += pg
			b += pb
			a += pa
			n++
		}
	}
	if n == 0 {
		n = 1
	}
	r, g, b, a = r/n, g/n, b/n, a/n
	if a == 0 {
		return color.NRGBA{}
	}
	return color.NRGBA{
		R: uint8(min(255, r*255/a)),
		G: uint8(min(255, g*255/a)),
		B: uint8(min(255, b*255/a)),
		A: uint8(a >> 8),
	}
}
