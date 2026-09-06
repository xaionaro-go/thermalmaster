package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xaionaro-go/thermalmaster/internal/cliflags"
	"github.com/xaionaro-go/thermalmaster/pkg/thermalmaster"
)

func TestCommandPassesExecuteContextThroughRunAndCleanup(t *testing.T) {
	cause := errors.New("signal context canceled")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	var gotCtx context.Context
	cmd := newCommand(func(
		ctx context.Context,
		_ cliflags.Config,
		_ string,
		_ int,
		_ string,
		_ string,
	) error {
		gotCtx = ctx
		return nil
	})
	cmd.SetArgs([]string{"/dev/video-test"})

	err := cmd.ExecuteContext(ctx)

	require.NoError(t, err)
	require.NotNil(t, gotCtx)
	assert.ErrorIs(t, context.Cause(gotCtx), cause)
}

func TestExecuteSecondSignalUsesDefaultActionDuringBlockedDeviceClose(t *testing.T) {
	if os.Getenv("THERMALMASTER_DEVICE_SIGNAL_HELPER") == "1" {
		runDeviceSignalCleanupHelper(t)
		return
	}

	safetyCtx, cancelSafety := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancelSafety()
	cmd := exec.CommandContext(
		safetyCtx,
		os.Args[0],
		"-test.run=^TestExecuteSecondSignalUsesDefaultActionDuringBlockedDeviceClose$",
	)
	cmd.Env = append(os.Environ(), "THERMALMASTER_DEVICE_SIGNAL_HELPER=1")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start())
	scanner := bufio.NewScanner(stdout)
	require.True(t, scanner.Scan(), "helper did not become ready")
	require.Equal(t, "run-ready", scanner.Text())

	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))
	require.True(t, scanner.Scan(), "helper did not enter cleanup")
	require.Equal(t, "cleanup-alt0-entered", scanner.Text())
	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))
	waitResult := make(chan error, 1)
	go func() { waitResult <- cmd.Wait() }()
	select {
	case err := <-waitResult:
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr)
		status, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus)
		require.True(t, ok)
		assert.True(t, status.Signaled())
		assert.Equal(t, syscall.SIGINT, status.Signal())
	case <-time.After(time.Second):
		require.NoError(t, cmd.Process.Kill())
		<-waitResult
		t.Fatal("second SIGINT did not take the default action while Device.Close was blocked in alt0")
	}
}

func runDeviceSignalCleanupHelper(t *testing.T) {
	t.Helper()
	transport := &blockingDeviceCleanupTransport{}
	operations := &deviceSignalRunOperations{transport: transport}
	err := execute([]string{"/dev/video-test"}, func(
		ctx context.Context,
		cfg cliflags.Config,
		devicePath string,
		maxFPS int,
		cpuProfile string,
		logLevel string,
	) error {
		return runWithOperations(
			ctx,
			cfg,
			devicePath,
			maxFPS,
			cpuProfile,
			logLevel,
			operations,
		)
	})
	t.Fatalf("helper escaped blocked Device.Close: %v", err)
}

type deviceSignalRunOperations struct {
	transport *blockingDeviceCleanupTransport
}

func (*deviceSignalRunOperations) checkpoint(ctx context.Context, stage runStage) error {
	if err := context.Cause(ctx); err != nil {
		return &runStageError{stage: stage, err: err}
	}
	return nil
}

func (o *deviceSignalRunOperations) setupCamera(
	cliflags.Config,
) (*thermalmaster.Device, thermalmaster.DeviceInfo, error) {
	device := thermalmaster.NewDeviceWithTransport(o.transport, thermalmaster.ConfigP3)
	return device, thermalmaster.DeviceInfo{}, nil
}

func (*deviceSignalRunOperations) startStreaming(
	ctx context.Context,
	device *thermalmaster.Device,
) error {
	return device.StartStreaming(ctx)
}

func (*deviceSignalRunOperations) applyHardwareSettings(
	cliflags.Config,
	*thermalmaster.Device,
) error {
	return nil
}

