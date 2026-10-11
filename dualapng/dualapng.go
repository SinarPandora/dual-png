// Package dualapng writes a "dual-image" APNG: static PNG decoders show
// image A (the default IDAT image, which is not part of the animation),
// while APNG-aware viewers play frames B and a transparent overlay.
package dualapng

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"

	xdraw "golang.org/x/image/draw"
)

// Padding selects what fills the letterbox area when B's aspect ratio
// differs from A's.
type Padding int

const (
	PadTransparent Padding = iota
	PadBlack
)

// Load decodes a PNG or JPEG file.
func Load(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// Fit scales src to fit inside w×h keeping its aspect ratio, centered,
// with the remaining area filled per pad.
func Fit(src image.Image, w, h int, pad Padding) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	if pad == PadBlack {
		xdraw.Draw(dst, dst.Bounds(), image.NewUniform(color.Black), image.Point{}, xdraw.Src)
	}
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	nw, nh := w, sh*w/sw
	if nh > h {
		nw, nh = sw*h/sh, h
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	x0, y0 := (w-nw)/2, (h-nh)/2
	xdraw.CatmullRom.Scale(dst, image.Rect(x0, y0, x0+nw, y0+nh), src, sb, xdraw.Over, nil)
	return dst
}

func toNRGBA(img image.Image) *image.NRGBA {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	xdraw.Draw(out, out.Bounds(), img, b.Min, xdraw.Src)
	return out
}

func chunk(w io.Writer, typ string, data []byte) error {
	var hdr [8]byte
	binary.BigEndian.PutUint32(hdr[:4], uint32(len(data)))
	copy(hdr[4:], typ)
	crc := crc32.NewIEEE()
	crc.Write(hdr[4:])
	crc.Write(data)
	var tail [4]byte
	binary.BigEndian.PutUint32(tail[:], crc.Sum32())
	for _, p := range [][]byte{hdr[:], data, tail[:]} {
		if _, err := w.Write(p); err != nil {
			return err
		}
	}
	return nil
}

// deflate returns zlib data of the image's scanlines (filter 0 = None,
// RGBA 8-bit, non-premultiplied).
func deflate(img *image.NRGBA) []byte {
	var buf bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	w, h := img.Rect.Dx(), img.Rect.Dy()
	row := make([]byte, 1+4*w)
	for y := 0; y < h; y++ {
		row[0] = 0
		copy(row[1:], img.Pix[y*img.Stride:y*img.Stride+4*w])
		zw.Write(row)
	}
	zw.Close()
	return buf.Bytes()
}

func fcTL(seq uint32, w, h int, blend byte) []byte {
	b := make([]byte, 26)
	binary.BigEndian.PutUint32(b[0:], seq)
	binary.BigEndian.PutUint32(b[4:], uint32(w))
	binary.BigEndian.PutUint32(b[8:], uint32(h))
	// x/y offset 0
	binary.BigEndian.PutUint16(b[20:], 1)  // delay_num
	binary.BigEndian.PutUint16(b[22:], 10) // delay_den -> 1/10 s
	b[24] = 0                              // dispose_op NONE
	b[25] = blend
	return b
}

// Encode writes the dual APNG. b is fitted to a's size.
func Encode(w io.Writer, a, b image.Image, pad Padding) error {
	A := toNRGBA(a)
	W, H := A.Rect.Dx(), A.Rect.Dy()
	if W == 0 || H == 0 {
		return fmt.Errorf("图片 A 尺寸为空")
	}
	B := Fit(b, W, H, pad)
	T := image.NewNRGBA(image.Rect(0, 0, W, H)) // fully transparent

	if _, err := w.Write([]byte("\x89PNG\r\n\x1a\n")); err != nil {
		return err
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(W))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(H))
	ihdr[8], ihdr[9] = 8, 6 // 8-bit RGBA
	actl := make([]byte, 8)
	binary.BigEndian.PutUint32(actl[0:], 2) // num_frames
	// num_plays = 0 (infinite)
	steps := []struct {
		t string
		d []byte
	}{{"IHDR", ihdr}, {"acTL", actl}}
	// Default image A: IDAT with no preceding fcTL => not an animation frame.
	idat := deflate(A)
	for len(idat) > 0 {
		n := min(len(idat), 1<<20)
		steps = append(steps, struct {
			t string
			d []byte
		}{"IDAT", idat[:n]})
		idat = idat[n:]
	}
	seq := uint32(0)
	for i, fr := range []*image.NRGBA{B, T} {
		blend := byte(0)
		if i == 1 {
			blend = 1 // OVER
		}
		steps = append(steps, struct {
			t string
			d []byte
		}{"fcTL", fcTL(seq, W, H, blend)})
		seq++
		fd := make([]byte, 4)
		binary.BigEndian.PutUint32(fd, seq)
		fd = append(fd, deflate(fr)...)
		steps = append(steps, struct {
			t string
			d []byte
		}{"fdAT", fd})
		seq++
	}
	steps = append(steps, struct {
		t string
		d []byte
	}{"IEND", nil})
	for _, s := range steps {
		if err := chunk(w, s.t, s.d); err != nil {
			return err
		}
	}
	return nil
}

// EncodeFiles loads two files and writes the result to out.
func EncodeFiles(aPath, bPath, out string, pad Padding) error {
	a, err := Load(aPath)
	if err != nil {
		return fmt.Errorf("读取图片 A 失败: %w", err)
	}
	b, err := Load(bPath)
	if err != nil {
		return fmt.Errorf("读取图片 B 失败: %w", err)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, a, b, pad); err != nil {
		return err
	}
	return os.WriteFile(out, buf.Bytes(), 0o644)
}
