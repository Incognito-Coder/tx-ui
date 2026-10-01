package network

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"sync"
)

type AutoHttpsConn struct {
	net.Conn

	reader     *bufio.Reader
	redirected bool

	readRequestOnce sync.Once
}

func NewAutoHttpsConn(conn net.Conn) net.Conn {
	return &AutoHttpsConn{
		Conn: conn,
	}
}

func (c *AutoHttpsConn) readRequest() bool {
	c.reader = bufio.NewReader(c.Conn)
	firstByte, err := c.reader.Peek(1)
	if err != nil || firstByte[0] == 0x16 { // TLS handshake record
		return false
	}
	request, err := http.ReadRequest(c.reader)
	if err != nil {
		c.Close()
		c.redirected = true
		return true
	}
	resp := http.Response{
		Header: http.Header{},
	}
	resp.StatusCode = http.StatusTemporaryRedirect
	location := fmt.Sprintf("https://%v%v", request.Host, request.RequestURI)
	resp.Header.Set("Location", location)
	_ = resp.Write(c.Conn)
	c.Close()
	c.redirected = true
	return true
}

func (c *AutoHttpsConn) Read(buf []byte) (int, error) {
	c.readRequestOnce.Do(func() {
		c.readRequest()
	})

	if c.redirected {
		return 0, net.ErrClosed
	}
	if c.reader != nil {
		return c.reader.Read(buf)
	}

	return c.Conn.Read(buf)
}
