package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	if err := execute(os.Args[1:], run); err != nil {
		os.Exit(1)
	}
}

func execute(args []string, runFn runFunc) error {
	signalCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	workCtx, cancelWork := context.WithCancelCause(context.Background())
	signalCallbackDone := make(chan struct{})
	stopSignalCallback := context.AfterFunc(signalCtx, func() {
		defer close(signalCallbackDone)
		stopSignals()
		cancelWork(context.Cause(signalCtx))
	})
	cmd := newCommand(runFn)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(workCtx)
	if !stopSignalCallback() {
		<-signalCallbackDone
	}
	stopSignals()
	cancelWork(context.Canceled)
	return err
}

type runFunc func(context.Context, cliflags.Config, string, int, string, string) error

type runStage uint8

const (
	runStageBeforeCameraSetup runStage = iota
	runStageAfterCameraSetup
	runStageBeforeStreamStart
	runStageAfterStreamStart
	runStageBeforeHardwareSettings
	runStageAfterHardwareSettings
	runStageBeforeV4L2Setup
	runStageAfterV4L2Setup
)

func (s runStage) String() string {
	switch s {
	case runStageBeforeCameraSetup:
		return "before camera setup"
	case runStageAfterCameraSetup:
		return "after camera setup"
	case runStageBeforeStreamStart:
		return "before stream start"
	case runStageAfterStreamStart:
		return "after stream start"
	case runStageBeforeHardwareSettings:
		return "before hardware settings"
	case runStageAfterHardwareSettings:
		return "after hardware settings"
	case runStageBeforeV4L2Setup:
		return "before v4l2 setup"
	case runStageAfterV4L2Setup:
		return "after v4l2 setup"
	default:
		return "unknown run stage"
	}
}

type runStageError struct {
	stage runStage
	err   error
}

func (e *runStageError) Error() string {
	return fmt.Sprintf("%s: %v", e.stage, e.err)
}

func (e *runStageError) Unwrap() error {
	return e.err
}

type runPreLoopOperations interface {
	checkpoint(context.Context, runStage) error
	setupCamera(cliflags.Config) (*thermalmaster.Device, thermalmaster.DeviceInfo, error)
	startStreaming(context.Context, *thermalmaster.Device) error
	applyHardwareSettings(cliflags.Config, *thermalmaster.Device) error
	setupV4L2(string, uint32, uint32, uint32, uint32) (io.WriteCloser, error)
}

type productionRunPreLoopOperations struct{}

func (productionRunPreLoopOperations) checkpoint(ctx context.Context, stage runStage) error {
	if err := context.Cause(ctx); err != nil {
		return &runStageError{stage: stage, err: err}
	}
	return nil
}

func (productionRunPreLoopOperations) setupCamera(
	cfg cliflags.Config,
) (*thermalmaster.Device, thermalmaster.DeviceInfo, error) {
	return cfg.SetupCamera()
}

func (productionRunPreLoopOperations) startStreaming(
	ctx context.Context,
	dev *thermalmaster.Device,
) error {
	return dev.StartStreaming(ctx)
}

func (productionRunPreLoopOperations) applyHardwareSettings(
	cfg cliflags.Config,
	dev *thermalmaster.Device,
) error {
	return cfg.ApplyHardwareSettings(dev)
}

func (productionRunPreLoopOperations) setupV4L2(
	devicePath string,
	width uint32,
	height uint32,
	pixelFormat uint32,
	bytesPerPixel uint32,
) (io.WriteCloser, error) {
	v4l2, err := os.OpenFile(devicePath, os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf(
			"opening %s: %w (is v4l2loopback loaded? try: sudo modprobe v4l2loopback devices=1)",
			devicePath,
			err,
		)
	}
	if err := setV4L2Format(v4l2, width, height, pixelFormat, bytesPerPixel); err != nil {
		_ = v4l2.Close()
		return nil, fmt.Errorf("setting format on %s: %w", devicePath, err)
	}
	return v4l2, nil
}

type frameReader interface {
	ReadFrame(context.Context) ([]byte, error)
}

type streamLoopConfig struct {
	modelConfig    thermalmaster.ModelConfig
	frameBuilder   thermalmaster.FrameBuilderConfig
	legendRenderer *thermalmaster.LegendRenderer
	outputWidth    int
	outputHeight   int
	frameBytes     int
	devicePath     string
	rateTick       <-chan time.Time
	statusWriter   io.Writer
}