func (*deviceSignalRunOperations) setupV4L2(
	string,
	uint32,
	uint32,
	uint32,
	uint32,
) (io.WriteCloser, error) {
	fmt.Fprintln(os.Stdout, "run-ready")
	return discardWriteCloser{Writer: io.Discard}, nil
}

type discardWriteCloser struct {
	io.Writer
}

func (discardWriteCloser) Close() error {
	return nil
}

type blockingDeviceCleanupTransport struct{}

func (*blockingDeviceCleanupTransport) Control(
	_ uint8,
	request uint8,
	_ uint16,
	_ uint16,
	data []byte,
) (int, error) {
	if request == 0x22 && len(data) == 1 {
		data[0] = 2
	}
	return len(data), nil
}

func (*blockingDeviceCleanupTransport) BulkRead(
	ctx context.Context,
	_ uint8,
	_ []byte,
) (int, error) {
	<-ctx.Done()
	return 0, context.Cause(ctx)
}

func (*blockingDeviceCleanupTransport) ActivateStreamingInterface() (
	thermalmaster.StreamingInterfacePhase,
	error,
) {
	return thermalmaster.StreamingInterfaceActive, nil
}

func (*blockingDeviceCleanupTransport) SetStreamingInterfaceIdle() error {
	fmt.Fprintln(os.Stdout, "cleanup-alt0-entered")
	select {}
}

func (*blockingDeviceCleanupTransport) ReleaseStreamingInterface() error {
	return nil
}

func (*blockingDeviceCleanupTransport) Close() error {
	return nil
}

func TestRunWithOperationsChecksCancellationAtEveryTypedStage(t *testing.T) {
	stages := []runStage{
		runStageBeforeCameraSetup,
		runStageAfterCameraSetup,
		runStageBeforeStreamStart,
		runStageAfterStreamStart,
		runStageBeforeHardwareSettings,
		runStageAfterHardwareSettings,
		runStageBeforeV4L2Setup,
		runStageAfterV4L2Setup,
	}
	for _, stage := range stages {
		t.Run(stage.String(), func(t *testing.T) {
			cause := errors.New("injected cancellation")
			ctx, cancel := context.WithCancelCause(context.Background())
			operations := newCancelingRunOperations(stage, cancel, cause)

			err := runWithOperations(
				ctx,
				preLoopTestConfig(),
				"/dev/video-test",
				0,
				"",
				"warning",
				operations,
			)

			assert.ErrorIs(t, err, cause)
			var stageErr *runStageError
			require.ErrorAs(t, err, &stageErr)
			assert.Equal(t, stage, stageErr.stage)
		})
	}
}

type closeResultTransport struct {
	err   error
	calls int
}

func (t *closeResultTransport) Control(
	requestType uint8,
	request uint8,
	value uint16,
	index uint16,
	data []byte,
) (int, error) {
	return 0, nil
}

func (t *closeResultTransport) Close() error {
	t.calls++
	return t.err
}

func TestJoinDeviceCloseErrorPreservesPrimaryAndCleanupFailures(t *testing.T) {
	primaryErr := errors.New("stream loop failed")
	cleanupErr := errors.New("camera restoration failed")
	transport := &closeResultTransport{err: cleanupErr}
	device := thermalmaster.NewDeviceWithTransport(transport, thermalmaster.ConfigP3)
	resultErr := primaryErr

	joinDeviceCloseError(&resultErr, device)

	assert.ErrorIs(t, resultErr, primaryErr)
	assert.ErrorIs(t, resultErr, cleanupErr)
	assert.Equal(t, 1, transport.calls)
}

func TestJoinDeviceCloseErrorReturnsCleanupFailureWithoutPrimary(t *testing.T) {
	cleanupErr := errors.New("camera restoration failed")
	transport := &closeResultTransport{err: cleanupErr}
	device := thermalmaster.NewDeviceWithTransport(transport, thermalmaster.ConfigP3)
	var resultErr error

	joinDeviceCloseError(&resultErr, device)

	assert.ErrorIs(t, resultErr, cleanupErr)
	assert.Equal(t, 1, transport.calls)
}

