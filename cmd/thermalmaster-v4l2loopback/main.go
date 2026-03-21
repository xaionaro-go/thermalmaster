package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/facebookincubator/go-belt"
	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/facebookincubator/go-belt/tool/logger/implementation/logrus"
	logtypes "github.com/facebookincubator/go-belt/tool/logger/types"
	"github.com/spf13/cobra"
	"github.com/xaionaro-go/thermalmaster/internal/cliflags"
	"github.com/xaionaro-go/thermalmaster/pkg/colormap"
	"github.com/xaionaro-go/thermalmaster/pkg/thermalmaster"
)

func main() {
	var cfg cliflags.Config
	var maxFPS int
	var cpuProfile string
	var logLevel string

	cmd := &cobra.Command{
		Use:   "thermalmaster-v4l2loopback <v4l2-device>",
		Short: "Stream ThermalMaster P3 camera to a v4l2loopback device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg, args[0], maxFPS, cpuProfile, logLevel)
		},
	}

	cliflags.RegisterFlags(cmd, &cfg)
	cmd.Flags().IntVar(&maxFPS, "max-fps", 0, "limit output frame rate (0 = unlimited)")
	cmd.Flags().StringVar(&cpuProfile, "cpuprofile", "", "write CPU profile to file")
	cmd.Flags().StringVar(&logLevel, "log-level", "warning", "log level: trace, debug, info, warning, error")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(
	cfg cliflags.Config,
	devicePath string,
	maxFPS int,
	cpuProfile string,
	logLevelStr string,
) error {
	ll, err := logtypes.ParseLogLevel(logLevelStr)
	if err != nil {
		return fmt.Errorf("parsing log level: %w", err)
	}
	l := logrus.Default().WithLevel(ll)
	ctx := logger.CtxWithLogger(context.Background(), l)
	ctx = belt.WithField(ctx, "cmd", "v4l2loopback")

	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			return fmt.Errorf("creating CPU profile: %w", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("starting CPU profile: %w", err)
		}
		defer pprof.StopCPUProfile()
	}

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

	if err := dev.StartStreaming(ctx); err != nil {
		return fmt.Errorf("starting stream: %w", err)
	}
	defer dev.StopStreaming()

	if err := cfg.ApplyHardwareSettings(dev); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	modelCfg := dev.Config()
	upCfg := cfg.BuildUpscaleConfig(sensor)

	fbCfg := thermalmaster.FrameBuilderConfig{
		Sensor:   sensor,
		Colormap: cm,
		Upscale:  upCfg,
	}

	// Set up legend renderer if enabled.
	var legendR *thermalmaster.LegendRenderer
	if cfg.Legend && cm != nil {
		lcfg, err := cfg.BuildLegendConfig(cm)
		if err != nil {
			return fmt.Errorf("building legend config: %w", err)
		}

		legendR, err = thermalmaster.NewLegendRenderer(lcfg)
		if err != nil {
			return fmt.Errorf("creating legend renderer: %w", err)
		}
	}

	// Determine output dimensions.
	outW, outH := modelCfg.SensorW, modelCfg.SensorH
	if sensor == thermalmaster.SensorBlended && upCfg != nil {
		outW *= upCfg.Factor
		outH *= upCfg.Factor
	}

	// When legend extends the frame, compute the output dimensions for v4l2.
	v4l2OutW, v4l2OutH := outW, outH
	if legendR != nil {
		dummyPixels := make([]byte, outW*outH*3)
		dummyResult := legendR.Apply(dummyPixels, thermalmaster.PixelFormatRGB24, outW, outH, 0, 1)
		if dummyResult != nil {
			v4l2OutW = dummyResult.Bounds().Dx()
			v4l2OutH = dummyResult.Bounds().Dy()
		}
	}

	bpp := bytesPerPixelFor(sensor, cm)
	frameBytes := v4l2OutW * v4l2OutH * bpp

	v4l2, err := os.OpenFile(devicePath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf(
			"opening %s: %w (is v4l2loopback loaded? try: sudo modprobe v4l2loopback devices=1)",
			devicePath, err,
		)
	}
	defer v4l2.Close()

	pixFmt := v4l2PixFmtFor(sensor, cm)
	if err := setV4L2Format(v4l2, uint32(v4l2OutW), uint32(v4l2OutH), pixFmt, uint32(bpp)); err != nil {
		return fmt.Errorf("setting format on %s: %w", devicePath, err)
	}

	fmt.Fprintf(os.Stderr, "Streaming %dx%d (%d bpp) to %s\n",
		v4l2OutW, v4l2OutH, bpp*8, devicePath)
	if cm != nil {
		fmt.Fprintf(os.Stderr, "Colormap: %s\n", cfg.Colormap)
	}
	if legendR != nil {
		fmt.Fprintln(os.Stderr, "Legend: enabled")
	}
	fmt.Fprintf(os.Stderr, "View: mpv --profile=low-latency --untimed av://v4l2:%s\n", devicePath)
	fmt.Fprintln(os.Stderr, "Press Ctrl+C to stop")

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var rateTick <-chan time.Time
	if maxFPS > 0 {
		ticker := time.NewTicker(time.Second / time.Duration(maxFPS))
		defer ticker.Stop()
		rateTick = ticker.C
	}

	var (
		frameCount, periodFrames               uint64
		readErrors, extractErrors, sizeErrors  uint64
		firstFrameTime                         time.Time
		lastStatus                             = time.Now()
	)

	for {
		select {
		case <-ctx.Done():
			if frameCount > 0 {
				elapsed := time.Since(firstFrameTime)
				actualFPS := float64(frameCount) / elapsed.Seconds()
				fmt.Fprintf(os.Stderr, "\nStopped. %d frames in %v (%.1f fps), dropped: %d read / %d extract / %d size\n",
					frameCount, elapsed.Round(time.Millisecond), actualFPS, readErrors, extractErrors, sizeErrors)
			} else {
				fmt.Fprintf(os.Stderr, "\nStopped. 0 frames, dropped: %d read / %d extract / %d size\n",
					readErrors, extractErrors, sizeErrors)
			}
			return nil
		default:
		}

		rawFrame, err := dev.ReadFrame(ctx)
		if err != nil {
			readErrors++
			reportDropped(&lastStatus, &periodFrames, frameCount, readErrors, extractErrors, sizeErrors)
			continue
		}

		pixelData, _, _, _, thermal, ok := thermalmaster.BuildPixels(rawFrame, modelCfg, fbCfg)
		if !ok {
			extractErrors++
			reportDropped(&lastStatus, &periodFrames, frameCount, readErrors, extractErrors, sizeErrors)
			continue
		}

		// Apply legend overlay if enabled.
		if legendR != nil && thermal != nil {
			tMin, tMax := thermalmaster.ThermalMinMax(thermal)
			result := legendR.Apply(pixelData, thermalmaster.PixelFormatRGB24, outW, outH, tMin, tMax)
			if result != nil {
				pixelData = thermalmaster.RGBAToRGB24(result)
			}
		}

		if len(pixelData) != frameBytes {
			fmt.Fprintf(os.Stderr, "Warning: frame size mismatch: got %d, want %d\n",
				len(pixelData), frameBytes)
			sizeErrors++
			reportDropped(&lastStatus, &periodFrames, frameCount, readErrors, extractErrors, sizeErrors)
			continue
		}

		if _, err := v4l2.Write(pixelData); err != nil {
			return fmt.Errorf("writing to %s: %w", devicePath, err)
		}

		frameCount++
		periodFrames++
		if frameCount == 1 {
			firstFrameTime = time.Now()
		}

		reportDropped(&lastStatus, &periodFrames, frameCount, readErrors, extractErrors, sizeErrors)

		if rateTick != nil {
			<-rateTick
		}
	}
}

func reportDropped(
	lastStatus *time.Time,
	periodFrames *uint64,
	frameCount, readErrors, extractErrors, sizeErrors uint64,
) {
	now := time.Now()
	if now.Sub(*lastStatus) < 5*time.Second {
		return
	}

	periodFPS := float64(*periodFrames) / now.Sub(*lastStatus).Seconds()
	fmt.Fprintf(os.Stderr, "  %d frames (%.1f fps), dropped: %d read / %d extract / %d size\n",
		frameCount, periodFPS, readErrors, extractErrors, sizeErrors)
	*lastStatus = now
	*periodFrames = 0
}

func bytesPerPixelFor(sensor thermalmaster.SensorSource, cm colormap.Colormap) int {
	if cm != nil {
		return 3 // RGB24
	}

	switch sensor {
	case thermalmaster.SensorIR:
		return 1 // GRAY8
	default:
		return 2 // GRAY16LE (thermal or blended)
	}
}
