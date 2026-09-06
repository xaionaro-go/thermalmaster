package thermalmaster

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	readAdmissionContestAttempts    = 10_000
	readAdmissionContestTimeout     = 30 * time.Second
	controlAdmissionContestAttempts = 1_000
	controlAdmissionTarget          = 100
	controlLifecyclePhaseTimeout    = 2 * time.Second
	controlLifecycleCleanupTimeout  = 5 * time.Second
	controlAdmissionContestTimeout  = 30 * time.Second
)

func TestUSBStreamingTransportBulkReadIsContextFirst(t *testing.T) {
	method, ok := reflect.TypeOf((*USBStreamingTransport)(nil)).Elem().MethodByName("BulkRead")
	require.True(t, ok)
	require.Equal(t, 3, method.Type.NumIn())
	assert.Equal(t, reflect.TypeOf((*context.Context)(nil)).Elem(), method.Type.In(0))
	assert.Equal(t, reflect.TypeOf(uint8(0)), method.Type.In(1))
	assert.Equal(t, reflect.TypeOf([]byte(nil)), method.Type.In(2))
}

func setMockStreamingActive(
	dev *Device,
	transport *mockTransport,
) {
	dev.stateMu.Lock()
	defer dev.stateMu.Unlock()
	dev.streamingPhase = StreamingInterfaceActive

	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.streamingPhase = StreamingInterfaceActive
	transport.currentAlt[streamingIntf] = streamingAltStart
}

func TestStopStreamingKeepsActiveStateWhenIdleTransitionFails(t *testing.T) {
	dev, transport := newMockDevice()
	setMockStreamingActive(dev, transport)
	idleErr := errors.New("idle transition failed")
	transport.idleError = idleErr

	err := dev.StopStreaming()

	assert.ErrorIs(t, err, idleErr)
	assert.True(t, dev.IsStreaming())
	assert.False(t, transport.closed)
	assert.Equal(t, []string{"set-streaming-interface-idle"}, transport.operationSnapshot())
}

func TestStopStreamingRetriesIdleTransitionBeforeRelease(t *testing.T) {
	dev, transport := newMockDevice()
	setMockStreamingActive(dev, transport)
	idleErr := errors.New("idle transition failed")
	transport.idleError = idleErr

	firstErr := dev.StopStreaming()
	secondErr := dev.StopStreaming()

	assert.ErrorIs(t, firstErr, idleErr)
	assert.NoError(t, secondErr)
	assert.False(t, dev.IsStreaming())
	assert.Equal(t, []string{
		"set-streaming-interface-idle",
		"set-streaming-interface-idle",
		"release-streaming-interface",
	}, transport.operationSnapshot())
}

func TestCloseDoesNotDestroyTransportWhenRestorationFails(t *testing.T) {
	dev, transport := newMockDevice()
	setMockStreamingActive(dev, transport)
	idleErr := errors.New("idle transition failed")
	transport.idleError = idleErr

	err := dev.Close()

	assert.ErrorIs(t, err, idleErr)
	assert.True(t, dev.IsStreaming())
	assert.False(t, transport.closed)
	assert.Zero(t, transport.closeCalls)
	assert.Equal(t, []string{"set-streaming-interface-idle"}, transport.operationSnapshot())
}

func TestCloseRestoresInterfaceBeforeClosingTransport(t *testing.T) {
	dev, transport := newMockDevice()
	setMockStreamingActive(dev, transport)

	err := dev.Close()

	require.NoError(t, err)
	assert.False(t, dev.IsStreaming())
	assert.True(t, transport.closed)
	assert.Equal(t, []string{
		"set-streaming-interface-idle",
		"release-streaming-interface",
		"close",
	}, transport.operationSnapshot())
}

func TestCloseCancelsAndDrainsBlockedReadBeforeInterfaceRestoration(t *testing.T) {
	dev, transport := newMockDevice()
	setMockStreamingActive(dev, transport)
	readStarted := make(chan struct{})
	readDone := make(chan struct{})
	transport.bulkRead = func(ctx context.Context, _ uint8, _ []byte) (int, error) {
		close(readStarted)
		<-ctx.Done()
		close(readDone)
		return 0, context.Cause(ctx)
	}
	parentCtx, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	readResult := make(chan error, 1)
	go func() {
		_, err := dev.ReadFrame(parentCtx)
		readResult <- err
	}()
	<-readStarted

	closeResult := make(chan error, 1)
	go func() { closeResult <- dev.Close() }()

	var closeErr error
	closeReturned := false
	select {
	case readErr := <-readResult:
		assert.ErrorIs(t, readErr, errStreamingStopped)
	case closeErr = <-closeResult:
		closeReturned = true
		select {
		case readErr := <-readResult:
			assert.ErrorIs(t, readErr, errStreamingStopped)
		default:
			cancelParent()
			assert.Failf(t, "close returned before read drained", "Close error: %v", closeErr)
			<-readDone
			<-readResult
		}
	}
	if !closeReturned {
		closeErr = <-closeResult
	}
	assert.NoError(t, closeErr)
	assert.Equal(t, []string{
		"bulk-read:81",
		"set-streaming-interface-idle",
		"release-streaming-interface",
		"close",
	}, transport.operationSnapshot())
}

func TestStopCancelsAndDrainsBlockedReadBeforeInterfaceRestoration(t *testing.T) {
	dev, transport := newMockDevice()
	setMockStreamingActive(dev, transport)
	readStarted := make(chan struct{})
	readReturned := make(chan struct{})
	transport.bulkRead = func(ctx context.Context, _ uint8, _ []byte) (int, error) {
		close(readStarted)
		<-ctx.Done()
		close(readReturned)
		return 0, context.Cause(ctx)
	}
	readResult := make(chan error, 1)
	go func() {
		_, err := dev.ReadFrame(context.Background())
		readResult <- err
	}()
	<-readStarted

	stopResult := make(chan error, 1)
	go func() { stopResult <- dev.StopStreaming() }()

	<-readReturned
	assert.ErrorIs(t, <-readResult, errStreamingStopped)
	assert.NoError(t, <-stopResult)
	assert.Equal(t, []string{
		"bulk-read:81",
		"set-streaming-interface-idle",
		"release-streaming-interface",
	}, transport.operationSnapshot())
}

type drainBeforeUSBTeardownOutcome struct {
	phaseErr, cleanupErr                  error
	readErr, firstControlErr, teardownErr error
	teardownReturnedBeforeRelease         bool
	events                                []string
	readCancelCount                       int
	unfinishedWorkers                     []string
}

type postGateControlOutcome struct {
	phaseErr, cleanupErr                      error
	firstControlErr, postGateErr, teardownErr error
	afterTeardownErr                          error
	postGateReturnedBeforeRelease             bool
	teardownReturnedBeforeRelease             bool
	eventsBeforeRelease, eventsAfterCleanup   []string
	unfinishedWorkers                         []string
}

type closeControlAdmissionOutcome struct {
	phaseErr, cleanupErr                        error
	firstControlErr, secondControlErr, closeErr error
	afterCloseErr                               error
	events                                      []string
	unfinishedWorkers                           []string
}

func waitForLifecycleTestSignal(
	ctx context.Context,
	signal <-chan struct{},
) bool {
	select {
	case <-signal:
		return true
	case <-ctx.Done():
		return false
	}
}

func waitForLifecycleTestResult(
	ctx context.Context,
	result <-chan error,
) (error, bool) {
	select {
	case err := <-result:
		return err, true
	case <-ctx.Done():
		return nil, false
	}
}

func waitForDeviceLifecycle(
	ctx context.Context,
	dev *Device,
	want deviceLifecycle,
) error {
	for {
		dev.stateMu.Lock()
		got := dev.lifecycle
		dev.stateMu.Unlock()
		if got == want {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for device lifecycle %s: %w", want, context.Cause(ctx))
		default:
			runtime.Gosched()
		}
	}
}

