package main

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/spf13/cobra"
	"github.com/xaionaro-go/thermalmaster/internal/cliflags"
	"github.com/xaionaro-go/thermalmaster/pkg/thermalmaster"
)

func main() {
	var cfg cliflags.Config
	var skipFrames int

	cmd := &cobra.Command{
		Use:   "thermalmaster-photo <output.png>",
		Short: "Capture a single frame from the ThermalMaster P3 camera",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg, skipFrames, args[0])
		},
	}

	cliflags.RegisterFlags(cmd, &cfg)
	cmd.Flags().IntVar(&skipFrames, "skip", 5, "frames to skip before capture (camera warmup)")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(
	cfg cliflags.Config,
	skipFrames int,
	outputPath string,
) error {
	sensor, err := cfg.ParseSensor()
	if err != nil {
		return err
	}

	cm, err := cfg.ParseColormap()
	if err != nil {
		return err
	}

	dev, info, err := cfg.SetupCamera()
	if err != nil {
		return err
	}
	defer dev.Close()
	fmt.Fprintf(os.Stderr, "Camera: %s (FW %s, SN %s)\n", info.Model, info.FWVersion, info.Serial)

	if err := dev.StartStreaming(context.Background()); err != nil {
		return fmt.Errorf("starting stream: %w", err)
	}
	defer dev.StopStreaming()

	// Apply hardware settings after streaming starts — some settings (palette,
	// gain) trigger ISP reconfiguration that disrupts USB before streaming.
	if err := cfg.ApplyHardwareSettings(dev); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	modelCfg := dev.Config()
	upCfg := cfg.BuildUpscaleConfig(sensor)
	ctx := context.Background()

	// Skip initial frames for camera warmup / AGC settling.
	for range skipFrames {
		if _, err := dev.ReadFrame(ctx); err != nil {
			return fmt.Errorf("reading warmup frame: %w", err)
		}
	}

	rawFrame, err := dev.ReadFrame(ctx)
	if err != nil {
		return fmt.Errorf("reading frame: %w", err)
	}

	img, thermal, ok := thermalmaster.BuildImage(rawFrame, modelCfg, thermalmaster.FrameBuilderConfig{
		Sensor:   sensor,
		Colormap: cm,
		Upscale:  upCfg,
	})
	if !ok {
		return fmt.Errorf("failed to extract frame data")
	}

	outW := img.Bounds().Dx()
	outH := img.Bounds().Dy()

	// Apply legend overlay if enabled and colormap is active.
	if cfg.Legend && cm != nil {
		lcfg, err := cfg.BuildLegendConfig(cm)
		if err != nil {
			return fmt.Errorf("building legend config: %w", err)
		}

		legendR, err := thermalmaster.NewLegendRenderer(lcfg)
		if err != nil {
			return fmt.Errorf("creating legend renderer: %w", err)
		}

		if thermal != nil {
			tMin, tMax := thermalmaster.ThermalMinMax(thermal)
			rgbaImg, ok := img.(*image.RGBA)
			if ok {
				result := legendR.Apply(rgbaImg.Pix, thermalmaster.PixelFormatRGBA32, outW, outH, tMin, tMax)
				if result != nil {
					img = result
					outW = result.Bounds().Dx()
					outH = result.Bounds().Dy()
				}
			}
		}
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outputPath, err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encoding PNG: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Saved %dx%d image to %s\n", outW, outH, outputPath)
	return nil
}
