package modbus_test

import (
	"math/rand/v2"
	"net"
	"testing"
	"time"

	"github.com/pascaldekloe/modbus"
)

func testTCPClient(t *testing.T) *modbus.TCPClient {
	conn, err := net.Dial("tcp", "localhost:5020")
	if err != nil {
		t.Fatal("no connection to the test server:", err)
	}

	t.Cleanup(func() {
		err := conn.Close()
		if err != nil {
			t.Error("connection shutdown:", err)
		}
	})

	conn.SetDeadline(time.Now().Add(time.Second))

	return &modbus.TCPClient{Conn: conn}
}

func TestTCPRegSingle(t *testing.T) {
	client := testTCPClient(t)

	const addr = 42
	value := uint16(rand.Uint())

	// set register
	err := client.WriteReg(addr, value)
	if err != nil {
		t.Fatal(err)
	}

	// get register
	got, err := client.ReadHoldReg(addr)
	if err != nil {
		t.Fatal(err)
	}
	if got != value {
		t.Errorf("got value %d, want %d", got, value)
	}
}

func TestTCPRegBatch(t *testing.T) {
	client := testTCPClient(t)

	const startAddr = 1001
	values := [3]uint16{
		uint16(rand.Uint()),
		uint16(rand.Uint()),
		uint16(rand.Uint()),
	}

	// set registers
	err := client.WriteRegs(startAddr, values[0], values[1], values[2])
	if err != nil {
		t.Fatal(err)
	}

	// get registers
	var got [len(values)]uint16
	err = client.ReadHoldRegs(got[:], startAddr)
	if err != nil {
		t.Fatal(err)
	}
	if got != values {
		t.Errorf("got values %d, want %d", got, values)
	}
}