func runDrainBeforeUSBTeardownScenario(
	parent context.Context,
	teardown func(*Device) error,
	wantLifecycle deviceLifecycle,
) drainBeforeUSBTeardownOutcome {
	transport := newOrderedDrainTransport()
	dev := NewDeviceWithTransport(transport, ConfigP3)
	dev.streamingPhase = StreamingInterfaceActive
	transport.streamingPhase = StreamingInterfaceActive

	phaseCtx, cancelPhase := context.WithTimeout(parent, controlLifecyclePhaseTimeout)
	readCtx, cancelRead := context.WithCancel(parent)
	var releaseControl sync.Once
	defer func() {
		releaseControl.Do(func() { close(transport.releaseFirstControl) })
		cancelRead()
		cancelPhase()
	}()

	var outcome drainBeforeUSBTeardownOutcome
	readResult := make(chan error, 1)
	readDone := make(chan struct{})
	readLaunched := true
	go func() {
		defer close(readDone)
		_, err := dev.ReadFrame(readCtx)
		transport.record("read-finish")
		readResult <- err
	}()
	if !waitForLifecycleTestSignal(phaseCtx, transport.readStarted) {
		outcome.phaseErr = fmt.Errorf("waiting for read transport entry: %w", context.Cause(phaseCtx))
	}

	firstControlResult := make(chan error, 1)
	firstControlDone := make(chan struct{})
	firstControlLaunched := false
	if outcome.phaseErr == nil {
		firstControlLaunched = true
		go func() {
			defer close(firstControlDone)
			err := dev.SendCommandNoResponse(CmdShutter)
			transport.record("control-1-finish")
			firstControlResult <- err
		}()
		if !waitForLifecycleTestSignal(phaseCtx, transport.firstControlStarted) {
			outcome.phaseErr = fmt.Errorf("waiting for first control transport entry: %w", context.Cause(phaseCtx))
		}
	}

	teardownResult := make(chan error, 1)
	teardownDone := make(chan struct{})
	teardownLaunched := false
	readResultReceived := false
	teardownResultReceived := false
	if outcome.phaseErr == nil {
		teardownLaunched = true
		go func() {
			defer close(teardownDone)
			teardownResult <- teardown(dev)
		}()
		outcome.phaseErr = waitForDeviceLifecycle(phaseCtx, dev, wantLifecycle)
	}
	if outcome.phaseErr == nil {
		outcome.readErr, readResultReceived = waitForLifecycleTestResult(phaseCtx, readResult)
		if !readResultReceived {
			outcome.phaseErr = fmt.Errorf("waiting for admitted read return: %w", context.Cause(phaseCtx))
		}
	}
	if outcome.phaseErr == nil {
		select {
		case outcome.teardownErr = <-teardownResult:
			outcome.teardownReturnedBeforeRelease = true
			teardownResultReceived = true
		default:
		}
	}

	releaseControl.Do(func() { close(transport.releaseFirstControl) })
	cancelRead()
	cancelPhase()

	cleanupCtx, cancelCleanup := context.WithTimeout(parent, controlLifecycleCleanupTimeout)
	defer cancelCleanup()
	if readLaunched {
		resultReceived := readResultReceived
		if !resultReceived {
			outcome.readErr, resultReceived = waitForLifecycleTestResult(cleanupCtx, readResult)
		}
		doneReceived := waitForLifecycleTestSignal(cleanupCtx, readDone)
		if !resultReceived || !doneReceived {
			outcome.unfinishedWorkers = append(outcome.unfinishedWorkers, "read")
		}
	}
	if firstControlLaunched {
		resultReceived := false
		outcome.firstControlErr, resultReceived = waitForLifecycleTestResult(cleanupCtx, firstControlResult)
		doneReceived := waitForLifecycleTestSignal(cleanupCtx, firstControlDone)
		if !resultReceived || !doneReceived {
			outcome.unfinishedWorkers = append(outcome.unfinishedWorkers, "control-1")
		}
	}
	if teardownLaunched {
		resultReceived := teardownResultReceived
		if !resultReceived {
			outcome.teardownErr, resultReceived = waitForLifecycleTestResult(cleanupCtx, teardownResult)
		}
		doneReceived := waitForLifecycleTestSignal(cleanupCtx, teardownDone)
		if !resultReceived || !doneReceived {
			outcome.unfinishedWorkers = append(outcome.unfinishedWorkers, "teardown")
		}
	}
	if len(outcome.unfinishedWorkers) != 0 {
		outcome.cleanupErr = fmt.Errorf(
			"cleanup deadline exceeded: %w; unfinished workers: %v",
			context.Cause(cleanupCtx),
			outcome.unfinishedWorkers,
		)
		outcome.unfinishedWorkers = append([]string(nil), outcome.unfinishedWorkers...)
		return outcome
	}

	outcome.events = transport.eventSnapshot()
	dev.stateMu.Lock()
	outcome.readCancelCount = len(dev.readCancels)
	dev.stateMu.Unlock()
	outcome.unfinishedWorkers = append([]string(nil), outcome.unfinishedWorkers...)
	return outcome
}

func runPostGateControlRejectionScenario(
	parent context.Context,
	teardown func(*Device) error,
	wantLifecycle deviceLifecycle,
) postGateControlOutcome {
	transport := newOrderedDrainTransport()
	dev := NewDeviceWithTransport(transport, ConfigP3)
	dev.streamingPhase = StreamingInterfaceActive
	transport.streamingPhase = StreamingInterfaceActive

	phaseCtx, cancelPhase := context.WithTimeout(parent, controlLifecyclePhaseTimeout)
	postGateReady := make(chan struct{})
	postGateStart := make(chan struct{})
	postGateApproached := make(chan struct{})
	var releaseControl sync.Once
	var releasePostGate sync.Once
	defer func() {
		releasePostGate.Do(func() { close(postGateStart) })
		releaseControl.Do(func() { close(transport.releaseFirstControl) })
		cancelPhase()
	}()

	var outcome postGateControlOutcome
	firstControlResult := make(chan error, 1)
	firstControlDone := make(chan struct{})
	firstControlLaunched := true
	go func() {
		defer close(firstControlDone)
		firstControlResult <- dev.SendCommandNoResponse(CmdShutter)
	}()
	if !waitForLifecycleTestSignal(phaseCtx, transport.firstControlStarted) {
		outcome.phaseErr = fmt.Errorf("waiting for first control transport entry: %w", context.Cause(phaseCtx))
	}

	postGateResult := make(chan error, 1)
	postGateDone := make(chan struct{})
	postGateLaunched := false
	if outcome.phaseErr == nil {
		postGateLaunched = true
		go func() {
			defer close(postGateDone)
			close(postGateReady)
			<-postGateStart
			close(postGateApproached)
			postGateResult <- dev.SendCommandNoResponse(CmdShutter)
		}()
		if !waitForLifecycleTestSignal(phaseCtx, postGateReady) {
			outcome.phaseErr = fmt.Errorf("waiting for post-gate control readiness: %w", context.Cause(phaseCtx))
		}
	}

	teardownResult := make(chan error, 1)
	afterTeardownResult := make(chan error, 1)
	teardownDone := make(chan struct{})
	teardownLaunched := false
	teardownResultReceived := false
	postGateResultReceived := false
	if outcome.phaseErr == nil {
		teardownLaunched = true
		go func() {
			defer close(teardownDone)
			teardownResult <- teardown(dev)
			afterTeardownResult <- dev.SendCommandNoResponse(CmdShutter)
		}()
		outcome.phaseErr = waitForDeviceLifecycle(phaseCtx, dev, wantLifecycle)
	}
	if outcome.phaseErr == nil {
		releasePostGate.Do(func() { close(postGateStart) })
		if !waitForLifecycleTestSignal(phaseCtx, postGateApproached) {
			outcome.phaseErr = fmt.Errorf("waiting for post-gate control approach: %w", context.Cause(phaseCtx))
		}
	}
	if outcome.phaseErr == nil {
		outcome.postGateErr, postGateResultReceived = waitForLifecycleTestResult(phaseCtx, postGateResult)
		outcome.postGateReturnedBeforeRelease = postGateResultReceived
		if !postGateResultReceived {
			outcome.phaseErr = fmt.Errorf("waiting for post-gate control return: %w", context.Cause(phaseCtx))
		}
	}
	if outcome.phaseErr == nil {
		outcome.eventsBeforeRelease = transport.eventSnapshot()
		select {
		case outcome.teardownErr = <-teardownResult:
			outcome.teardownReturnedBeforeRelease = true
			teardownResultReceived = true
		default:
		}
	}

	releasePostGate.Do(func() { close(postGateStart) })
	releaseControl.Do(func() { close(transport.releaseFirstControl) })
	cancelPhase()

	cleanupCtx, cancelCleanup := context.WithTimeout(parent, controlLifecycleCleanupTimeout)
	defer cancelCleanup()
	if firstControlLaunched {
		resultReceived := false
		outcome.firstControlErr, resultReceived = waitForLifecycleTestResult(cleanupCtx, firstControlResult)
		doneReceived := waitForLifecycleTestSignal(cleanupCtx, firstControlDone)
		if !resultReceived || !doneReceived {
			outcome.unfinishedWorkers = append(outcome.unfinishedWorkers, "control-1")
		}
	}
	if postGateLaunched {
		resultReceived := postGateResultReceived
		if !resultReceived {
			outcome.postGateErr, resultReceived = waitForLifecycleTestResult(cleanupCtx, postGateResult)
		}
		doneReceived := waitForLifecycleTestSignal(cleanupCtx, postGateDone)
		if !resultReceived || !doneReceived {
			outcome.unfinishedWorkers = append(outcome.unfinishedWorkers, "post-gate-control")
		}
	}
	if teardownLaunched {
		resultReceived := teardownResultReceived
		if !resultReceived {
			outcome.teardownErr, resultReceived = waitForLifecycleTestResult(cleanupCtx, teardownResult)
		}
		policyReceived := false
		outcome.afterTeardownErr, policyReceived = waitForLifecycleTestResult(cleanupCtx, afterTeardownResult)
		doneReceived := waitForLifecycleTestSignal(cleanupCtx, teardownDone)
		if !resultReceived || !policyReceived || !doneReceived {
			outcome.unfinishedWorkers = append(outcome.unfinishedWorkers, "teardown")
		}
	}
	if len(outcome.unfinishedWorkers) != 0 {
		outcome.cleanupErr = fmt.Errorf(
			"cleanup deadline exceeded: %w; unfinished workers: %v",
			context.Cause(cleanupCtx),
			outcome.unfinishedWorkers,
		)
		outcome.unfinishedWorkers = append([]string(nil), outcome.unfinishedWorkers...)
		return outcome
	}

	outcome.eventsAfterCleanup = transport.eventSnapshot()
	outcome.eventsBeforeRelease = append([]string(nil), outcome.eventsBeforeRelease...)
	outcome.unfinishedWorkers = append([]string(nil), outcome.unfinishedWorkers...)
	return outcome
}