func TestJoinDeviceCloseErrorPreservesPrimaryOnSuccessfulCleanup(t *testing.T) {
	primaryErr := errors.New("stream loop failed")
	transport := &closeResultTransport{}
	device := thermalmaster.NewDeviceWithTransport(transport, thermalmaster.ConfigP3)
	resultErr := primaryErr

	joinDeviceCloseError(&resultErr, device)

	assert.ErrorIs(t, resultErr, primaryErr)
	assert.Equal(t, primaryErr, resultErr)
	assert.Equal(t, 1, transport.calls)
}

func TestJoinDeviceCloseErrorLeavesNilOnSuccessfulCleanup(t *testing.T) {
	transport := &closeResultTransport{}
	device := thermalmaster.NewDeviceWithTransport(transport, thermalmaster.ConfigP3)
	var resultErr error

	joinDeviceCloseError(&resultErr, device)

	assert.NoError(t, resultErr)
	assert.Equal(t, 1, transport.calls)
}

type cancelingRunOperations struct {
	target runStage
	cancel context.CancelCauseFunc
	cause  error
	device *thermalmaster.Device
}

func newCancelingRunOperations(
	target runStage,
	cancel context.CancelCauseFunc,
	cause error,
) *cancelingRunOperations {
	return &cancelingRunOperations{
		target: target,
		cancel: cancel,
		cause:  cause,
		device: thermalmaster.NewDeviceWithTransport(&stageTransport{}, thermalmaster.ConfigP3),
	}
}

func (o *cancelingRunOperations) checkpoint(ctx context.Context, stage runStage) error {
	if o.target == runStageBeforeCameraSetup && stage == runStageBeforeCameraSetup {
		o.cancel(o.cause)
	}
	if err := context.Cause(ctx); err != nil {
		return &runStageError{stage: stage, err: err}
	}
	if previousStage(o.target) == stage {
		o.cancel(o.cause)
	}
	return nil
}

func (o *cancelingRunOperations) setupCamera(
	cliflags.Config,
) (*thermalmaster.Device, thermalmaster.DeviceInfo, error) {
	if o.target == runStageBeforeCameraSetup || o.target == runStageAfterCameraSetup {
		o.cancel(o.cause)
	}
	return o.device, thermalmaster.DeviceInfo{}, nil
}

func (o *cancelingRunOperations) startStreaming(
	context.Context,
	*thermalmaster.Device,
) error {
	if o.target == runStageBeforeStreamStart || o.target == runStageAfterStreamStart {
		o.cancel(o.cause)
	}
	return nil
}

func (o *cancelingRunOperations) applyHardwareSettings(
	cliflags.Config,
	*thermalmaster.Device,
) error {
	if o.target == runStageBeforeHardwareSettings || o.target == runStageAfterHardwareSettings {
		o.cancel(o.cause)
	}
	return nil
}

func (o *cancelingRunOperations) setupV4L2(
	string,
	uint32,
	uint32,
	uint32,
	uint32,
) (io.WriteCloser, error) {
	if o.target == runStageBeforeV4L2Setup || o.target == runStageAfterV4L2Setup {
		o.cancel(o.cause)
	}
	return stageWriteCloser{Writer: io.Discard}, nil
}

func previousStage(target runStage) runStage {
	switch target {
	case runStageBeforeStreamStart:
		return runStageAfterCameraSetup
	case runStageBeforeHardwareSettings:
		return runStageAfterStreamStart
	case runStageBeforeV4L2Setup:
		return runStageAfterHardwareSettings
	default:
		return runStage(255)
	}
}

type stageTransport struct{}

func (*stageTransport) Control(uint8, uint8, uint16, uint16, []byte) (int, error) {
	return 0, nil
}

func (*stageTransport) Close() error {
	return nil
}

type stageWriteCloser struct {
	io.Writer
}

func (stageWriteCloser) Close() error {
	return nil
}

func preLoopTestConfig() cliflags.Config {
	return cliflags.Config{
		Sensor:     "ir",
		Colormap:   "none",
		USBBus:     -1,
		USBAddr:    -1,
		Gain:       "auto",
		Brightness: -1,
		Contrast:   -1,
	}
}

