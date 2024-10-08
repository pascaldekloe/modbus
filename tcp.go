package modbus

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

// TCPDial connects with a lean setup.
func TCPDial(addr string, timeout time.Duration) (*TCPClient, error) {
	d := net.Dialer{
		Timeout:   timeout,
		KeepAlive: -1, // disabled
	}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	client := TCPClient{
		Conn:      conn,
		TxTimeout: timeout,
		UnitID:    0xff,
	}

	t, ok := conn.(*net.TCPConn)
	if !ok {
		return nil, errors.Join(
			fmt.Errorf("TCP dial got connection type %T", conn),
			conn.Close(),
		)
	}
	err = t.SetReadBuffer(512)
	if err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	err = t.SetWriteBuffer(512)
	if err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	return &client, nil
}

// TCPClient manages a connection for use from witin a single goroutine.
// Transactions are dealt with sequentially—only one request at a time.
//
// Any errors other than Exception or ErrLimit are fatal to the client.
// Reuse with a new net.Conn is permitted.
type TCPClient struct {
	// Buf is (re)used for both reading and writing. The function code
	// starts at the 8th byte, right after its 7-byte MBAP-header.
	// Keep first in struct for memory alignment.
	buf [7 + 253]byte

	// The defaults from net.Dial are good here.
	net.Conn

	// Limit the time for a request–response pair on connection level.
	// The zero value omits timeout protection.
	TxTimeout time.Duration

	// read-only transaction counter
	TxN uint64

	// read-only packet-fragementation counter (should be low if any)
	FragN uint64

	// The unit-identifier is supposed to be 0xFF with TCP.
	// Broadcast address 0x00 “is also accepted”. In practice,
	// quite a few devices out there only respond to 0x01.
	UnitID byte
}

// ReadInputReg fetches an input register at the given address.
func (c *TCPClient) ReadInputReg(addr uint16) (uint16, error) {
	return c.readReg(addr, readInputRegs)
}

// ReadHoldReg fetches a holding register at the given address.
func (c *TCPClient) ReadHoldReg(addr uint16) (uint16, error) {
	return c.readReg(addr, readHoldRegs)
}

func (c *TCPClient) readReg(addr uint16, funcCode byte) (uint16, error) {
	err := c.readNRegs(1, addr, funcCode)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(c.buf[9:11]), nil
}

func (c *TCPClient) readNRegs(n int, startAddr uint16, funcCode byte) error {
	// compose request
	binary.BigEndian.PutUint32(c.buf[8:12], uint32(startAddr)<<16|uint32(n))

	readN, err := c.sendAndReceive(c.buf[:12], funcCode)
	if err != nil {
		return err
	}

	if int(uint(c.buf[8])) != n*2 {
		return fmt.Errorf("Modbus repsonse with %d-byte payload for a %d-register request",
			c.buf[1], n)
	}
	if readN != 9+n*2 {
		return errFrameFit
	}
	return nil
}

// SendAndReceive writes the frame header plus function code in c.buf[:8] before
// submission. The req slice must include c.buf[:8] as such. The read count also
// includes the frame header.
func (c *TCPClient) sendAndReceive(req []byte, funcCode byte) (readN int, err error) {
	c.TxN++

	if c.TxTimeout != 0 {
		err := c.Conn.SetDeadline(time.Now().Add(c.TxTimeout))
		if err != nil {
			return 0, fmt.Errorf("timeout on Modbus connection needed: %w", err)
		}
		defer func() {
			err := c.Conn.SetDeadline(time.Time{})
			if err != nil { // probably never
				log.Println("timeout on Modbus connection got stuck:", err)
			}
		}()
	}

	// See “MBAP Header description” from chapter 3.1.3 of “MODBUS Messaging
	// on TCP/IP Implementation Guide V1.0b” for the specification.
	var reqHead uint64
	// 2-byte transaction identifier taken from LSB of counter:
	reqHead |= c.TxN << 48
	// 2-byte protocol identifier remains zero for Modbus
	// …
	// 2-byte size of what follows:
	reqHead |= uint64(uint64(len(req))-6) << 16
	// 1-byte unit identifier:
	reqHead |= uint64(c.UnitID) << 8
	// 1-byte function code:
	reqHead |= uint64(funcCode)

	binary.BigEndian.PutUint64(c.buf[:8], reqHead)

	_, err = c.Write(req)
	if err != nil {
		return 0, fmt.Errorf("Modbus request submission: %w", err)
	}

	readN, err = io.ReadAtLeast(c.Conn, c.buf[:], 9)
	if err != nil {
		return readN, fmt.Errorf("Modbus response unavailable: %w", err)
	}
	resHead := binary.BigEndian.Uint64(c.buf[:8])

	// The transaction, protocol and unit identifier all must equal the
	// request's. The function code in return may include an error flag.
	const sizeMask = 0xffff << 16
	switch resHead &^ sizeMask {
	case reqHead &^ sizeMask:
		break // regular response

	case (reqHead &^ sizeMask) | errorFlag:
		if readN != 9 {
			return readN, errFrameFit
		}
		return readN, Exception(c.buf[8])

	default:
		return readN, fmt.Errorf("Modbus response frame %#08x… does not match request frame %#08x…",
			resHead, reqHead)
	}

	remainLen := (resHead >> 16) & 0xffff
	end := int(remainLen + 6)
	switch {
	case end == readN:
		break // happy flow
	case end < readN:
		return readN, errors.New("Modbus response reception exceeds frame length")
	case end > len(c.buf):
		return readN, errors.New("Modbus frame size exceeds reponse [PDU] limit")
	default:
		// packet fragmentation should be a rare occurrence
		c.FragN++

		_, err = io.ReadFull(c.Conn, c.buf[readN:end])
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return readN, fmt.Errorf("Modbus response frame incomplete: %w", err)
		}
		readN = end
	}

	return readN, nil
}