func runCloseControlAdmissionAttempt(
	contestCtx context.Context,
	cleanupParent context.Context,
) closeControlAdmissionOutcome {
	transport := newOrderedDrainTransport()
	dev := NewDeviceWithTransport(transport, ConfigP3)
	dev.streamingPhase = StreamingInterfaceActive
	transport.streamingPhase = StreamingInterfaceActive

	secondControlReady := make(chan struct{})
	secondControlStart := make(chan struct{})
	secondControlApproached := make(chan struct{})
	closeReady := make(chan struct{})
	closeStart := make(chan struct{})
	closeApproached := make(chan struct{})
	var releaseControl sync.Once
	var releaseSecondControl sync.Once
	var releaseClose sync.Once
	defer func() {
		releaseSecondControl.Do(func() { close(secondControlStart) })
		releaseClose.Do(func() { close(closeStart) })
		releaseControl.Do(func() { close(transport.releaseFirstControl) })
	}()

	var outcome closeControlAdmissionOutcome
	firstControlResult := make(chan error, 1)
	firstControlDone := make(chan struct{})
	firstControlLaunched := true
	go func() {
		defer close(firstControlDone)
		firstControlResult <- dev.SendCommandNoResponse(CmdShutter)
	}()
	if !waitForLifecycleTestSignal(contestCtx, transport.firstControlStarted) {
		outcome.phaseErr = fmt.Errorf("waiting for first control transport entry: %w", context.Cause(contestCtx))
	}

	secondControlResult := make(chan error, 1)
	secondControlDone := make(chan struct{})
	secondControlLaunched := false
	closeResult := make(chan error, 1)
	afterCloseResult := make(chan error, 1)
	closeDone := make(chan struct{})
	closeLaunched := false
	if outcome.phaseErr == nil {
		secondControlLaunched = true
		go func() {
			defer close(secondControlDone)
			close(secondControlReady)
			<-secondControlStart
			close(secondControlApproached)
			secondControlResult <- dev.SendCommandNoResponse(CmdShutter)
		}()
		closeLaunched = true
		go func() {
			defer close(closeDone)
			close(closeReady)
			<-closeStart
			close(closeApproached)
			closeResult <- dev.Close()
			afterCloseResult <- dev.SendCommandNoResponse(CmdShutter)
		}()
		if !waitForLifecycleTestSignal(contestCtx, secondControlReady) {
			outcome.phaseErr = fmt.Errorf("waiting for second control readiness: %w", context.Cause(contestCtx))
		}
		if outcome.phaseErr == nil && !waitForLifecycleTestSignal(contestCtx, closeReady) {
			outcome.phaseErr = fmt.Errorf("waiting for close readiness: %w", context.Cause(contestCtx))
		}
	}

	if outcome.phaseErr == nil {
		func() {
			dev.stateMu.Lock()
			defer dev.stateMu.Unlock()

			releaseSecondControl.Do(func() { close(secondControlStart) })
			if !waitForLifecycleTestSignal(contestCtx, secondControlApproached) {
				outcome.phaseErr = fmt.Errorf("waiting for second control approach: %w", context.Cause(contestCtx))
				return
			}
			releaseClose.Do(func() { close(closeStart) })
			if !waitForLifecycleTestSignal(contestCtx, closeApproached) {
				outcome.phaseErr = fmt.Errorf("waiting for close approach: %w", context.Cause(contestCtx))
			}
		}()
	}
	if outcome.phaseErr == nil {
		outcome.phaseErr = waitForDeviceLifecycle(contestCtx, dev, deviceLifecycleClosing)
	}

	releaseSecondControl.Do(func() { close(secondControlStart) })
	releaseClose.Do(func() { close(closeStart) })
	releaseControl.Do(func() { close(transport.releaseFirstControl) })

	cleanupCtx, cancelCleanup := context.WithTimeout(cleanupParent, controlLifecycleCleanupTimeout)
	defer cancelCleanup()
	if firstControlLaunched {
		resultReceived := false
		outcome.firstControlErr, resultReceived = waitForLifecycleTestResult(cleanupCtx, firstControlResult)
		doneReceived := waitForLifecycleTestSignal(cleanupCtx, firstControlDone)
		if !resultReceived || !doneReceived {
			outcome.unfinishedWorkers = append(outcome.unfinishedWorkers, "control-1")
		}
	}
	if secondControlLaunched {
		resultReceived := false
		outcome.secondControlErr, resultReceived = waitForLifecycleTestResult(cleanupCtx, secondControlResult)
		doneReceived := waitForLifecycleTestSignal(cleanupCtx, secondControlDone)
		if !resultReceived || !doneReceived {
			outcome.unfinishedWorkers = append(outcome.unfinishedWorkers, "control-2")
		}
	}
	if closeLaunched {
		resultReceived := false
		outcome.closeErr, resultReceived = waitForLifecycleTestResult(cleanupCtx, closeResult)
		policyReceived := false
		outcome.afterCloseErr, policyReceived = waitForLifecycleTestResult(cleanupCtx, afterCloseResult)
		doneReceived := waitForLifecycleTestSignal(cleanupCtx, closeDone)
		if !resultReceived || !policyReceived || !doneReceived {
			outcome.unfinishedWorkers = append(outcome.unfinishedWorkers, "close")
		}
	}
	if len(outcome.unfinishedWorkers) != 0 {
		outcome.cleanupErr = fmt.Errorf(
			"cleanup deadline exceeded: %w; unfinished workers: %v",
			context.Cause(cleanupCtx),
			outcome.unfinishedWorkers,
		)
		outcome.unfinishedWorkers = append([]string(nil), outcome.unfinishedWorkers...)
		return outcome
	}

	outcome.events = transport.eventSnapshot()
	outcome.unfinishedWorkers = append([]string(nil), outcome.unfinishedWorkers...)
	return outcome
}

func TestStopAndCloseDrainAllPreGateOperationsBeforeUSBTeardown(t *testing.T) {
	tests := []struct {
		name               string
		teardown           func(*Device) error
		lifecycle          deviceLifecycle
		expectedCloseCount int
	}{
		{
			name:               "stop-drain",
			teardown:           (*Device).StopStreaming,
			lifecycle:          deviceLifecycleStopping,
			expectedCloseCount: 0,
		},
		{
			name:               "close-drain",
			teardown:           (*Device).Close,
			lifecycle:          deviceLifecycleClosing,
			expectedCloseCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := runDrainBeforeUSBTeardownScenario(t.Context(), tt.teardown, tt.lifecycle)

			require.NoError(t, outcome.phaseErr)
			require.NoError(t, outcome.cleanupErr)
			require.Empty(t, outcome.unfinishedWorkers)
			require.False(t, outcome.teardownReturnedBeforeRelease)
			require.ErrorIs(t, outcome.readErr, errStreamingStopped)
			require.ErrorIs(t, outcome.firstControlErr, errStreamingStopped)
			require.NoError(t, outcome.teardownErr)
			require.Zero(t, outcome.readCancelCount)
			require.Equal(t, 1, countEvent(outcome.events, "read-return"))
			require.Equal(t, 1, countEvent(outcome.events, "read-finish"))
			require.Equal(t, 1, countEvent(outcome.events, "control-1-start"))
			require.Equal(t, 1, countEvent(outcome.events, "control-1-return"))
			require.Equal(t, 1, countEvent(outcome.events, "control-1-finish"))
			require.Equal(t, 1, countEvent(outcome.events, "alternate:0"))
			require.Equal(t, 1, countEvent(outcome.events, "release"))
			require.Equal(t, tt.expectedCloseCount, countEvent(outcome.events, "close"))
			assertEventBefore(t, outcome.events, "read-return", "read-finish")
			assertEventBefore(t, outcome.events, "read-return", "alternate:0")
			assertEventBefore(t, outcome.events, "control-1-return", "control-1-finish")
			assertEventBefore(t, outcome.events, "control-1-return", "alternate:0")
			assertEventBefore(t, outcome.events, "alternate:0", "release")
			if tt.expectedCloseCount == 1 {
				assertEventBefore(t, outcome.events, "release", "close")
			}
		})
	}

}