func newCommand(runFn runFunc) *cobra.Command {
	var cfg cliflags.Config
	var maxFPS int
	var cpuProfile string
	var logLevel string

	cmd := &cobra.Command{
		Use:   "thermalmaster-v4l2loopback <v4l2-device>",
		Short: "Stream ThermalMaster P3 camera to a v4l2loopback device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFn(cmd.Context(), cfg, args[0], maxFPS, cpuProfile, logLevel)
		},
	}

	cliflags.RegisterFlags(cmd, &cfg)
	cmd.Flags().IntVar(&maxFPS, "max-fps", 0, "limit output frame rate (0 = unlimited)")
	cmd.Flags().StringVar(&cpuProfile, "cpuprofile", "", "write CPU profile to file")
	cmd.Flags().StringVar(&logLevel, "log-level", "warning", "log level: trace, debug, info, warning, error")

	return cmd
}

func run(
	ctx context.Context,
	cfg cliflags.Config,
	devicePath string,
	maxFPS int,
	cpuProfile string,
	logLevelStr string,
) error {
	return runWithOperations(
		ctx,
		cfg,
		devicePath,
		maxFPS,
		cpuProfile,
		logLevelStr,
		productionRunPreLoopOperations{},
	)
}

func runWithOperations(
	ctx context.Context,
	cfg cliflags.Config,
	devicePath string,
	maxFPS int,
	cpuProfile string,
	logLevelStr string,
	operations runPreLoopOperations,
) (_err error) {
	ll, err := logtypes.ParseLogLevel(logLevelStr)
	if err != nil {
		return fmt.Errorf("parsing log level: %w", err)
	}
	l := logrus.Default().WithLevel(ll)
	ctx = logger.CtxWithLogger(ctx, l)
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

	if err := operations.checkpoint(ctx, runStageBeforeCameraSetup); err != nil {
		return err
	}
	dev, info, err := operations.setupCamera(cfg)
	if err != nil {
		return err
	}
	defer joinDeviceCloseError(&_err, dev)
	if err := operations.checkpoint(ctx, runStageAfterCameraSetup); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Camera: %s (FW %s, SN %s)\n", info.Model, info.FWVersion, info.Serial)

	if err := operations.checkpoint(ctx, runStageBeforeStreamStart); err != nil {
		return err
	}
	if err := operations.startStreaming(ctx, dev); err != nil {
		return fmt.Errorf("starting stream: %w", err)
	}
	if err := operations.checkpoint(ctx, runStageAfterStreamStart); err != nil {
		return err
	}

	if err := operations.checkpoint(ctx, runStageBeforeHardwareSettings); err != nil {
		return err
	}
	if err := operations.applyHardwareSettings(cfg, dev); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	if err := operations.checkpoint(ctx, runStageAfterHardwareSettings); err != nil {
		return err
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

	if err := operations.checkpoint(ctx, runStageBeforeV4L2Setup); err != nil {
		return err
	}
	pixFmt := v4l2PixFmtFor(sensor, cm)
	v4l2, err := operations.setupV4L2(
		devicePath,
		uint32(v4l2OutW),
		uint32(v4l2OutH),
		pixFmt,
		uint32(bpp),
	)
	if err != nil {
		return err
	}
	defer v4l2.Close()
	if err := operations.checkpoint(ctx, runStageAfterV4L2Setup); err != nil {
		return err
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

	var rateTick <-chan time.Time
	if maxFPS > 0 {
		ticker := time.NewTicker(time.Second / time.Duration(maxFPS))
		defer ticker.Stop()
		rateTick = ticker.C
	}

	return runStreamLoop(ctx, dev, v4l2, streamLoopConfig{
		modelConfig:    modelCfg,
		frameBuilder:   fbCfg,
		legendRenderer: legendR,
		outputWidth:    outW,
		outputHeight:   outH,
		frameBytes:     frameBytes,
		devicePath:     devicePath,
		rateTick:       rateTick,
		statusWriter:   os.Stderr,
	})
}

func runStreamLoop(
	ctx context.Context,
	reader frameReader,
	output io.Writer,
	cfg streamLoopConfig,
) error {
	statusWriter := cfg.statusWriter
	if statusWriter == nil {
		statusWriter = os.Stderr
	}
	var (
		frameCount, periodFrames              uint64
		readErrors, extractErrors, sizeErrors uint64
		firstFrameTime                        time.Time
		lastStatus                            = time.Now()
	)

normalExit:
	for {
		select {
		case <-ctx.Done():
			break normalExit
		default:
		}

		rawFrame, err := reader.ReadFrame(ctx)
		if err != nil {
			if context.Cause(ctx) != nil {
				break normalExit
			}
			readErrors++
			reportDropped(statusWriter, &lastStatus, &periodFrames, frameCount, readErrors, extractErrors, sizeErrors)
			continue
		}

		pixelData, _, _, _, thermal, ok := thermalmaster.BuildPixels(
			rawFrame,
			cfg.modelConfig,
			cfg.frameBuilder,
		)
		if !ok {
			extractErrors++
			reportDropped(statusWriter, &lastStatus, &periodFrames, frameCount, readErrors, extractErrors, sizeErrors)
			continue
		}

		// Apply legend overlay if enabled.
		if cfg.legendRenderer != nil && thermal != nil {
			tMin, tMax := thermalmaster.ThermalMinMax(thermal)
			result := cfg.legendRenderer.Apply(
				pixelData,
				thermalmaster.PixelFormatRGB24,
				cfg.outputWidth,
				cfg.outputHeight,
				tMin,
				tMax,
			)
			if result != nil {
				pixelData = thermalmaster.RGBAToRGB24(result)
			}
		}

		if len(pixelData) != cfg.frameBytes {
			fmt.Fprintf(statusWriter, "Warning: frame size mismatch: got %d, want %d\n",
				len(pixelData), cfg.frameBytes)
			sizeErrors++
			reportDropped(statusWriter, &lastStatus, &periodFrames, frameCount, readErrors, extractErrors, sizeErrors)
			continue
		}

		if _, err := output.Write(pixelData); err != nil {
			return fmt.Errorf("writing to %s: %w", cfg.devicePath, err)
		}

		frameCount++
		periodFrames++
		if frameCount == 1 {
			firstFrameTime = time.Now()
		}

		reportDropped(statusWriter, &lastStatus, &periodFrames, frameCount, readErrors, extractErrors, sizeErrors)

		if cfg.rateTick != nil {
			select {
			case <-ctx.Done():
				break normalExit
			case <-cfg.rateTick:
			}
		}
	}
	writeStoppedSummary(statusWriter, frameCount, firstFrameTime, readErrors, extractErrors, sizeErrors)
	return nil
}

func writeStoppedSummary(
	w io.Writer,
	frameCount uint64,
	firstFrameTime time.Time,
	readErrors uint64,
	extractErrors uint64,
	sizeErrors uint64,
) {
	if frameCount > 0 {
		elapsed := time.Since(firstFrameTime)
		actualFPS := float64(frameCount) / elapsed.Seconds()
		fmt.Fprintf(w, "\nStopped. %d frames in %v (%.1f fps), dropped: %d read / %d extract / %d size\n",
			frameCount, elapsed.Round(time.Millisecond), actualFPS, readErrors, extractErrors, sizeErrors)
		return
	}
	fmt.Fprintf(w, "\nStopped. 0 frames, dropped: %d read / %d extract / %d size\n",
		readErrors, extractErrors, sizeErrors)
}

func joinDeviceCloseError(
	resultErr *error,
	dev *thermalmaster.Device,
) {
	if err := dev.Close(); err != nil {
		*resultErr = errors.Join(*resultErr, fmt.Errorf("closing camera: %w", err))
	}
}

func reportDropped(
	w io.Writer,
	lastStatus *time.Time,
	periodFrames *uint64,
	frameCount, readErrors, extractErrors, sizeErrors uint64,
) {
	now := time.Now()
	if now.Sub(*lastStatus) < 5*time.Second {
		return
	}

	periodFPS := float64(*periodFrames) / now.Sub(*lastStatus).Seconds()
	fmt.Fprintf(w, "  %d frames (%.1f fps), dropped: %d read / %d extract / %d size\n",
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