func TestRunStreamLoopTopCancellationWritesExactlyOneSummary(t *testing.T) {
	cause := errors.New("stop before read")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	var summary bytes.Buffer
	reader := frameReaderFunc(func(context.Context) ([]byte, error) {
		t.Fatal("ReadFrame called after top-of-loop cancellation")
		return nil, nil
	})

	err := runStreamLoop(ctx, reader, io.Discard, testStreamLoopConfig(&summary))

	assert.NoError(t, err)
	assert.Equal(t, 1, strings.Count(summary.String(), "Stopped."))
}

func TestRunStreamLoopReadCancellationWritesExactlyOneSummary(t *testing.T) {
	cause := errors.New("stop blocked read")
	ctx, cancel := context.WithCancelCause(context.Background())
	readStarted := make(chan struct{})
	reader := frameReaderFunc(func(ctx context.Context) ([]byte, error) {
		close(readStarted)
		<-ctx.Done()
		return nil, context.Cause(ctx)
	})
	var summary bytes.Buffer
	result := make(chan error, 1)
	go func() {
		result <- runStreamLoop(ctx, reader, io.Discard, testStreamLoopConfig(&summary))
	}()
	<-readStarted
	cancel(cause)

	assert.NoError(t, <-result)
	assert.Equal(t, 1, strings.Count(summary.String(), "Stopped."))
}

func TestRunStreamLoopRateWaitCancellationWritesExactlyOneSummary(t *testing.T) {
	cause := errors.New("stop blocked rate wait")
	ctx, cancel := context.WithCancelCause(context.Background())
	reader := frameReaderFunc(func(context.Context) ([]byte, error) {
		return testIRFrame(), nil
	})
	writeStarted := make(chan struct{})
	output := &writeSignalWriter{w: io.Discard, wrote: writeStarted}
	var summary bytes.Buffer
	cfg := testStreamLoopConfig(&summary)
	cfg.rateTick = make(chan time.Time)
	result := make(chan error, 1)
	go func() {
		result <- runStreamLoop(ctx, reader, output, cfg)
	}()
	<-writeStarted
	cancel(cause)

	assert.NoError(t, <-result)
	assert.Equal(t, 1, strings.Count(summary.String(), "Stopped."))
}

func TestRunStreamLoopWriterFailureDoesNotWriteStoppedSummary(t *testing.T) {
	writeErr := errors.New("v4l2 write failed")
	reader := frameReaderFunc(func(context.Context) ([]byte, error) {
		return testIRFrame(), nil
	})
	var summary bytes.Buffer

	err := runStreamLoop(
		context.Background(),
		reader,
		errorWriter{err: writeErr},
		testStreamLoopConfig(&summary),
	)

	assert.ErrorIs(t, err, writeErr)
	assert.Zero(t, strings.Count(summary.String(), "Stopped."))
}

type frameReaderFunc func(context.Context) ([]byte, error)

func (f frameReaderFunc) ReadFrame(ctx context.Context) ([]byte, error) {
	return f(ctx)
}

type writeSignalWriter struct {
	w     io.Writer
	wrote chan struct{}
}

func (w *writeSignalWriter) Write(data []byte) (int, error) {
	n, err := w.w.Write(data)
	select {
	case <-w.wrote:
	default:
		close(w.wrote)
	}
	return n, err
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func testStreamLoopConfig(statusWriter io.Writer) streamLoopConfig {
	return streamLoopConfig{
		modelConfig: thermalmaster.ConfigP3,
		frameBuilder: thermalmaster.FrameBuilderConfig{
			Sensor: thermalmaster.SensorIR,
		},
		outputWidth:  thermalmaster.ConfigP3.SensorW,
		outputHeight: thermalmaster.ConfigP3.SensorH,
		frameBytes:   thermalmaster.ConfigP3.SensorW * thermalmaster.ConfigP3.SensorH,
		devicePath:   "/dev/video-test",
		statusWriter: statusWriter,
	}
}

func testIRFrame() []byte {
	return make([]byte, thermalmaster.MarkerSize+thermalmaster.ConfigP3.FrameSize())
}