func TestStopAndCloseRejectPostGateControlWhileAdmittedControlIsHeld(t *testing.T) {
	tests := []struct {
		name               string
		teardown           func(*Device) error
		lifecycle          deviceLifecycle
		postGateError      string
		afterTeardownError string
		expectedCloseCount int
		expectedControlTwo int
	}{
		{
			name:               "stop",
			teardown:           (*Device).StopStreaming,
			lifecycle:          deviceLifecycleStopping,
			postGateError:      "device is stopping",
			expectedCloseCount: 0,
			expectedControlTwo: 1,
		},
		{
			name:               "close",
			teardown:           (*Device).Close,
			lifecycle:          deviceLifecycleClosing,
			postGateError:      "device is closing",
			afterTeardownError: "device is closed",
			expectedCloseCount: 1,
			expectedControlTwo: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := runPostGateControlRejectionScenario(t.Context(), tt.teardown, tt.lifecycle)

			require.NoError(t, outcome.phaseErr)
			require.NoError(t, outcome.cleanupErr)
			require.Empty(t, outcome.unfinishedWorkers)
			require.True(t, outcome.postGateReturnedBeforeRelease)
			require.False(t, outcome.teardownReturnedBeforeRelease)
			require.EqualError(t, outcome.postGateErr, tt.postGateError)
			require.Equal(t, []string{"control-1-start"}, outcome.eventsBeforeRelease)
			require.ErrorIs(t, outcome.firstControlErr, errStreamingStopped)
			require.NoError(t, outcome.teardownErr)
			if tt.afterTeardownError == "" {
				require.NoError(t, outcome.afterTeardownErr)
			} else {
				require.EqualError(t, outcome.afterTeardownErr, tt.afterTeardownError)
			}
			require.Equal(t, 1, countEvent(outcome.eventsAfterCleanup, "control-1-start"))
			require.Equal(t, 1, countEvent(outcome.eventsAfterCleanup, "control-1-return"))
			require.Equal(t, tt.expectedControlTwo, countEvent(outcome.eventsAfterCleanup, "control-2-start"))
			require.Equal(t, tt.expectedControlTwo, countEvent(outcome.eventsAfterCleanup, "control-2-return"))
			require.Equal(t, 1, countEvent(outcome.eventsAfterCleanup, "alternate:0"))
			require.Equal(t, 1, countEvent(outcome.eventsAfterCleanup, "release"))
			require.Equal(t, tt.expectedCloseCount, countEvent(outcome.eventsAfterCleanup, "close"))
			assertEventBefore(t, outcome.eventsAfterCleanup, "control-1-return", "alternate:0")
			assertEventBefore(t, outcome.eventsAfterCleanup, "alternate:0", "release")
			if tt.expectedCloseCount == 1 {
				assertEventBefore(t, outcome.eventsAfterCleanup, "release", "close")
			}
		})
	}
}

func TestCloseCancelsQueuedAdmittedControlBeforeNativeIO(t *testing.T) {
	transport := newQueuedControlTransport()
	dev := NewDeviceWithTransport(transport, ConfigP3)

	firstResult := make(chan error, 1)
	go func() { firstResult <- dev.SendCommandNoResponse(CmdShutter) }()
	<-transport.firstControlStarted

	secondResult := make(chan error, 1)
	go func() { secondResult <- dev.SendCommandNoResponse(CmdShutter) }()
	require.Eventually(t, func() bool {
		dev.stateMu.Lock()
		defer dev.stateMu.Unlock()
		return len(dev.controlCancels) == 2
	}, time.Second, time.Millisecond)

	closeResult := make(chan error, 1)
	go func() { closeResult <- dev.Close() }()
	require.NoError(t, waitForDeviceLifecycle(t.Context(), dev, deviceLifecycleClosing))
	close(transport.releaseFirstControl)

	assert.ErrorIs(t, <-firstResult, errStreamingStopped)
	assert.ErrorIs(t, <-secondResult, errStreamingStopped)
	assert.NoError(t, <-closeResult)
	assert.Equal(t, 1, transport.sendCommandCalls())
	assert.Equal(t, 1, transport.closeCalls())
	assert.EqualError(t, dev.SendCommandNoResponse(CmdShutter), "device is closed")
}

func TestCloseCancelsActiveStartBeforeItsNextNativeCall(t *testing.T) {
	transport := newWithheldStartTransport()
	dev := NewDeviceWithTransport(transport, ConfigP3)

	startResult := make(chan error, 1)
	go func() { startResult <- dev.StartStreaming(context.Background()) }()
	<-transport.firstControlStarted

	closeResult := make(chan error, 1)
	go func() { closeResult <- dev.Close() }()
	enteredClosing := assert.Eventually(t, func() bool {
		dev.stateMu.Lock()
		defer dev.stateMu.Unlock()
		return dev.lifecycle == deviceLifecycleClosing
	}, 250*time.Millisecond, time.Millisecond)

	close(transport.releaseFirstControl)
	startErr := <-startResult
	closeErr := <-closeResult

	assert.True(t, enteredClosing)
	assert.ErrorIs(t, startErr, errStreamingStopped)
	assert.NoError(t, closeErr)
	assert.Equal(t, []string{
		"control-1-start",
		"control-1-return",
		"close",
	}, transport.eventSnapshot())
}

func TestReadAdmissionRaceBeforeStopAndCloseTeardown(t *testing.T) {
	tests := []struct {
		name               string
		teardown           func(*Device) error
		expectedCloseCount int
	}{
		{
			name:               "stop",
			teardown:           (*Device).StopStreaming,
			expectedCloseCount: 0,
		},
		{
			name:               "close",
			teardown:           (*Device).Close,
			expectedCloseCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deadlineCtx, cancelDeadline := context.WithTimeout(
				t.Context(),
				readAdmissionContestTimeout,
			)
			defer cancelDeadline()

			attempt := 0
			runAttempt := func(readerPreferred bool) bool {
				currentAttempt := attempt
				attempt++

				transport := newOrderedDrainTransport()
				dev := NewDeviceWithTransport(transport, ConfigP3)
				dev.streamingPhase = StreamingInterfaceActive
				transport.streamingPhase = StreamingInterfaceActive

				readResult := make(chan error, 1)
				readDone := make(chan struct{})
				teardownResult := make(chan error, 1)
				teardownDone := make(chan struct{})
				readApproached := make(chan struct{})
				teardownApproached := make(chan struct{})
				readParentCtx, cancelReadParent := context.WithCancel(deadlineCtx)
				defer cancelReadParent()
				readCtx := &doneObservedContext{
					Context:  readParentCtx,
					observed: readApproached,
				}

				readerLaunched := false
				teardownLaunched := false
				launchReader := func() {
					readerLaunched = true
					go func() {
						defer close(readDone)
						_, err := dev.ReadFrame(readCtx)
						transport.record("read-finish")
						readResult <- err
					}()
				}
				launchTeardown := func() {
					teardownLaunched = true
					go func() {
						defer close(teardownDone)
						close(teardownApproached)
						teardownResult <- tt.teardown(dev)
					}()
				}

				var phaseErr error
				func() {
					dev.stateMu.Lock()
					defer dev.stateMu.Unlock()

					if readerPreferred {
						launchReader()
						if !waitForLifecycleTestSignal(deadlineCtx, readApproached) {
							phaseErr = fmt.Errorf(
								"reader did not approach admission: attempt %d, row %s: %w",
								currentAttempt,
								tt.name,
								context.Cause(deadlineCtx),
							)
							return
						}
						runtime.Gosched()
						launchTeardown()
						if !waitForLifecycleTestSignal(deadlineCtx, teardownApproached) {
							phaseErr = fmt.Errorf(
								"teardown did not approach admission: attempt %d, row %s: %w",
								currentAttempt,
								tt.name,
								context.Cause(deadlineCtx),
							)
							return
						}
					} else {
						launchTeardown()
						if !waitForLifecycleTestSignal(deadlineCtx, teardownApproached) {
							phaseErr = fmt.Errorf(
								"teardown did not approach admission: attempt %d, row %s: %w",
								currentAttempt,
								tt.name,
								context.Cause(deadlineCtx),
							)
							return
						}
						runtime.Gosched()
						launchReader()
						if !waitForLifecycleTestSignal(deadlineCtx, readApproached) {
							phaseErr = fmt.Errorf(
								"reader did not approach admission: attempt %d, row %s: %w",
								currentAttempt,
								tt.name,
								context.Cause(deadlineCtx),
							)
							return
						}
					}
					runtime.Gosched()
				}()

				var readErr error
				readResultReceived := false
				if phaseErr == nil && readerLaunched {
					readErr, readResultReceived = waitForLifecycleTestResult(deadlineCtx, readResult)
					if !readResultReceived {
						phaseErr = fmt.Errorf(
							"reader did not return: attempt %d, row %s: %w",
							currentAttempt,
							tt.name,
							context.Cause(deadlineCtx),
						)
					}
				}
				var teardownErr error
				teardownResultReceived := false
				if phaseErr == nil && teardownLaunched {
					teardownErr, teardownResultReceived = waitForLifecycleTestResult(deadlineCtx, teardownResult)
					if !teardownResultReceived {
						phaseErr = fmt.Errorf(
							"teardown did not return: attempt %d, row %s: %w",
							currentAttempt,
							tt.name,
							context.Cause(deadlineCtx),
						)
					}
				}

				cancelReadParent()
				cleanupCtx, cancelCleanup := context.WithTimeout(
					t.Context(),
					controlLifecycleCleanupTimeout,
				)
				var unfinishedWorkers []string
				if readerLaunched {
					if !readResultReceived {
						readErr, readResultReceived = waitForLifecycleTestResult(cleanupCtx, readResult)
					}
					readDoneReceived := waitForLifecycleTestSignal(cleanupCtx, readDone)
					if !readResultReceived || !readDoneReceived {
						unfinishedWorkers = append(unfinishedWorkers, "read")
					}
				}
				if teardownLaunched {
					if !teardownResultReceived {
						teardownErr, teardownResultReceived = waitForLifecycleTestResult(cleanupCtx, teardownResult)
					}
					teardownDoneReceived := waitForLifecycleTestSignal(cleanupCtx, teardownDone)
					if !teardownResultReceived || !teardownDoneReceived {
						unfinishedWorkers = append(unfinishedWorkers, "teardown")
					}
				}
				var cleanupErr error
				if len(unfinishedWorkers) != 0 {
					cleanupErr = fmt.Errorf(
						"cleanup deadline exceeded: %w; unfinished workers: %v",
						context.Cause(cleanupCtx),
						unfinishedWorkers,
					)
				}
				cancelCleanup()
				require.NoError(t, cleanupErr, "attempt %d, row %s", currentAttempt, tt.name)
				require.Empty(t, unfinishedWorkers, "attempt %d, row %s", currentAttempt, tt.name)
				require.NoError(t, phaseErr, "attempt %d, row %s", currentAttempt, tt.name)

				events := transport.eventSnapshot()
				require.NoError(
					t,
					teardownErr,
					"attempt %d, row %s, events %v",
					currentAttempt,
					tt.name,
					events,
				)
				require.Equal(t, 1, countEvent(events, "alternate:0"))
				require.Equal(t, 1, countEvent(events, "release"))
				assertEventBefore(t, events, "alternate:0", "release")
				require.Equal(t, tt.expectedCloseCount, countEvent(events, "close"))
				if tt.expectedCloseCount == 1 {
					assertEventBefore(t, events, "release", "close")
				}
				dev.stateMu.Lock()
				readCancelCount := len(dev.readCancels)
				dev.stateMu.Unlock()
				require.Zero(t, readCancelCount)

				readReturnCount := countEvent(events, "read-return")
				readFinishCount := countEvent(events, "read-finish")
				switch {
				case readReturnCount == 1 && readFinishCount == 1:
					require.ErrorIs(
						t,
						readErr,
						errStreamingStopped,
						"attempt %d, row %s, events %v",
						currentAttempt,
						tt.name,
						events,
					)
					readReturnIndex := -1
					readFinishIndex := -1
					alternateIndex := -1
					for index, event := range events {
						switch event {
						case "read-return":
							readReturnIndex = index
						case "read-finish":
							readFinishIndex = index
						case "alternate:0":
							alternateIndex = index
						}
					}
					require.Less(
						t,
						readReturnIndex,
						readFinishIndex,
						"read returned after worker finished: attempt %d, row %s, events %v",
						currentAttempt,
						tt.name,
						events,
					)
					require.Less(
						t,
						readReturnIndex,
						alternateIndex,
						"alternate 0 occurred before admitted read returned: attempt %d, row %s, events %v",
						currentAttempt,
						tt.name,
						events,
					)
					return true
				case readReturnCount == 0 && readFinishCount == 1:
					require.Error(
						t,
						readErr,
						"attempt %d, row %s, events %v",
						currentAttempt,
						tt.name,
						events,
					)
					require.NotErrorIs(
						t,
						readErr,
						errStreamingStopped,
						"attempt %d, row %s, events %v",
						currentAttempt,
						tt.name,
						events,
					)
					return false
				default:
					require.Failf(
						t,
						"unclassified reader outcome",
						"attempt %d, row %s, read error %v, read-return count %d, read-finish count %d, events %v",
						currentAttempt,
						tt.name,
						readErr,
						readReturnCount,
						readFinishCount,
						events,
					)
					return false
				}
			}

			readerWins := 0
			readerRejections := 0
			for range readAdmissionContestAttempts {
				if runAttempt(true) {
					readerWins++
				} else {
					readerRejections++
				}
			}
			for fallbackAttempt := 0; readerRejections == 0 && fallbackAttempt < readAdmissionContestAttempts; fallbackAttempt++ {
				if runAttempt(false) {
					readerWins++
				} else {
					readerRejections++
				}
			}

			require.Positive(t, readerWins, "row %s", tt.name)
			require.Positive(t, readerRejections, "row %s", tt.name)
		})
	}
}

