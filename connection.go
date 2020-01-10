package main

import (
	"net"
	"time"
)

type Connection struct {
	conn net.Conn
	mr   *MessageReader
}

func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		conn: conn,
		mr:   NewMessageReader(conn),
	}
}

func (c *Connection) Close() error {
	return c.conn.Close()
}

func (c *Connection) ReadMessage(timeout time.Duration) (Transferable, error) {
	if timeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(timeout))
		defer c.conn.SetReadDeadline(time.Time{})
	}
	return c.mr.Read()
}

func (c *Connection) SendMessage(msg Message) error {
	_, err := c.conn.Write(msg.Bytes())
	return err
}

func (c Connection) String() string {
	return c.conn.RemoteAddr().String()
}

func (c *Connection) SendOK() error {
	_, err := c.conn.Write([]byte("type:OK\n\n"))
	return err
}
