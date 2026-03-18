package main

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"math"
	"os"
	"sync"
	"sync/atomic"

	qrcode "github.com/skip2/go-qrcode"
)

// centerRatio is the fraction of each QR's width used as the centre hole
// for the next nested level. Also the zoom ratio between levels.
const centerRatio = 0.30

func getQRMatrix(content string) ([][]bool, error) {
	qr, err := qrcode.New(content, qrcode.Highest)
	if err != nil {
		return nil, err
	}
	m := qr.Bitmap()
	blankCenter(m)
	return m, nil
}

// blankCenter sets the centre 30% of the QR matrix to white so the next
// nested level shows through cleanly.
func blankCenter(matrix [][]bool) {
	n := len(matrix)
	margin := int(math.Round(float64(n) * (1 - centerRatio) / 2))
	for row := margin; row < n-margin; row++ {
		for col := margin; col < n-margin; col++ {
			matrix[row][col] = false
		}
	}
}

// sampleMatrix returns the average darkness [0,1] of a QR matrix over a
// rectangular region in module-space. Out-of-bounds area counts as white.
func sampleMatrix(matrix [][]bool, x0, y0, x1, y1 float64) float64 {
	n := float64(len(matrix))
	totalArea := (x1 - x0) * (y1 - y0)
	if totalArea <= 0 {
		return 0
	}
	x0 = max(x0, 0)
	y0 = max(y0, 0)
	x1 = min(x1, n)
	y1 = min(y1, n)
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	blackArea := 0.0
	for row := int(math.Floor(y0)); row < int(math.Ceil(y1)); row++ {
		if row < 0 || row >= len(matrix) {
			continue
		}
		yOver := math.Min(float64(row+1), y1) - math.Max(float64(row), y0)
		for col := int(math.Floor(x0)); col < int(math.Ceil(x1)); col++ {
			if col < 0 || col >= len(matrix[row]) {
				continue
			}
			if matrix[row][col] {
				xOver := math.Min(float64(col+1), x1) - math.Max(float64(col), x0)
				blackArea += xOver * yOver
			}
		}
	}
	return blackArea / totalArea
}

// samplePixel returns the grey value [0,255] for a single hires pixel by
// walking depth levels until the pixel centre falls outside the cutout.
func samplePixel(matrices [][][]bool, t float64, outputSize, px, py int) uint8 {
	for depth := range matrices {
		n := float64(len(matrices[depth]))
		levelPx := float64(outputSize) * math.Pow(centerRatio, float64(depth)-t)
		offset := (float64(outputSize) - levelPx) / 2

		mx0 := (float64(px) - offset) * n / levelPx
		my0 := (float64(py) - offset) * n / levelPx
		mx1 := (float64(px+1) - offset) * n / levelPx
		my1 := (float64(py+1) - offset) * n / levelPx

		if mx1 <= 0 || my1 <= 0 || mx0 >= n || my0 >= n {
			continue
		}

		margin := math.Round(n * (1 - centerRatio) / 2)
		cMin := margin
		cMax := n - margin

		// Hard cutout threshold based on pixel centre — no blending.
		midX := (mx0 + mx1) / 2
		midY := (my0 + my1) / 2
		if midX < cMin || midX >= cMax || midY < cMin || midY >= cMax {
			v := sampleMatrix(matrices[depth], mx0, my0, mx1, my1)
			return uint8(math.Round((1 - v) * 255))
		}
		// Inside cutout — fall through to next level.
	}
	return 255 // background
}