func TestStopStreamingIsIdempotentAfterSuccessfulRestoration(t *testing.T) {
	dev, transport := newMockDevice()
	setMockStreamingActive(dev, transport)

	require.NoError(t, dev.StopStreaming())
	require.NoError(t, dev.StopStreaming())

	assert.False(t, dev.IsStreaming())
	assert.Equal(t, []string{
		"set-streaming-interface-idle",
		"release-streaming-interface",
	}, transport.operationSnapshot())
}

func TestStopStreamingRetriesOnlyPendingRelease(t *testing.T) {
	dev, transport := newMockDevice()
	setMockStreamingActive(dev, transport)
	releaseErr := errors.New("streaming interface release failed")
	transport.releaseError = releaseErr

	firstErr := dev.StopStreaming()
	secondErr := dev.StopStreaming()

	assert.ErrorIs(t, firstErr, releaseErr)
	assert.NoError(t, secondErr)
	assert.False(t, dev.IsStreaming())
	assert.Equal(t, []string{
		"set-streaming-interface-idle",
		"release-streaming-interface",
		"release-streaming-interface",
	}, transport.operationSnapshot())
}

func TestCloseRetriesPendingReleaseBeforeClosing(t *testing.T) {
	dev, transport := newMockDevice()
	setMockStreamingActive(dev, transport)
	releaseErr := errors.New("streaming interface release failed")
	transport.releaseError = releaseErr

	firstErr := dev.Close()
	secondErr := dev.Close()

	assert.ErrorIs(t, firstErr, releaseErr)
	assert.NoError(t, secondErr)
	assert.False(t, dev.IsStreaming())
	assert.Equal(t, []string{
		"set-streaming-interface-idle",
		"release-streaming-interface",
		"release-streaming-interface",
		"close",
	}, transport.operationSnapshot())
}

func TestCloseConsumesTerminalStreamingFailuresAndFinalizesTransport(t *testing.T) {
	tests := []struct {
		name       string
		idleError  error
		releaseErr error
		terminal   error
	}{
		{
			name: "disconnect while selecting idle alternate",
			idleError: &LibUSBError{
				Operation: "setting streaming interface idle",
				Code:      LibUSBErrorNoDevice,
			},
			releaseErr: &LibUSBError{
				Operation: "releasing streaming interface",
				Code:      LibUSBErrorNoDevice,
			},
			terminal: ErrUSBNoDevice,
		},
		{
			name: "disconnect while releasing streaming interface",
			releaseErr: &LibUSBError{
				Operation: "releasing streaming interface",
				Code:      LibUSBErrorNoDevice,
			},
			terminal: ErrUSBNoDevice,
		},
		{
			name: "already released streaming interface",
			releaseErr: &LibUSBError{
				Operation: "releasing streaming interface",
				Code:      LibUSBErrorNotFound,
			},
			terminal: ErrUSBNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, transport := newMockDevice()
			setMockStreamingActive(dev, transport)
			transport.idleError = tt.idleError
			transport.releaseError = tt.releaseErr

			firstErr := dev.Close()
			secondErr := dev.Close()

			assert.ErrorIs(t, firstErr, tt.terminal)
			var typedErr *LibUSBError
			assert.ErrorAs(t, firstErr, &typedErr)
			assert.NoError(t, secondErr)
			assert.False(t, dev.IsStreaming())
			assert.Equal(t, 1, transport.closeCalls)
			assertOperationSuffix(t, transport.operationSnapshot(), []string{
				"set-streaming-interface-idle",
				"release-streaming-interface",
				"close",
			})
		})
	}
}

func TestStopConsumesTerminalStreamingFailureWithoutClosingBaseTransport(t *testing.T) {
	dev, transport := newMockDevice()
	setMockStreamingActive(dev, transport)
	transport.releaseError = &LibUSBError{
		Operation: "releasing streaming interface",
		Code:      LibUSBErrorNotFound,
	}

	firstErr := dev.StopStreaming()
	secondErr := dev.StopStreaming()

	assert.ErrorIs(t, firstErr, ErrUSBNotFound)
	assert.NoError(t, secondErr)
	assert.False(t, dev.IsStreaming())
	assert.Zero(t, transport.closeCalls)
	assert.Equal(t, []string{
		"set-streaming-interface-idle",
		"release-streaming-interface",
	}, transport.operationSnapshot())
}

func TestCloseIsIdempotentAfterTransportCloseSucceeds(t *testing.T) {
	dev, transport := newMockDevice()
	closeErr := errors.New("transport close failed")
	transport.closeError = closeErr

	firstErr := dev.Close()
	secondErr := dev.Close()
	thirdErr := dev.Close()

	assert.ErrorIs(t, firstErr, closeErr)
	assert.NoError(t, secondErr)
	assert.NoError(t, thirdErr)
	assert.Equal(t, 2, transport.closeCalls)
	assert.Equal(t, []string{"close", "close"}, transport.operationSnapshot())
}

func TestCloseConsumesTerminalBaseTransportError(t *testing.T) {
	for _, terminalErr := range []error{
		&LibUSBError{Operation: "releasing control interface", Code: LibUSBErrorNoDevice},
		&LibUSBError{Operation: "releasing control interface", Code: LibUSBErrorNotFound},
	} {
		dev, transport := newMockDevice()
		transport.closeError = terminalErr

		firstErr := dev.Close()
		secondErr := dev.Close()

		assert.ErrorIs(t, firstErr, terminalErr)
		assert.NoError(t, secondErr)
		assert.Equal(t, 1, transport.closeCalls)
		assert.EqualError(t, dev.SendCommandNoResponse(CmdShutter), "device is closed")
	}
}

