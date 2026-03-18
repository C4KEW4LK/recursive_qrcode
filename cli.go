package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

func main() {
	runCLI()
}

func runCLI() {
	output := flag.String("output", "infinite_zoom_qr.gif", "Output GIF path")
	frames := flag.Int("frames", 60, "Frame count (smoothness)")
	fps := flag.Float64("fps", 30, "Playback speed in frames per second")
	size := flag.Int("size", 600, "Output size in pixels")
	levels := flag.Int("levels", 6, "Number of zoom levels")
	superSample := flag.Int("supersample", 4, "Supersample factor (higher = smoother, more RAM)")
	flag.Parse()

	texts := flag.Args()
	if len(texts) == 0 {
		fmt.Fprintln(os.Stderr, `Usage: qrzoom [flags] "<text1>" ["<text2>" ...]`)
		flag.PrintDefaults()
		os.Exit(1)
	}

	outPath := *output
	if filepath.Ext(outPath) == "" {
		outPath += ".gif"
	}
	if *fps < 1 {
		*fps = 1
	}

	cfg := GenerateConfig{
		Texts:         texts,
		OutputPath:    outPath,
		OutputSize:    *size,
		SuperSample:   *superSample,
		Levels:        *levels,
		FrameCount:    *frames,
		FrameDuration: int(math.Round(1000 / *fps)),
		Progress: func(step, total int, msg string) {
			fmt.Printf("\r[%3d%%] %s", step*100/total, msg)
		},
	}

	fmt.Printf("Generating infinite zoom QR GIF...\n")
	for _, t := range texts {
		fmt.Printf("  %q\n", t)
	}
	if err := Generate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nDone: %s\n", outPath)
}
