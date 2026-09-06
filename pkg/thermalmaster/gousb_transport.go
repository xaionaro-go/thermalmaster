package thermalmaster

import (
	"context"
	"errors"
	"fmt"

	"github.com/xaionaro-go/usb"
)

// goUSBOperations owns the usb hierarchy. Interface ownership stays live
// after retryable release failures, so closing parents cannot bypass cleanup.
type goUSBOperations struct {
	context    *usb.Context
	device     *usb.Device
	config     *usb.Config
	interfaces map[int]*usb.Interface
}

func newGoUSBOperations(
	ctx *usb.Context,
	device *usb.Device,
) *goUSBOperations {
	device.ControlTimeout = usbControlTransferTimeout
	return &goUSBOperations{
		context:    ctx,
		device:     device,
		interfaces: make(map[int]*usb.Interface),
	}
}

func (o *goUSBOperations) control(
	requestType uint8,
	request uint8,
	value uint16,
	index uint16,
	data []byte,
) (int, error) {
	n, err := o.device.Control(requestType, request, value, index, data)
	return n, adaptGoUSBError("USB control transfer", err)
}

func (o *goUSBOperations) bulkRead(
	ctx context.Context,
	address uint8,
	buf []byte,
) (int, error) {
	intf := o.interfaces[streamingIntf]
	if intf == nil {
		return 0, fmt.Errorf("streaming interface is not claimed")
	}
	if address != bulkEndpointAddr {
		return 0, fmt.Errorf("unsupported bulk IN endpoint %02x", address)
	}
	endpoint, err := intf.InEndpoint(int(address & 0x0f))
	if err != nil {
		return 0, adaptGoUSBError("opening bulk IN endpoint", err)
	}
	n, err := endpoint.ReadContext(ctx, buf)
	return n, adaptGoUSBError("USB bulk transfer", err)
}

func (o *goUSBOperations) claimInterface(number int) error {
	intf, err := o.config.ClaimInterface(number)
	if err != nil {
		return adaptGoUSBError("claiming USB interface", err)
	}
	o.interfaces[number] = intf
	return nil
}

func (o *goUSBOperations) setInterfaceAlternate(
	number int,
	alternate int,
) error {
	intf := o.interfaces[number]
	if intf == nil {
		return fmt.Errorf("USB interface %d is not claimed", number)
	}
	return adaptGoUSBError("selecting USB alternate setting", intf.SetAlternate(alternate))
}

func (o *goUSBOperations) releaseInterface(number int) error {
	intf := o.interfaces[number]
	if intf == nil {
		return nil
	}
	err := adaptGoUSBError("releasing USB interface", intf.Release())
	if err == nil || errors.Is(err, ErrUSBNoDevice) || errors.Is(err, ErrUSBNotFound) {
		delete(o.interfaces, number)
	}
	return err
}

// disposeInterface is only for unpublished setup rollback, immediately followed
// by closing the configuration and device. Normal teardown uses Release.
func (o *goUSBOperations) disposeInterface(number int) error {
	intf := o.interfaces[number]
	if intf == nil {
		return nil
	}
	err := intf.CloseWithError()
	delete(o.interfaces, number)
	return adaptGoUSBError("disposing USB interface after setup failure", err)
}

func (o *goUSBOperations) close() error {
	if o.config != nil {
		if err := o.config.Close(); err != nil {
			return adaptGoUSBError("closing USB configuration", err)
		}
		o.config = nil
	}
	if o.device != nil {
		if err := o.device.Close(); err != nil {
			return adaptGoUSBError("closing USB device", err)
		}
		o.device = nil
	}
	if o.context != nil {
		if err := o.context.Close(); err != nil {
			return adaptGoUSBError("closing USB context", err)
		}
		o.context = nil
	}
	return nil
}

type goUSBConfigDetails struct {
	controlAlternateCount int
	streamEndpoint        bool
}

