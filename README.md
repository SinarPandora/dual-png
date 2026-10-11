# 双面 PNG（dual-png）

生成一种“双面” PNG：在聊天列表、相册缩略图、文件管理器等只认普通 PNG 的地方显示 **图片 A**，在浏览器、iOS / macOS 照片等支持 APNG 的查看器里点开后显示 **图片 B**。

基于 [MyGo](https://github.com/egoist/mygo) 原生 UI，纯 Go、无 cgo，可同时编译 Windows / macOS / Linux。

## 原理

APNG 允许文件里的默认图（`IDAT`）不属于动画：只要 `IDAT` 前面没有 `fcTL`，不支持 APNG 的解码器就只会读出默认图，而 APNG 解码器会跳过它，只播放 `fdAT` 动画帧。本程序写出的块顺序如下：

```
IHDR → acTL(2 帧, 无限循环)
     → IDAT          图片 A（默认图，不参与动画）
     → fcTL + fdAT   第 1 帧：图片 B
     → fcTL + fdAT   第 2 帧：全透明，按 OVER 叠加，让 B 保持显示
     → IEND
```

编码器在 `dualapng/` 包里，zlib、CRC 和块写入都是自己实现的，输出 8 位 RGBA。

## 使用

### 图形界面

1. 选择或拖入 **图片 A（缩略图/静态显示）** 和 **图片 B（点开后显示）**，支持 PNG / JPEG。
2. 点「输出到…」选择保存位置，默认是 `<A 的文件名>_dual.png`。
3. 点「生成」。

### 命令行

```sh
dualpng -a A.png -b B.png -o out.png [-black]
```

输出尺寸与 A 一致。B 会按原比例缩放后居中放置，空白处默认透明，加 `-black`（或勾选界面里的复选框）则填成黑色。

> Windows 版是窗口程序，命令行模式不会在控制台打印信息，但会正常写出文件。

## 注意

- 微信、QQ 等平台会重新压缩图片，转发后这个效果会消失。请用原图或文件方式发送。
- 动图输入只取第一帧。
- 输出不做行滤波优化，文件会比同尺寸的普通 PNG 大一些。
- 发布的二进制没有签名：Windows SmartScreen 里点「更多信息 → 仍要运行」；macOS 上右键「打开」，或运行 `xattr -dr com.apple.quarantine "双面 PNG.app"`。

## 构建

需要 Go 1.27+。

```sh
go test ./dualapng          # 校验：标准解码得到 A，块布局正确
go tool mygo dev            # 开发运行
go tool mygo build -platform windows/amd64,darwin/arm64
```

`verify_apng.py` 用 Pillow 和 ffmpeg 交叉验证 `testdata/out.png`：静态读取得到 A，APNG 帧得到 B。

## License

MIT
