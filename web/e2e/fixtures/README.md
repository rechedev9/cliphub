# Stream media fixture

`stream-source.mp4` is a generated 30-second solid-colour H.264 video with silent AAC audio (about 16 KB). It contains no third-party footage. It lets the editor tests exercise a real media clock without network access or requiring FFmpeg in CI.

Regenerate with:

```sh
ffmpeg -f lavfi -i "color=c=0x183443:s=160x90:r=10:d=30" -f lavfi -i "anullsrc=r=8000:cl=mono" -t 30 -c:v libx264 -pix_fmt yuv420p -c:a aac -b:a 8k -movflags +faststart stream-source.mp4
```
