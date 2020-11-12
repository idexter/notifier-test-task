// Package main provides simple HTTP server created for testing purposes.
//
// By default it runs on "localhost:8080" and just write log message for each request body.
package main

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"golang.org/x/net/netutil"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	portFlag           string
	maxConnectionsFlag int
)

func main() {
	kingpin.Flag("port", "port").Default(":8080").StringVar(&portFlag)
	kingpin.Flag("max-connections", "Maximum server connections\n").Default("10").IntVar(&maxConnectionsFlag)
	kingpin.Parse()

	l, err := net.Listen("tcp", portFlag)
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	defer l.Close() //nolint: errcheck

	l = netutil.LimitListener(l, maxConnectionsFlag)

	if err := http.Serve(l, http.HandlerFunc(defaultHandler)); err != nil {
		log.Fatalf("Unable to run test server, reason: %v", err)
	}
}

func defaultHandler(resp http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Got message: %s\n", body)
	resp.WriteHeader(http.StatusOK)
}
