package thermalmaster

import (
	"fmt"
	"sync"
)

// mockTransport records USB operations and returns canned responses.
type mockTransport struct {
	mu sync.Mutex

	// Record of all control transfers sent.
	controlCalls []controlCall

	// Canned responses for control reads.
	// Key: fmt.Sprintf("%02x:%02x", requestType, request)
	responses   map[string][]mockResponse
	responseIdx map[string]int

	// Bulk read data.
	bulkData [][]byte
	bulkIdx  int

	// Interface state.
	currentAlt map[int]int

	// Error injection.
	nextControlError error
	nextBulkError    error

	closed bool
}

type controlCall struct {
	RequestType uint8
	Request     uint8
	Val         uint16
	Idx         uint16
	Data        []byte
}

type mockResponse struct {
	Data []byte
	Err  error
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		responses:   make(map[string][]mockResponse),
		responseIdx: make(map[string]int),
		currentAlt:  make(map[int]int),
	}
}

// addResponse adds a canned response for a specific request type/request pair.
func (m *mockTransport) addResponse(requestType, request uint8, data []byte) {
	key := fmt.Sprintf("%02x:%02x", requestType, request)
	m.responses[key] = append(m.responses[key], mockResponse{Data: data})
}

// addResponseErr adds a canned error response for a specific request type/request pair.
func (m *mockTransport) addResponseErr(requestType, request uint8, err error) {
	key := fmt.Sprintf("%02x:%02x", requestType, request)
	m.responses[key] = append(m.responses[key], mockResponse{Err: err})
}

// addStatusResponse adds a standard status byte response (for readStatus calls).
func (m *mockTransport) addStatusResponse(status byte) {
	m.addResponse(0xC1, 0x22, []byte{status})
}

// addReadResponse adds a response for readResponse calls.
func (m *mockTransport) addReadResponse(data []byte) {
	m.addResponse(0xC1, 0x21, data)
}

// setupStandardGetResponse sets up the typical sequence for a "get" command:
// sendCommand -> readStatus(0x02) -> readResponse(data) -> readStatus(0x03)
func (m *mockTransport) setupStandardGetResponse(responseData []byte) {
	m.addStatusResponse(0x02) // status after command
	m.addReadResponse(responseData)
	m.addStatusResponse(0x03) // status after read
}

// setupStandardSetResponse sets up the typical sequence for a "set" command
// that does NOT have read-back verification:
// sendCommand -> readStatus(0x02)
func (m *mockTransport) setupStandardSetResponse() {
	m.addStatusResponse(0x02)
}

func (m *mockTransport) Control(
	requestType, request uint8,
	val, idx uint16,
	data []byte,
) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.nextControlError != nil {
		err := m.nextControlError
		m.nextControlError = nil
		return 0, err
	}

	// Record the call.
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	m.controlCalls = append(m.controlCalls, controlCall{
		RequestType: requestType,
		Request:     request,
		Val:         val,
		Idx:         idx,
		Data:        dataCopy,
	})

	// For OUT transfers (writes), just return success.
	if requestType&0x80 == 0 {
		return len(data), nil
	}

	// For IN transfers (reads), return canned response.
	key := fmt.Sprintf("%02x:%02x", requestType, request)
	resps, ok := m.responses[key]
	if !ok || len(resps) == 0 {
		return 0, fmt.Errorf("no mock response for %s", key)
	}

	idx2 := m.responseIdx[key]
	if idx2 >= len(resps) {
		return 0, fmt.Errorf("mock responses exhausted for %s (used %d)", key, idx2)
	}

	resp := resps[idx2]
	m.responseIdx[key] = idx2 + 1

	if resp.Err != nil {
		return 0, resp.Err
	}

	n := copy(data, resp.Data)
	return n, nil
}

func (m *mockTransport) BulkRead(endpoint uint8, buf []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.nextBulkError != nil {
		err := m.nextBulkError
		m.nextBulkError = nil
		return 0, err
	}

	if m.bulkIdx >= len(m.bulkData) {
		return 0, fmt.Errorf("no more bulk data")
	}

	data := m.bulkData[m.bulkIdx]
	m.bulkIdx++
	n := copy(buf, data)
	return n, nil
}

func (m *mockTransport) SetInterfaceAlt(intf, alt int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentAlt[intf] = alt
	return nil
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// lastCommand returns the last command sent (the 18-byte command data from the
// OUT control transfer to bRequestSendCmd).
func (m *mockTransport) lastCommand() [CommandSize]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	var cmd [CommandSize]byte
	for i := len(m.controlCalls) - 1; i >= 0; i-- {
		if m.controlCalls[i].RequestType == bmRequestTypeOut && m.controlCalls[i].Request == bRequestSendCmd {
			copy(cmd[:], m.controlCalls[i].Data)
			return cmd
		}
	}
	return cmd
}

// allCommands returns all commands sent (OUT control transfers to bRequestSendCmd).
func (m *mockTransport) allCommands() [][CommandSize]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	var cmds [][CommandSize]byte
	for _, c := range m.controlCalls {
		if c.RequestType == bmRequestTypeOut && c.Request == bRequestSendCmd {
			var cmd [CommandSize]byte
			copy(cmd[:], c.Data)
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

// countCalls counts number of specific control transfer types.
func (m *mockTransport) countCalls(requestType, request uint8) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, c := range m.controlCalls {
		if c.RequestType == requestType && c.Request == request {
			count++
		}
	}
	return count
}

// newMockDevice creates a mock device for testing.
func newMockDevice() (*Device, *mockTransport) {
	mock := newMockTransport()
	dev := NewDeviceWithTransport(mock, ConfigP3)
	return dev, mock
}