func (o *goUSBOperations) configure() (goUSBConfigDetails, error) {
	cfg, err := o.device.Config(usbConfigNum)
	if err != nil {
		return goUSBConfigDetails{}, adaptGoUSBError("selecting USB configuration", err)
	}
	o.config = cfg
	var details goUSBConfigDetails
	for _, intf := range cfg.Desc.Interfaces {
		for _, alternate := range intf.AltSettings {
			switch {
			case intf.Number == controlIntf && alternate.Alternate == controlAlt:
				details.controlAlternateCount = len(intf.AltSettings)
			case intf.Number == streamingIntf && alternate.Alternate == streamingAltStart:
				endpoint, ok := alternate.Endpoints[usb.EndpointAddress(bulkEndpointAddr)]
				details.streamEndpoint = ok && endpoint.TransferType == usb.TransferTypeBulk
			}
		}
	}
	if details.controlAlternateCount == 0 {
		return goUSBConfigDetails{}, fmt.Errorf("USB configuration %d has no interface %d alternate %d", usbConfigNum, controlIntf, controlAlt)
	}
	return details, nil
}

func (o *goUSBOperations) setAutoDetach() error {
	return adaptGoUSBError("enabling automatic kernel-driver detach", o.device.SetAutoDetach(true))
}

func prepareGoUSBDevice(operations goUSBDevicePreparationOperations) (*goUSBTransport, error) {
	// usb's configuration selection must precede auto-detach. Enabling it
	// first makes Device.Config manually detach every interface's driver.
	details, err := operations.configure()
	if err != nil {
		return nil, err
	}
	if err := operations.setAutoDetach(); err != nil {
		return nil, err
	}
	if err := operations.claimInterface(controlIntf); err != nil {
		return nil, err
	}
	if details.controlAlternateCount > 1 {
		if err := operations.setInterfaceAlternate(controlIntf, controlAlt); err != nil {
			return nil, errors.Join(err, operations.disposeInterface(controlIntf))
		}
	}
	transport := newGoUSBTransport(operations, true)
	transport.streamEndpoint = details.streamEndpoint
	return transport, nil
}

