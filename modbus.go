// Package modbus implements the Modbus protocol from a client perspective.
//
// Registers are 16-bit values with a 16-bit address each. Input registers are
// read-only, while holding registers allow for updates.
//
// Modbus is big-endian per specification. Deviation with little-endian
// can be translated with bits.ReverseBytes16.
package modbus

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Standardised Error Codes
const (
	ErrFunc   Exception = 1 // illegal function
	ErrAddr   Exception = 2 // illegal data address
	ErrValue  Exception = 3 // illegal data value
	ErrDev    Exception = 4 // server device failure
	ErrAck    Exception = 5 // acknowledge
	ErrBusy   Exception = 6 // server device bussy
	ErrParity Exception = 8 // memory parity error

	ErrGatePath   Exception = 0xA // gateway path unavailable
	ErrGateTarget Exception = 0xB // gateway target device failed to respond
)

// Exception is an error response.
type Exception byte

// Error implements the builtin.error interface.
func (e Exception) Error() string {
	switch e {
	case ErrFunc:
		return "Modbus exception 0x01: illegal function"
	case ErrAddr:
		return "Modbus exception 0x02: illegal data address"
	case ErrValue:
		return "Modbus exception 0x03: illegal data value"
	case ErrDev:
		return "Modbus exception 0x04: server device failure"
	case ErrAck:
		return "Modbus exception 0x05: acknowldege"
	case ErrBusy:
		return "Modbus exception 0x06: server device busy"
	case ErrParity:
		return "Modbus exception 0x08: memory parity error"
	case ErrGatePath:
		return "Modbus exception 0x0A: gateway path unavailable"
	case ErrGateTarget:
		return "Modbus exception 0x0B: gateway target device failed to respond"
	}
	return fmt.Sprintf("Modbus exception 0x%#02X", e)
}

// ErrLimit denies a request based on the amount of values requested.
var ErrLimit = errors.New("Modbus value count exceeds protocol limit")

// Response Errors
var (
	errFrameFit    = errors.New("Modbus payload does not match frame size")
	errAddrMatch   = errors.New("Modbus address in response does not match the requested")
	errValueMatch  = errors.New("Modbus value in response does not match the requested")
	errWriteNMatch = errors.New("Modbus write count does not match the requested")
)

// Function Codes
const (
	readCoils  = 0x01
	writeCoil  = 0x05
	writeCoils = 0x0f

	readInputRegs = 0x04
	readHoldRegs  = 0x03
	writeReg      = 0x06
	writeRegs     = 0x10
	readWriteRegs = 0x17
	maskWriteReg  = 0x16
	readFIFO      = 0x18

	readFile  = 0x14
	writeFile = 0x15

	errorFlag = 0x80
)

// RegPairFloat extracts a single-precission floating-point from two registers.
func RegPairFloat(p *[4]byte) float32 {
	bits := binary.BigEndian.Uint32(p[:4])
	return math.Float32frombits(bits)
}

// PutRegPairFloat places a single-precission floating-point over two registers.
func PutRegPairFloat(p *[4]byte, f float32) {
	bits := math.Float32bits(f)
	binary.BigEndian.PutUint32(p[:4], bits)
}

// RegQuadFloat extracts a double-precission floating-point from four registers.
func RegQuadFloat(p *[8]byte) float64 {
	bits := binary.BigEndian.Uint64(p[:8])
	return math.Float64frombits(bits)
}

// PutRegQuadFloat places a double-precission floating-point over four registers.
func PutRegQuadFloat(p *[8]byte, f float64) {
	bits := math.Float64bits(f)
	binary.BigEndian.PutUint64(p[:8], bits)
}
