package main

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

type Route map[string]RouteRecord

type RouteRecord struct {
	AgentID string
	Host    string
}

func ReadJsonRoute(filename string) (r Route, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	rawRoute := map[string]string{}
	err = json.NewDecoder(f).Decode(&rawRoute)
	if err != nil {
		return
	}

	r = make(Route)
	for host, record := range rawRoute {
		i := strings.Index(record, ":")
		if i < 0 {
			err = errors.New("invalid route format")
			return
		}
		r[host] = RouteRecord{
			AgentID: record[:i],
			Host:    record[i+1:],
		}
	}
	return
}
