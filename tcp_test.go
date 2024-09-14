package modbus_test

import (
	"io"
	"log"
	"math/rand/v2"
	"os"
	"testing"
	"time"

	"github.com/pascaldekloe/modbus"
)

func init() {
	log.SetOutput(io.Discard)
}

func Example() {
	client, err := modbus.TCPDial("localhost:502", time.Second)
	if err != nil {
		log.Print("no connection: ", err)
		return
	}

	borrow, err := client.ReadNHoldRegSlice(2, 1001)
	if err != nil {
		log.Print("registers unavailable: ", err)
		return
	}

	f := modbus.RegPairFloat((*[4]byte)(borrow))
	log.Printf("register 1001 and 1002 contain %f", f)
	// Output:
}

func testTCPClient(t *testing.T) *modbus.TCPClient {
	addr := os.Getenv("TEST_MODBUS_ADDR")
	if addr == "" {
		addr = "localhost:5020"
	}

	client, err := modbus.TCPDial(addr, time.Second/2)
	if err != nil {
		t.Fatal("no connection to test server:", err)
	}

	t.Cleanup(func() {
		err := client.Close()
		if err != nil {
			t.Error("connection leak:", err)
		}
	})

	// test timeout on connection level
	client.SetDeadline(time.Now().Add(time.Second))
	// disable time management of client
	client.TxTimeout = 0

	return client
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
