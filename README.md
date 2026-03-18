# qrzoom

Generates a seamlessly looping GIF that infinitely zooms into a recursive QR code. Each zoom level is a scannable QR code with a 30% centre cutout revealing the next level nested inside it. Multiple texts can be provided and cycle through one by one.

## Build

Requires Go 1.21+.

```sh
go mod tidy
go build -o qrzoom .
```

## Usage

```
qrzoom [flags] <text1> [text2] ...
```

Positional arguments are the URLs or texts to encode. With multiple texts the GIF cycles through each in order before looping.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-output` | `infinite_zoom_qr.gif` | Output GIF path |
| `-frames` | `60` | Frames per zoom loop (smoothness) |
| `-fps` | `30` | Playback speed |
| `-size` | `600` | Output size in pixels |
| `-levels` | `6` | Number of nested QR levels visible per loop |
| `-supersample` | `4` | Render at N× resolution then downsample. Higher = smoother edges, more RAM |

### Examples

Single QR code:
```sh
qrzoom "https://example.com"
```

Two alternating QR codes:
```sh
qrzoom -output out.gif "https://example.com" "https://other.com"
```

Custom settings:
```sh
qrzoom -fps 24 -frames 48 -size 800 -supersample 6 "https://example.com"
```

## How it works

1. Each input text is encoded as a QR code bitmap (`[][]bool`).
2. The centre 30% of each matrix is blanked to white — the cutout where the next level shows through.
3. Each animation frame is rendered analytically at N× supersampled resolution:
   - For each hi-res pixel, the code walks depth levels. The pixel's centre point in module-space determines which level owns it — if inside the cutout it falls through to the next level, otherwise it area-samples the current QR matrix.
   - The hi-res grid is box-averaged down to the output size, producing smooth anti-aliased edges.
4. All frames are rendered concurrently and written as a looping GIF with a 16-shade greyscale palette.

**Seamless loop:** at `t=1` the inner QR fills the viewport identically to the outer QR at `t=0`, so the animation loops without a visible cut.
