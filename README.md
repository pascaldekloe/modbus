[![API Documentation](https://godoc.org/github.com/pascaldekloe/modbus?status.svg)](https://godoc.org/github.com/pascaldekloe/modbus)
[![Build Status](https://github.com/pascaldekloe/modbus/actions/workflows/go.yml/badge.svg)](https://github.com/pascaldekloe/modbus/actions/workflows/go.yml)

# Modbus

The library consists of a TCP client for the Go programming language.

This is free and unencumbered software released into the
[public domain](http://creativecommons.org/publicdomain/zero/1.0).


# Use

Connections are handled automagically including reconnects when needed.

```go
	client := modbus.TCPClient{
		RemoteAddr: "bot.example.com:502",
		TxTimeout:  time.Second,
		UnitID:     0x01,
	}
	defer client.Close()
```


### Testing

Run `go test` with a server on localhost as follows.

    docker run -d -p 5020:5020 oitc/modbus-server:latest