func TestStartStreamingChecksInitialCommandStatusAndResponseErrors(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*mockTransport, error)
	}{
		{
			name: "status after command",
			setup: func(transport *mockTransport, injectedErr error) {
				transport.addResponseErr(bmRequestTypeIn, bRequestReadStatus, injectedErr)
			},
		},
		{
			name: "command response",
			setup: func(transport *mockTransport, injectedErr error) {
				transport.addStatusResponse(0x02)
				transport.addResponseErr(bmRequestTypeIn, bRequestReadResp, injectedErr)
			},
		},
		{
			name: "status after response",
			setup: func(transport *mockTransport, injectedErr error) {
				transport.addStatusResponse(0x02)
				transport.addReadResponse([]byte{0x01})
				transport.addResponseErr(bmRequestTypeIn, bRequestReadStatus, injectedErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, transport := newMockDevice()
			injectedErr := errors.New("injected start handshake failure")
			tt.setup(transport, injectedErr)
			err := dev.StartStreaming(context.Background())

			assert.ErrorIs(t, err, injectedErr)
			assert.False(t, dev.IsStreaming())
			assert.Empty(t, transport.currentAlt)
		})
	}
}

func TestStartStreamingStatusPollingReturnsCallerCancellation(t *testing.T) {
	dev, transport := newMockDevice()
	for range statusPollLimit {
		transport.addStatusResponse(1)
	}
	cause := errors.New("cancel startup status polling")
	ctx, cancel := context.WithCancelCause(context.Background())
	transport.afterControl = func(call controlCall) {
		if call.RequestType == bmRequestTypeIn && call.Request == bRequestReadStatus {
			cancel(cause)
		}
	}

	startedAt := time.Now()
	err := dev.StartStreaming(ctx)
	elapsed := time.Since(startedAt)

	assert.ErrorIs(t, err, cause)
	assert.Less(t, elapsed, 250*time.Millisecond)
	assert.Equal(t, 1, transport.countCalls(bmRequestTypeIn, bRequestReadStatus))
	assert.NotContains(t, transport.operationSnapshot(), "activate-streaming-interface")
}

func TestStartStreamingRejectsShortHandshakeResponse(t *testing.T) {
	dev, transport := newMockDevice()
	transport.addStatusResponse(0x02)
	transport.addReadResponse(nil)
	transport.addStatusResponse(0x03)
	err := dev.StartStreaming(context.Background())

	assert.ErrorContains(t, err, "initial start_stream response: got 0 bytes, want 1")
	assert.False(t, dev.IsStreaming())
}

func TestStartStreamingRejectsShortStatusResponse(t *testing.T) {
	dev, transport := newMockDevice()
	transport.addResponse(bmRequestTypeIn, bRequestReadStatus, nil)
	transport.addReadResponse([]byte{0x01})
	transport.addStatusResponse(0x03)
	err := dev.StartStreaming(context.Background())

	assert.ErrorContains(t, err, "initial start_stream status after command: got 0 bytes, want 1")
	assert.False(t, dev.IsStreaming())
}

func TestStartStreamingRestoresAfterStartControlFailure(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	startErr := errors.New("0xEE start control failed")
	transport.addControlError(bmRequestTypeDevOut, bRequestStartStream, startErr)

	err := dev.StartStreaming(context.Background())

	assert.ErrorIs(t, err, startErr)
	assert.False(t, dev.IsStreaming())
	assert.Equal(t, streamingAltIdle, transport.currentAlt[streamingIntf])
	assert.Contains(t, transport.operationSnapshot(), "activate-streaming-interface")
	assert.Contains(t, transport.operationSnapshot(), "set-streaming-interface-idle")
	assert.Contains(t, transport.operationSnapshot(), "release-streaming-interface")
}

func TestStartStreamingRejectsNonzeroStartTriggerOUTCount(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	transport.addControlCount(bmRequestTypeDevOut, bRequestStartStream, 1)

	err := dev.StartStreaming(context.Background())

	assert.ErrorContains(t, err, "start stream (0xEE): got 1 bytes, want 0")
	assert.False(t, dev.IsStreaming())
	assertOperationSuffix(t, transport.operationSnapshot(), []string{
		"control:40:ee",
		"set-streaming-interface-idle",
		"release-streaming-interface",
	})
}

func TestStartStreamingRejectsShortInitialCommandOUT(t *testing.T) {
	dev, transport := newMockDevice()
	transport.addControlCount(bmRequestTypeOut, bRequestSendCmd, CommandSize-1)

	err := dev.StartStreaming(context.Background())

	assert.ErrorContains(t, err, "initial start_stream command: got 17 bytes, want 18")
	assert.False(t, dev.IsStreaming())
	assert.NotContains(t, transport.operationSnapshot(), "activate-streaming-interface")
}

func TestStartStreamingRejectsShortFinalCommandOUTAndRestores(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	transport.addControlCount(bmRequestTypeOut, bRequestSendCmd, CommandSize)
	transport.addControlCount(bmRequestTypeOut, bRequestSendCmd, CommandSize-1)

	err := dev.StartStreaming(context.Background())

	assert.ErrorContains(t, err, "final start_stream command: got 17 bytes, want 18")
	assert.False(t, dev.IsStreaming())
	assertOperationSuffix(t, transport.operationSnapshot(), []string{
		"control:41:20",
		"set-streaming-interface-idle",
		"release-streaming-interface",
	})
}

func TestStartStreamingCancellationBeforeActivationDoesNotClaimInterface(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := dev.StartStreaming(ctx)

	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, dev.IsStreaming())
	assert.NotContains(t, transport.operationSnapshot(), "activate-streaming-interface")
	assert.NotContains(t, transport.operationSnapshot(), "set-streaming-interface-idle")
	assert.NotContains(t, transport.operationSnapshot(), "release-streaming-interface")
}

func TestStartStreamingCancellationDuringPreInterfaceDelayRestoresOpenLifecycle(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	cause := errors.New("cancel during pre-interface delay")
	baseCtx, cancel := context.WithCancelCause(context.Background())
	delayEntered := make(chan struct{})
	ctx := &doneObservedContext{
		Context:  baseCtx,
		observed: delayEntered,
	}

	startResult := make(chan error, 1)
	go func() {
		startResult <- dev.StartStreaming(ctx)
	}()
	select {
	case <-delayEntered:
	case <-time.After(time.Second):
		t.Fatal("StartStreaming did not enter the pre-interface delay")
	}
	cancel(cause)
	err := <-startResult

	require.ErrorIs(t, err, cause)
	dev.stateMu.Lock()
	assert.Equal(t, deviceLifecycleOpen, dev.lifecycle)
	assert.Equal(t, StreamingInterfaceIdle, dev.streamingPhase)
	dev.stateMu.Unlock()
	assert.NotContains(t, transport.operationSnapshot(), "activate-streaming-interface")
	assert.NotContains(t, transport.operationSnapshot(), "set-streaming-interface-idle")
	assert.NotContains(t, transport.operationSnapshot(), "release-streaming-interface")

	setupSuccessfulInitialStartHandshake(transport)
	transport.addStatusResponse(0x02)
	transport.addReadResponse([]byte{0x01})
	transport.addStatusResponse(0x03)
	require.NoError(t, dev.StartStreaming(context.Background()))
	require.NoError(t, dev.StopStreaming())
}

type doneObservedContext struct {
	context.Context
	once     sync.Once
	observed chan struct{}
}

func (c *doneObservedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

func TestStartStreamingRestoresWhenContextEndsAfterActivation(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	transport.afterControl = func(call controlCall) {
		if call.RequestType == bmRequestTypeDevOut && call.Request == bRequestStartStream {
			cancel()
		}
	}

	err := dev.StartStreaming(ctx)

	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, dev.IsStreaming())
	assert.Equal(t, streamingAltIdle, transport.currentAlt[streamingIntf])
	assert.Contains(t, transport.operationSnapshot(), "set-streaming-interface-idle")
	assert.Contains(t, transport.operationSnapshot(), "release-streaming-interface")
}

func TestStartStreamingRestoresAfterLateFailures(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*mockTransport, error)
	}{
		{
			name: "final command",
			setup: func(transport *mockTransport, injectedErr error) {
				transport.addControlError(bmRequestTypeOut, bRequestSendCmd, nil)
				transport.addControlError(bmRequestTypeOut, bRequestSendCmd, injectedErr)
			},
		},
		{
			name: "final status after command",
			setup: func(transport *mockTransport, injectedErr error) {
				transport.addResponseErr(bmRequestTypeIn, bRequestReadStatus, injectedErr)
			},
		},
		{
			name: "final response",
			setup: func(transport *mockTransport, injectedErr error) {
				transport.addStatusResponse(0x02)
				transport.addResponseErr(bmRequestTypeIn, bRequestReadResp, injectedErr)
			},
		},
		{
			name: "final status after response",
			setup: func(transport *mockTransport, injectedErr error) {
				transport.addStatusResponse(0x02)
				transport.addReadResponse([]byte{0x01})
				transport.addResponseErr(bmRequestTypeIn, bRequestReadStatus, injectedErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dev, transport := newMockDevice()
			setupSuccessfulInitialStartHandshake(transport)
			injectedErr := errors.New("injected late start failure")
			tt.setup(transport, injectedErr)

			err := dev.StartStreaming(context.Background())

			assert.ErrorIs(t, err, injectedErr)
			assert.False(t, dev.IsStreaming())
			assert.Equal(t, streamingAltIdle, transport.currentAlt[streamingIntf])
			assert.Contains(t, transport.operationSnapshot(), "set-streaming-interface-idle")
			assert.Contains(t, transport.operationSnapshot(), "release-streaming-interface")
		})
	}
}

