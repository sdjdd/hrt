package main

import (
	"bufio"
	"fmt"
)

var (
	HErrAgentNotOnline = HTTPError{503, "Agent Offline", "agent not online"}
	HErrNoRouteRecort  = HTTPError{404, "Not Found", "no such route record"}
)

const httpErrorTmpl = `hrt error: %s`

type HTTPError struct {
	Status  int
	Message string
	Content string
}

func (e HTTPError) Error() string {
	return e.Content
}

func (e HTTPError) Write(w *bufio.Writer) {
	msg := fmt.Sprintf(httpErrorTmpl, e.Content)
	fmt.Fprintf(w, "HTTP/1.1 %d %s\r\n", e.Status, e.Message)
	fmt.Fprintf(w, "Content-Length: %d\r\n", len(msg))
	w.WriteString("\r\n")
	w.WriteString(msg)
	w.Flush()
}