// renderFrame renders one animation frame. It builds a superSample× hires
// grid then box-averages down to outputSize.
func renderFrame(matrices [][][]bool, t float64, outputSize, superSample int) *image.Gray {
	ss := max(superSample, 1)
	hiSize := outputSize * ss

	hi := make([]float64, hiSize*hiSize)
	for py := 0; py < hiSize; py++ {
		for px := 0; px < hiSize; px++ {
			hi[py*hiSize+px] = float64(samplePixel(matrices, t, hiSize, px, py))
		}
	}

	out := image.NewGray(image.Rect(0, 0, outputSize, outputSize))
	for oy := 0; oy < outputSize; oy++ {
		for ox := 0; ox < outputSize; ox++ {
			var sum float64
			for dy := 0; dy < ss; dy++ {
				for dx := 0; dx < ss; dx++ {
					sum += hi[(oy*ss+dy)*hiSize+(ox*ss+dx)]
				}
			}
			out.Pix[oy*outputSize+ox] = uint8(math.Round(sum / float64(ss*ss)))
		}
	}
	return out
}

// paletted converts a grayscale image to a 16-shade paletted image for GIF.
func paletted(src *image.Gray) *image.Paletted {
	const shades = 16
	pal := make(color.Palette, shades)
	for i := range pal {
		v := uint8(math.Round(float64(i) * 255 / float64(shades-1)))
		pal[i] = color.Gray{Y: v}
	}
	dst := image.NewPaletted(src.Bounds(), pal)
	b := src.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			v := src.GrayAt(x, y).Y
			dst.SetColorIndex(x, y, uint8(math.Round(float64(v)*float64(shades-1)/255)))
		}
	}
	return dst
}

// makeFrames renders frameCount frames concurrently.
func makeFrames(matrices [][][]bool, frameCount, outputSize, superSample int, progress func(int)) []*image.Paletted {
	frames := make([]*image.Paletted, frameCount)
	var wg sync.WaitGroup
	var done atomic.Int64
	for i := range frames {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			t := float64(i) / float64(frameCount)
			frames[i] = paletted(renderFrame(matrices, t, outputSize, superSample))
			if progress != nil {
				progress(int(done.Add(1)))
			}
		}(i)
	}
	wg.Wait()
	return frames
}

// GenerateConfig holds all parameters for GIF generation.
type GenerateConfig struct {
	Texts         []string
	OutputPath    string
	OutputSize    int
	SuperSample   int // hires grid is SuperSample× output size, then box-averaged down
	Levels        int // number of nested QR levels visible per loop
	FrameCount    int // frames per zoom loop
	FrameDuration int // milliseconds per frame
	Progress      func(step, total int, msg string)
}

// Generate renders one zoom loop per text and writes the concatenated GIF.
func Generate(cfg GenerateConfig) error {
	report := func(step, total int, msg string) {
		if cfg.Progress != nil {
			cfg.Progress(step, total, msg)
		}
	}

	n := len(cfg.Texts)
	if n == 0 {
		return fmt.Errorf("at least one text is required")
	}

	allMatrices := make([][][]bool, n)
	for i, text := range cfg.Texts {
		m, err := getQRMatrix(text)
		if err != nil {
			return err
		}
		allMatrices[i] = m
	}

	results := make([][]*image.Paletted, n)
	errs := make([]error, n)
	var framesCompleted atomic.Int64
	totalFrames := int64(n * cfg.FrameCount)

	var wg sync.WaitGroup
	for i := range cfg.Texts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			levels := make([][][]bool, cfg.Levels)
			for j := range levels {
				levels[j] = allMatrices[(i+j)%n]
			}
			results[i] = makeFrames(levels, cfg.FrameCount, cfg.OutputSize, cfg.SuperSample, func(_ int) {
				done := framesCompleted.Add(1)
				report(int(done*90/totalFrames), 100, fmt.Sprintf("Rendering frames… (%d/%d)", done, totalFrames))
			})
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	var frames []*image.Paletted
	for _, r := range results {
		frames = append(frames, r...)
	}

	report(90, 100, "Saving GIF…")
	gifDelay := max(cfg.FrameDuration/10, 1)
	delays := make([]int, len(frames))
	for i := range delays {
		delays[i] = gifDelay
	}

	f, err := os.Create(cfg.OutputPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := gif.EncodeAll(f, &gif.GIF{Image: frames, Delay: delays, LoopCount: 0}); err != nil {
		return err
	}

	report(100, 100, "Done!")
	return nil
}
