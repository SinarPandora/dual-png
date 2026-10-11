# Verifies testdata/out.png: Pillow default image == A, Pillow frame and ffmpeg first frame == B.
import subprocess, sys
from PIL import Image, ImageChops
out = sys.argv[1] if len(sys.argv) > 1 else "testdata/out.png"
A = Image.open("testdata/A.png").convert("RGBA"); B = Image.open("testdata/B.png").convert("RGBA")
def same(x, y): return ImageChops.difference(x.convert("RGBA"), y).getbbox(alpha_only=False) is None
im = Image.open(out)
print("Pillow format", im.format, "n_frames", im.n_frames, "default_image", im.info.get("default_image"))
print("Pillow frame0 (default, before seek) == A:", same(im, A))
im.seek(1); print("Pillow seek(1) == B:", same(im, B))
if im.n_frames > 2: im.seek(2); print("Pillow seek(2) == B:", same(im, B))
subprocess.run(["ffmpeg","-v","error","-y","-i",out,"-frames:v","1","/tmp/ff1.png"],check=True)
print("ffmpeg first frame == B:", same(Image.open("/tmp/ff1.png"), B))
subprocess.run(["ffmpeg","-v","error","-y","-i",out,"-vsync","0","/tmp/ffall_%d.png"],check=True)
import glob
for f in sorted(glob.glob("/tmp/ffall_*.png")): print("ffmpeg", f, "== B:", same(Image.open(f), B), "== A:", same(Image.open(f), A))
