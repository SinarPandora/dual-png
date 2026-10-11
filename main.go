package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"apng-dual/dualapng"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type app struct {
	win          *mygo.Window
	aPath, bPath string
	aBmp, bBmp   *ui.Bitmap
	outPath      string
	blackPad     bool
	status       string
	busy         bool
}

var imgFilters = []mygo.FileFilter{{Name: "图片 (PNG/JPEG)", Extensions: []string{"png", "jpg", "jpeg"}}}

func (a *app) setImage(which byte, path string) {
	bmp, err := loadBitmap(path)
	if err != nil {
		a.status = "无法读取图片: " + err.Error()
		return
	}
	if which == 'A' {
		a.aPath, a.aBmp = path, bmp
		if a.outPath == "" {
			a.outPath = strings.TrimSuffix(path, filepath.Ext(path)) + "_dual.png"
		}
	} else {
		a.bPath, a.bBmp = path, bmp
	}
	a.status = ""
}

func loadBitmap(path string) (*ui.Bitmap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ui.DecodeBitmap(data)
}

func (a *app) pick(which byte, title string) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: title, Filters: imgFilters})
		if err != nil || len(paths) == 0 {
			return
		}
		a.win.Update(func() { a.setImage(which, paths[0]) })
	}()
}

func (a *app) pickOut() {
	go func() {
		def := a.outPath
		if def == "" {
			def = "dual.png"
		}
		p, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.win, Title: "保存为", DefaultPath: def,
			Filters: []mygo.FileFilter{{Name: "PNG", Extensions: []string{"png"}}}})
		if err != nil || p == "" {
			return
		}
		if !strings.EqualFold(filepath.Ext(p), ".png") {
			p += ".png"
		}
		a.win.Update(func() { a.outPath = p })
	}()
}

func (a *app) generate() {
	if a.aPath == "" || a.bPath == "" {
		a.status = "请先选择图片 A 和图片 B"
		return
	}
	if a.outPath == "" {
		a.status = "请选择输出路径"
		return
	}
	a.busy, a.status = true, "正在生成…"
	aP, bP, out := a.aPath, a.bPath, a.outPath
	pad := dualapng.PadTransparent
	if a.blackPad {
		pad = dualapng.PadBlack
	}
	go func() {
		err := dualapng.EncodeFiles(aP, bP, out, pad)
		a.win.Update(func() {
			a.busy = false
			if err != nil {
				a.status = "生成失败: " + err.Error()
			} else {
				a.status = "已生成: " + out
			}
		})
	}()
}

func (a *app) slot(c *ui.Context, which byte, title string, path string, bmp *ui.Bitmap) {
	box := ui.Column(c).Width(220).Gap(8).Padding(10).AlignItems(ui.Center)
	box.Children(func() {
		ui.Text(c, title).Bold()
		ui.Column(c).Width(200).Height(160).Center().Children(func() {
			if bmp != nil {
				ui.Image(c, bmp).Width(200).Height(160)
			} else {
				ui.Text(c, "拖放图片到这里").TextColor(c.Theme().TextMuted)
			}
		})
		name := "未选择"
		if path != "" {
			name = filepath.Base(path)
		}
		ui.Text(c, name).FontSize(12).TextColor(c.Theme().TextMuted)
		if ui.Button(c, "选择图片…").Clicked() {
			a.pick(which, title)
		}
	})
	if files := box.DroppedFiles(); len(files) > 0 {
		a.setImage(which, files[0])
	}
}

func (a *app) view(c *ui.Context) {
	ui.Column(c).Fill().Gap(14).Padding(20).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "双面 PNG").FontSize(22).Bold()
		ui.Row(c).Gap(16).Children(func() {
			a.slot(c, 'A', "图片 A（缩略图/静态显示）", a.aPath, a.aBmp)
			a.slot(c, 'B', "图片 B（点开后显示）", a.bPath, a.bBmp)
		})
		ui.Checkbox(c, &a.blackPad, "B 比例不同时用黑色填充（默认透明）")
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			out := a.outPath
			if out == "" {
				out = "（未选择输出路径）"
			}
			ui.Text(c, out).FontSize(12).Width(340)
			if ui.Button(c, "输出到…").Clicked() {
				a.pickOut()
			}
		})
		if ui.PrimaryButton(c, "生成").Width(140).Disabled(a.busy).Clicked() {
			a.generate()
		}
		ui.Text(c, a.status).FontSize(12)
	})
}

func main() {
	aF := flag.String("a", "", "图片 A（静态显示）")
	bF := flag.String("b", "", "图片 B（APNG 查看器显示）")
	oF := flag.String("o", "", "输出 PNG 路径")
	black := flag.Bool("black", false, "比例不同时用黑色填充（默认透明）")
	flag.Parse()
	if *aF != "" || *bF != "" || *oF != "" {
		if *aF == "" || *bF == "" || *oF == "" {
			fmt.Fprintln(os.Stderr, "用法: dualpng -a A.png -b B.png -o out.png [-black]")
			os.Exit(2)
		}
		pad := dualapng.PadTransparent
		if *black {
			pad = dualapng.PadBlack
		}
		if err := dualapng.EncodeFiles(*aF, *bF, *oF, pad); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("已生成:", *oF)
		return
	}
	a := &app{}
	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title: "双面 PNG", Width: 560, Height: 520, MinWidth: 520, MinHeight: 480,
			StateKey: "main", Content: ui.View(a.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