func adaptGoUSBError(
	operation string,
	err error,
) error {
	if err == nil {
		return nil
	}
	var native usb.Error
	if errors.As(err, &native) {
		return &LibUSBError{
			Operation: operation,
			Code:      LibUSBErrorCode(native),
			Name:      native.Error(),
			Cause:     err,
		}
	}
	var status usb.TransferStatus
	if errors.As(err, &status) && status == usb.TransferNoDevice {
		err = errors.Join(err, ErrUSBNoDevice)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

type goUSBTransportOperations interface {
	control(uint8, uint8, uint16, uint16, []byte) (int, error)
	bulkRead(context.Context, uint8, []byte) (int, error)
	claimInterface(int) error
	setInterfaceAlternate(int, int) error
	releaseInterface(int) error
	close() error
}

type goUSBDevicePreparationOperations interface {
	goUSBTransportOperations
	configure() (goUSBConfigDetails, error)
	setAutoDetach() error
	disposeInterface(int) error
}

type goUSBTransport struct {
	operations     goUSBTransportOperations
	controlClaimed bool
	streamingPhase StreamingInterfacePhase
	streamEndpoint bool
	closed         bool
}

func newGoUSBTransport(
	operations goUSBTransportOperations,
	controlClaimed bool,
) *goUSBTransport {
	return &goUSBTransport{
		operations:     operations,
		controlClaimed: controlClaimed,
		streamEndpoint: true,
	}
}

func (t *goUSBTransport) Control(
	requestType uint8,
	request uint8,
	value uint16,
	index uint16,
	data []byte,
) (int, error) {
	if t.closed || t.operations == nil {
		return 0, fmt.Errorf("USB transport is closed")
	}
	return t.operations.control(
		requestType,
		request,
		value,
		index,
		data,
	)
}

func (t *goUSBTransport) BulkRead(
	ctx context.Context,
	endpoint uint8,
	buf []byte,
) (int, error) {
	if cause := context.Cause(ctx); cause != nil {
		return 0, cause
	}
	if t.closed || t.operations == nil {
		return 0, fmt.Errorf("USB transport is closed")
	}
	if len(buf) == 0 {
		return 0, nil
	}
	n, err := t.operations.bulkRead(ctx, endpoint, buf)
	if cause := context.Cause(ctx); cause != nil {
		return n, errors.Join(cause, err)
	}
	if err != nil {
		return n, fmt.Errorf("reading USB bulk endpoint: %w", err)
	}
	return n, nil
}

func (t *goUSBTransport) ActivateStreamingInterface() (StreamingInterfacePhase, error) {
	if t.closed || t.operations == nil {
		return t.streamingPhase, fmt.Errorf("USB transport is closed")
	}
	if t.streamingPhase != StreamingInterfaceIdle {
		return t.streamingPhase, fmt.Errorf(
			"cannot activate streaming interface from %s phase",
			t.streamingPhase,
		)
	}
	if err := t.operations.claimInterface(streamingIntf); err != nil {
		return StreamingInterfaceIdle, err
	}
	t.streamingPhase = StreamingInterfaceRestorePending

	if err := t.operations.setInterfaceAlternate(streamingIntf, streamingAltStart); err != nil {
		return t.streamingPhase, err
	}
	t.streamingPhase = StreamingInterfaceActive
	if !t.streamEndpoint {
		return t.streamingPhase, fmt.Errorf("streaming interface has no bulk IN endpoint %02x", bulkEndpointAddr)
	}
	return t.streamingPhase, nil
}

func (t *goUSBTransport) SetStreamingInterfaceIdle() error {
	switch t.streamingPhase {
	case StreamingInterfaceIdle, StreamingInterfaceReleasePending:
		return nil
	case StreamingInterfaceRestorePending, StreamingInterfaceActive:
	default:
		return fmt.Errorf("unknown streaming interface phase %d", t.streamingPhase)
	}

	err := t.operations.setInterfaceAlternate(streamingIntf, streamingAltIdle)
	switch {
	case err == nil:
		t.streamingPhase = StreamingInterfaceReleasePending
	case errors.Is(err, ErrUSBNoDevice):
		t.streamingPhase = StreamingInterfaceReleasePending
	default:
	}
	return err
}

func (t *goUSBTransport) ReleaseStreamingInterface() error {
	switch t.streamingPhase {
	case StreamingInterfaceIdle:
		return nil
	case StreamingInterfaceActive:
		return fmt.Errorf("streaming interface must be idle before release")
	case StreamingInterfaceRestorePending:
		return fmt.Errorf("streaming interface must be restored to idle before release")
	case StreamingInterfaceReleasePending:
	default:
		return fmt.Errorf("unknown streaming interface phase %d", t.streamingPhase)
	}

	err := t.operations.releaseInterface(streamingIntf)
	switch {
	case err == nil:
		t.streamingPhase = StreamingInterfaceIdle
	case errors.Is(err, ErrUSBNoDevice), errors.Is(err, ErrUSBNotFound):
		t.streamingPhase = StreamingInterfaceIdle
	default:
	}
	return err
}

func (t *goUSBTransport) Close() error {
	if t.streamingPhase != StreamingInterfaceIdle {
		return fmt.Errorf(
			"cannot close USB transport with streaming interface in %s phase",
			t.streamingPhase,
		)
	}
	if t.closed {
		return nil
	}

	var closeErrors []error
	if t.controlClaimed {
		err := t.operations.releaseInterface(controlIntf)
		switch {
		case err == nil:
			t.controlClaimed = false
		case errors.Is(err, ErrUSBNoDevice), errors.Is(err, ErrUSBNotFound):
			t.controlClaimed = false
			closeErrors = append(closeErrors, err)
		default:
			return err
		}
	}

	if err := t.operations.close(); err != nil {
		return errors.Join(append(closeErrors, err)...)
	}
	t.operations = nil
	t.closed = true
	return errors.Join(closeErrors...)
}

var _ USBStreamingTransport = (*goUSBTransport)(nil)