// ReadInputRegs fetches consecutive input-registers at a start address into a
// read buffer. The return is ErrLimit when buf is larger than 125 entries.
func (c *TCPClient) ReadInputRegs(buf []uint16, startAddr uint16) error {
	return c.readRegs(buf, startAddr, readInputRegs)
}

// ReadHoldRegs fetches consecutive holding-registers at a start address into a
// read buffer. The return is ErrLimit when buf is larger than 125 entries.
func (c *TCPClient) ReadHoldRegs(buf []uint16, startAddr uint16) error {
	return c.readRegs(buf, startAddr, readHoldRegs)
}

func (c *TCPClient) readRegs(buf []uint16, startAddr uint16, funcCode byte) error {
	if len(buf) == 0 {
		return nil // allowed
	}
	if len(buf) > 125 {
		return ErrLimit
	}

	err := c.readNRegs(len(buf), startAddr, funcCode)
	if err != nil {
		return err
	}

	// map read buffer into register buffer
	for i := range buf {
		buf[i] = binary.BigEndian.Uint16(c.buf[9+i*2 : 11+i*2])
	}
	return nil
}

// ReadNInputRegSlice fetches n consecutive input-registers at a start address.
// The slice in return has 2 bytes in big-endian order per register. Bytes stop
// being valid at the next invocation to the TCPClient. The return is ErrLimit
// when n is over 125.
func (c *TCPClient) ReadNInputRegSlice(n int, startAddr uint16) ([]byte, error) {
	return c.readNRegSlice(n, startAddr, readInputRegs)
}

// ReadNHoldRegSlice fetches n consecutive holding-registers at a start address.
// The slice in return has 2 bytes in big-endian order per register. Bytes stop
// being valid at the next invocation to the TCPClient. The return is ErrLimit
// when n is over 125.
func (c *TCPClient) ReadNHoldRegSlice(n int, startAddr uint16) ([]byte, error) {
	return c.readNRegSlice(n, startAddr, readHoldRegs)
}

func (c *TCPClient) readNRegSlice(n int, startAddr uint16, funcCode byte) ([]byte, error) {
	if n < 1 {
		return nil, nil // allowed
	}
	if n > 125 {
		return nil, ErrLimit
	}
	err := c.readNRegs(n, startAddr, funcCode)
	if err != nil {
		return nil, err
	}
	return c.buf[9 : 9+(n*2)], nil
}

// WriteReg updates a single register.
func (c *TCPClient) WriteReg(addr, value uint16) error {
	order := uint32(addr)<<16 | uint32(value)
	binary.BigEndian.PutUint32(c.buf[8:12], order)
	readN, err := c.sendAndReceive(c.buf[:12], writeReg)
	if err != nil {
		return err
	}

	if readN != 12 {
		return errFrameFit
	}
	did := binary.BigEndian.Uint32(c.buf[8:12])
	if did != order {
		if did>>16 != order>>16 {
			return errAddrMatch
		}
		return errValueMatch
	}
	return nil
}

// WriteRegs updates consecutive registers at a start address.
// The return is ErrLimit when more than 123 values are given.
func (c *TCPClient) WriteRegs(startAddr uint16, values ...uint16) error {
	if len(values) == 0 {
		return nil // allow
	}
	if len(values) > 123 {
		return ErrLimit
	}

	order := uint32(startAddr)<<16 | uint32(len(values))
	binary.BigEndian.PutUint32(c.buf[8:12], order)
	c.buf[12] = byte(len(values) * 2)
	for i := range values {
		binary.BigEndian.PutUint16(c.buf[13+(2*i):15+(2*i)], values[i])
	}
	readN, err := c.sendAndReceive(c.buf[:13+(2*len(values))], writeRegs)
	if err != nil {
		return err
	}

	if readN != 12 {
		return errFrameFit
	}
	did := binary.BigEndian.Uint32(c.buf[8:12])
	if did != order {
		if did>>16 != order>>16 {
			return errAddrMatch
		}
		return errWriteNMatch
	}
	return nil
}