func TestStartStreamingContinuesAfterPrimingReadError(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	transport.nextBulkError = errors.New("priming read failed")
	transport.addStatusResponse(0x02)
	transport.addReadResponse([]byte{0x01})
	transport.addStatusResponse(0x03)

	err := dev.StartStreaming(context.Background())

	require.NoError(t, err)
	assert.True(t, dev.IsStreaming())
	assertOperationSuffix(t, transport.operationSnapshot(), []string{
		"activate-streaming-interface",
		"control:40:ee",
		"bulk-read:81",
		"control:41:20",
		"control:c1:22",
		"control:c1:21",
		"control:c1:22",
	})
	assert.NotContains(t, transport.operationSnapshot(), "set-streaming-interface-idle")
	assert.NotContains(t, transport.operationSnapshot(), "release-streaming-interface")
}

func TestStartStreamingBoundsBestEffortPrimingRead(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	transport.addStatusResponse(0x02)
	transport.addReadResponse([]byte{0x01})
	transport.addStatusResponse(0x03)
	bounded := false
	transport.bulkRead = func(ctx context.Context, _ uint8, _ []byte) (int, error) {
		deadline, ok := ctx.Deadline()
		bounded = ok && time.Until(deadline) <= 200*time.Millisecond
		return 0, errors.New("best-effort priming error")
	}

	err := dev.StartStreaming(context.Background())

	assert.NoError(t, err)
	assert.True(t, bounded)
}

func TestStartStreamingPropagatesParentCancellationFromPrimingRead(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	cause := errors.New("startup canceled")
	ctx, cancel := context.WithCancelCause(context.Background())
	transport.bulkRead = func(_ context.Context, _ uint8, _ []byte) (int, error) {
		cancel(cause)
		return 0, errors.New("ignored priming error")
	}

	err := dev.StartStreaming(ctx)

	assert.ErrorIs(t, err, cause)
	assert.False(t, dev.IsStreaming())
	assertOperationSuffix(t, transport.operationSnapshot(), []string{
		"bulk-read:81",
		"set-streaming-interface-idle",
		"release-streaming-interface",
	})
}

func TestStartStreamingRestoresWhenCanceledByFinalStatusTransfer(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	transport.addStatusResponse(0x02)
	transport.addReadResponse([]byte{0x01})
	transport.addStatusResponse(0x03)
	cause := errors.New("canceled by final status")
	ctx, cancel := context.WithCancelCause(context.Background())
	statusReads := 0
	transport.afterControl = func(call controlCall) {
		if call.RequestType == bmRequestTypeIn && call.Request == bRequestReadStatus {
			statusReads++
			if statusReads == 4 {
				cancel(cause)
			}
		}
	}

	err := dev.StartStreaming(ctx)

	assert.ErrorIs(t, err, cause)
	assert.False(t, dev.IsStreaming())
	assertOperationSuffix(t, transport.operationSnapshot(), []string{
		"control:c1:22",
		"set-streaming-interface-idle",
		"release-streaming-interface",
	})
}

func TestStartStreamingRestoresPhaseReturnedWithActivationError(t *testing.T) {
	tests := []struct {
		name       string
		phase      StreamingInterfacePhase
		operations []string
	}{
		{
			name:       "claim or endpoint acquisition failed while idle",
			phase:      StreamingInterfaceIdle,
			operations: []string{"activate-streaming-interface"},
		},
		{
			name:  "endpoint acquisition failed after activation",
			phase: StreamingInterfaceActive,
			operations: []string{
				"activate-streaming-interface",
				"set-streaming-interface-idle",
				"release-streaming-interface",
			},
		},
		{
			name:  "active alternate failed after claim",
			phase: StreamingInterfaceRestorePending,
			operations: []string{
				"activate-streaming-interface",
				"set-streaming-interface-idle",
				"release-streaming-interface",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dev, transport := newMockDevice()
			setupSuccessfulInitialStartHandshake(transport)
			activateErr := errors.New("streaming activation failed")
			transport.activatePhase = tt.phase
			transport.activateError = activateErr

			err := dev.StartStreaming(context.Background())

			assert.ErrorIs(t, err, activateErr)
			assert.False(t, dev.IsStreaming())
			assertOperationSuffix(t, transport.operationSnapshot(), tt.operations)
		})
	}
}

func TestStartStreamingRestoresFromRestorePendingAfterAlternateSettingFailure(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	activateErr := errors.New("setting active alternate failed")
	idleErr := errors.New("setting idle alternate failed")
	transport.activatePhase = StreamingInterfaceRestorePending
	transport.activateError = activateErr
	transport.idleError = idleErr

	startErr := dev.StartStreaming(context.Background())
	stopErr := dev.StopStreaming()

	assert.ErrorIs(t, startErr, activateErr)
	assert.ErrorIs(t, startErr, idleErr)
	assert.NoError(t, stopErr)
	assertOperationSuffix(t, transport.operationSnapshot(), []string{
		"activate-streaming-interface",
		"set-streaming-interface-idle",
		"set-streaming-interface-idle",
		"release-streaming-interface",
	})
}

func TestStartStreamingJoinsActivationAndRestorationErrors(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	activateErr := errors.New("endpoint acquisition failed")
	restoreErr := errors.New("idle transition failed")
	transport.activatePhase = StreamingInterfaceActive
	transport.activateError = activateErr
	transport.idleError = restoreErr

	err := dev.StartStreaming(context.Background())

	assert.ErrorIs(t, err, activateErr)
	assert.ErrorIs(t, err, restoreErr)
	assert.True(t, dev.IsStreaming())
	assertOperationSuffix(t, transport.operationSnapshot(), []string{
		"activate-streaming-interface",
		"set-streaming-interface-idle",
	})
}

func TestCloseRetriesRestorationAfterStartCleanupFailure(t *testing.T) {
	dev, transport := newMockDevice()
	setupSuccessfulInitialStartHandshake(transport)
	activateErr := errors.New("endpoint acquisition failed")
	restoreErr := errors.New("idle transition failed")
	transport.activatePhase = StreamingInterfaceActive
	transport.activateError = activateErr
	transport.idleError = restoreErr

	startErr := dev.StartStreaming(context.Background())
	closeErr := dev.Close()

	assert.ErrorIs(t, startErr, activateErr)
	assert.ErrorIs(t, startErr, restoreErr)
	assert.NoError(t, closeErr)
	assert.False(t, dev.IsStreaming())
	assertOperationSuffix(t, transport.operationSnapshot(), []string{
		"activate-streaming-interface",
		"set-streaming-interface-idle",
		"set-streaming-interface-idle",
		"release-streaming-interface",
		"close",
	})
}

func TestStartStreamingRejectsNonActiveSuccessfulActivation(t *testing.T) {
	tests := []struct {
		name       string
		phase      StreamingInterfacePhase
		phaseName  string
		operations []string
	}{
		{
			name:       "idle",
			phase:      StreamingInterfaceIdle,
			phaseName:  "idle",
			operations: []string{"activate-streaming-interface"},
		},
		{
			name:      "release pending",
			phase:     StreamingInterfaceReleasePending,
			phaseName: "release pending",
			operations: []string{
				"activate-streaming-interface",
				"release-streaming-interface",
			},
		},
		{
			name:       "unknown",
			phase:      StreamingInterfacePhase(99),
			phaseName:  "unknown",
			operations: []string{"activate-streaming-interface"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dev, transport := newMockDevice()
			setupSuccessfulInitialStartHandshake(transport)
			transport.activatePhase = tt.phase

			err := dev.StartStreaming(context.Background())

			assert.ErrorContains(t, err, "returned "+tt.phaseName+" phase without an error")
			assert.False(t, dev.IsStreaming())
			assertOperationSuffix(t, transport.operationSnapshot(), tt.operations)
		})
	}
}

func TestStartStreamingDoesNotMutateReleasePendingState(t *testing.T) {
	dev, transport := newMockDevice()
	dev.streamingPhase = StreamingInterfaceReleasePending
	dev.stats = FrameStats{FramesRead: 42}
	transport.streamingPhase = StreamingInterfaceReleasePending

	err := dev.StartStreaming(context.Background())

	assert.ErrorContains(t, err, "interface release is pending")
	assert.False(t, dev.IsStreaming())
	assert.Equal(t, FrameStats{FramesRead: 42}, dev.Stats())
	assert.Empty(t, transport.operationSnapshot())
}

func TestStartStreamingRejectsAlreadyActiveAndUnknownPhases(t *testing.T) {
	tests := []struct {
		name          string
		phase         StreamingInterfacePhase
		errorContains string
		isStreaming   bool
	}{
		{
			name:          "active",
			phase:         StreamingInterfaceActive,
			errorContains: "already streaming",
			isStreaming:   true,
		},
		{
			name:          "unknown",
			phase:         StreamingInterfacePhase(99),
			errorContains: "unknown interface phase 99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, transport := newMockDevice()
			dev.streamingPhase = tt.phase
			dev.stats = FrameStats{FramesRead: 42}

			err := dev.StartStreaming(context.Background())

			assert.ErrorContains(t, err, tt.errorContains)
			assert.Equal(t, tt.isStreaming, dev.IsStreaming())
			assert.Equal(t, FrameStats{FramesRead: 42}, dev.Stats())
			assert.Empty(t, transport.operationSnapshot())
		})
	}
}

func TestStartStreamingRejectsClosedDevice(t *testing.T) {
	dev, transport := newMockDevice()
	require.NoError(t, dev.Close())
	operationsBeforeStart := transport.operationSnapshot()
	dev.stats = FrameStats{FramesRead: 42}

	err := dev.StartStreaming(context.Background())

	assert.ErrorContains(t, err, "device is closed")
	assert.False(t, dev.IsStreaming())
	assert.Equal(t, FrameStats{FramesRead: 42}, dev.Stats())
	assert.Equal(t, operationsBeforeStart, transport.operationSnapshot())
}

