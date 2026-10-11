package dualapng

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func solid(w, h int, c color.NRGBA) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x/20+y/20)%2 == 0 {
				m.Set(x, y, c)
			} else {
				m.Set(x, y, color.NRGBA{255, 255, 255, 255})
			}
		}
	}
	return m
}

// TestDual writes testdata/A.png, B.png and out.png and checks that the
// standard decoder sees A and the chunk layout matches the reference.
func TestDual(t *testing.T) {
	A := solid(200, 300, color.NRGBA{255, 0, 0, 255}) // red checker
	B := solid(200, 300, color.NRGBA{0, 0, 255, 255}) // blue checker
	dir := filepath.Join("..", "testdata")
	os.MkdirAll(dir, 0o755)
	for n, m := range map[string]image.Image{"A.png": A, "B.png": B} {
		f, _ := os.Create(filepath.Join(dir, n))
		png.Encode(f, m)
		f.Close()
	}
	var buf bytes.Buffer
	if err := Encode(&buf, A, B, PadTransparent); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "out.png"), buf.Bytes(), 0o644)

	got, err := png.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	diff := 0
	for y := 0; y < 300; y++ {
		for x := 0; x < 200; x++ {
			if color.NRGBAModel.Convert(got.At(x, y)) != A.At(x, y) {
				diff++
			}
		}
	}
	if diff != 0 {
		t.Fatalf("static decode differs from A in %d pixels", diff)
	}

	d := buf.Bytes()[8:]
	var seq []string
	var seqNums []uint32
	for len(d) >= 12 {
		n := binary.BigEndian.Uint32(d)
		typ := string(d[4:8])
		body := d[8 : 8+n]
		if typ == "fcTL" || typ == "fdAT" {
			seqNums = append(seqNums, binary.BigEndian.Uint32(body))
		}
		if typ == "fcTL" {
			t.Logf("fcTL seq=%d w=%d h=%d delay=%d/%d dispose=%d blend=%d", binary.BigEndian.Uint32(body), binary.BigEndian.Uint32(body[4:]), binary.BigEndian.Uint32(body[8:]), binary.BigEndian.Uint16(body[20:]), binary.BigEndian.Uint16(body[22:]), body[24], body[25])
		}
		if typ == "acTL" {
			t.Logf("acTL frames=%d plays=%d", binary.BigEndian.Uint32(body), binary.BigEndian.Uint32(body[4:]))
		}
		if len(seq) == 0 || seq[len(seq)-1] != typ || typ != "IDAT" {
			seq = append(seq, typ)
		}
		d = d[12+n:]
	}
	want := "[IHDR acTL IDAT fcTL fdAT fcTL fdAT IEND]"
	if s := fmtS(seq); s != want {
		t.Fatalf("layout %s, want %s", s, want)
	}
	for i, s := range seqNums {
		if s != uint32(i) {
			t.Fatalf("sequence numbers %v", seqNums)
		}
	}
	t.Logf("layout %s, seq %v", fmtS(seq), seqNums)
}

func fmtS(s []string) string {
	b := "["
	for i, x := range s {
		if i > 0 {
			b += " "
		}
		b += x
	}
	return b + "]"
}