func TestStopStreamingRejectsUnrestorablePhases(t *testing.T) {
	t.Run("unsupported transport", func(t *testing.T) {
		transport := &controlOnlyMockTransport{}
		dev := NewDeviceWithTransport(transport, ConfigP3)
		dev.streamingPhase = StreamingInterfaceActive

		err := dev.StopStreaming()

		assert.ErrorIs(t, err, ErrStreamingUnsupported)
		assert.True(t, dev.IsStreaming())
		assert.Zero(t, transport.controlCalls)
		assert.Zero(t, transport.closeCalls)
	})

	t.Run("unknown phase", func(t *testing.T) {
		dev, transport := newMockDevice()
		dev.streamingPhase = StreamingInterfacePhase(99)

		err := dev.StopStreaming()

		assert.ErrorContains(t, err, "unknown streaming interface phase 99")
		assert.False(t, dev.IsStreaming())
		assert.Empty(t, transport.operationSnapshot())
	})
}

func TestStreamingInterfacePhaseString(t *testing.T) {
	tests := []struct {
		phase StreamingInterfacePhase
		name  string
	}{
		{phase: StreamingInterfaceIdle, name: "idle"},
		{phase: StreamingInterfaceActive, name: "active"},
		{phase: StreamingInterfaceReleasePending, name: "release pending"},
		{phase: StreamingInterfaceRestorePending, name: "restore pending"},
		{phase: StreamingInterfacePhase(99), name: "unknown"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.name, tt.phase.String())
	}
}

func TestStartStreamingRejectsUnsupportedTransportBeforeMutation(t *testing.T) {
	transport := &controlOnlyMockTransport{}
	dev := NewDeviceWithTransport(transport, ConfigP3)
	dev.stats = FrameStats{FramesRead: 42}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := dev.StartStreaming(ctx)

	assert.ErrorIs(t, err, ErrStreamingUnsupported)
	assert.NotErrorIs(t, err, context.Canceled)
	assert.False(t, dev.IsStreaming())
	assert.Equal(t, FrameStats{FramesRead: 42}, dev.Stats())
	assert.Zero(t, transport.controlCalls)
	assert.Zero(t, transport.closeCalls)
}

type controlOnlyMockTransport struct {
	controlCalls int
	closeCalls   int
}

func (m *controlOnlyMockTransport) Control(
	requestType uint8,
	request uint8,
	value uint16,
	index uint16,
	data []byte,
) (int, error) {
	m.controlCalls++
	return 0, nil
}

func (m *controlOnlyMockTransport) Close() error {
	m.closeCalls++
	return nil
}

func assertOperationSuffix(
	t *testing.T,
	actual []string,
	suffix []string,
) {
	t.Helper()
	if !assert.GreaterOrEqual(t, len(actual), len(suffix)) {
		return
	}
	assert.Equal(t, suffix, actual[len(actual)-len(suffix):])
}

func setupSuccessfulInitialStartHandshake(transport *mockTransport) {
	transport.addStatusResponse(0x02)
	transport.addReadResponse([]byte{0x01})
	transport.addStatusResponse(0x03)
	transport.bulkData = append(transport.bulkData, make([]byte, 100))
}

type orderedDrainTransport struct {
	mu                  sync.Mutex
	events              []string
	controlCount        int
	streamingPhase      StreamingInterfacePhase
	readStarted         chan struct{}
	firstControlStarted chan struct{}
	releaseFirstControl chan struct{}
}

type queuedControlTransport struct {
	mu                  sync.Mutex
	sendCalls           int
	closedCount         int
	firstControlStarted chan struct{}
	releaseFirstControl chan struct{}
}

type withheldStartTransport struct {
	mu                  sync.Mutex
	events              []string
	controlCalls        int
	firstControlStarted chan struct{}
	releaseFirstControl chan struct{}
}

func newWithheldStartTransport() *withheldStartTransport {
	return &withheldStartTransport{
		firstControlStarted: make(chan struct{}),
		releaseFirstControl: make(chan struct{}),
	}
}

func (t *withheldStartTransport) Control(
	_ uint8,
	_ uint8,
	_ uint16,
	_ uint16,
	data []byte,
) (int, error) {
	t.mu.Lock()
	t.controlCalls++
	call := t.controlCalls
	t.events = append(t.events, fmt.Sprintf("control-%d-start", call))
	t.mu.Unlock()
	if call == 1 {
		close(t.firstControlStarted)
		<-t.releaseFirstControl
		t.record("control-1-return")
		return len(data), nil
	}
	return 0, fmt.Errorf("unexpected native control %d after Close", call)
}

func (t *withheldStartTransport) BulkRead(
	context.Context,
	uint8,
	[]byte,
) (int, error) {
	return 0, errors.New("unexpected bulk read")
}

func (t *withheldStartTransport) ActivateStreamingInterface() (StreamingInterfacePhase, error) {
	return StreamingInterfaceIdle, errors.New("unexpected streaming activation")
}

func (t *withheldStartTransport) SetStreamingInterfaceIdle() error {
	return errors.New("unexpected streaming restoration")
}

func (t *withheldStartTransport) ReleaseStreamingInterface() error {
	return errors.New("unexpected streaming release")
}

func (t *withheldStartTransport) Close() error {
	t.record("close")
	return nil
}

func (t *withheldStartTransport) record(event string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}

func (t *withheldStartTransport) eventSnapshot() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.events...)
}

func newQueuedControlTransport() *queuedControlTransport {
	return &queuedControlTransport{
		firstControlStarted: make(chan struct{}),
		releaseFirstControl: make(chan struct{}),
	}
}

func (t *queuedControlTransport) Control(
	requestType, request uint8,
	_ uint16,
	_ uint16,
	data []byte,
) (int, error) {
	if requestType == bmRequestTypeOut && request == bRequestSendCmd {
		t.mu.Lock()
		t.sendCalls++
		call := t.sendCalls
		t.mu.Unlock()
		if call == 1 {
			close(t.firstControlStarted)
			<-t.releaseFirstControl
		}
		return len(data), nil
	}
	if requestType == bmRequestTypeIn && request == bRequestReadStatus {
		data[0] = 2
		return 1, nil
	}
	return 0, fmt.Errorf("unexpected control %02x:%02x", requestType, request)
}

func (t *queuedControlTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closedCount++
	return nil
}

func (t *queuedControlTransport) sendCommandCalls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sendCalls
}

func (t *queuedControlTransport) closeCalls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closedCount
}

func newOrderedDrainTransport() *orderedDrainTransport {
	return &orderedDrainTransport{
		readStarted:         make(chan struct{}),
		firstControlStarted: make(chan struct{}),
		releaseFirstControl: make(chan struct{}),
	}
}

func (t *orderedDrainTransport) Control(
	requestType, request uint8,
	_ uint16,
	_ uint16,
	data []byte,
) (int, error) {
	if requestType == bmRequestTypeOut && request == bRequestSendCmd {
		t.mu.Lock()
		t.controlCount++
		controlNumber := t.controlCount
		t.events = append(t.events, fmt.Sprintf("control-%d-start", controlNumber))
		t.mu.Unlock()
		if controlNumber == 1 {
			close(t.firstControlStarted)
			<-t.releaseFirstControl
		}
		t.record(fmt.Sprintf("control-%d-return", controlNumber))
		return len(data), nil
	}
	if requestType == bmRequestTypeIn && request == bRequestReadStatus {
		data[0] = 2
		return 1, nil
	}
	return 0, fmt.Errorf("unexpected control %02x:%02x", requestType, request)
}

func (t *orderedDrainTransport) BulkRead(
	ctx context.Context,
	_ uint8,
	_ []byte,
) (int, error) {
	close(t.readStarted)
	<-ctx.Done()
	t.record("read-return")
	return 0, context.Cause(ctx)
}

func (t *orderedDrainTransport) ActivateStreamingInterface() (StreamingInterfacePhase, error) {
	return t.streamingPhase, errors.New("unexpected activation")
}

func (t *orderedDrainTransport) SetStreamingInterfaceIdle() error {
	t.record("alternate:0")
	t.streamingPhase = StreamingInterfaceReleasePending
	return nil
}

func (t *orderedDrainTransport) ReleaseStreamingInterface() error {
	t.record("release")
	t.streamingPhase = StreamingInterfaceIdle
	return nil
}

func (t *orderedDrainTransport) Close() error {
	t.record("close")
	return nil
}

func (t *orderedDrainTransport) record(event string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}

func (t *orderedDrainTransport) eventSnapshot() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.events...)
}

func assertEventBefore(t *testing.T, events []string, first, second string) {
	t.Helper()
	firstIndex, secondIndex := -1, -1
	for index, event := range events {
		switch event {
		case first:
			firstIndex = index
		case second:
			secondIndex = index
		}
	}
	assert.NotEqual(t, -1, firstIndex, "%q absent from %v", first, events)
	assert.NotEqual(t, -1, secondIndex, "%q absent from %v", second, events)
	assert.Less(t, firstIndex, secondIndex, "%v", events)
}

func countEvent(events []string, target string) int {
	count := 0
	for _, event := range events {
		if event == target {
			count++
		}
	}
	return count
}
