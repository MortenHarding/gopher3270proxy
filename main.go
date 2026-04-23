package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
)

var (
	defaultHost = flag.String("host", "gopherspace.dk", "Default gopher host to connect to on startup")
	gopherPort  = flag.Int("port", 70, "Default gopher port")
	listenPort  = flag.Int("listen", 3270, "Port to listen for TN3270 connections")
	logFile     = flag.String("log", "", "Log file path (default: stdout)")
	verbose     = flag.Bool("v", false, "Verbose logging")
	asciiFlag   = flag.Bool("ascii", false, "Send ASCII instead of EBCDIC (for MochaSoft and clients that do their own translation)")
)

// asciiMode is set from the -ascii flag after flag.Parse() and used by screen.go WriteText.
var asciiMode bool

func main() {
	flag.Parse()
	asciiMode = *asciiFlag

	// Set up logging
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Cannot open log file: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	addr := fmt.Sprintf("0.0.0.0:%d", *listenPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}
	defer listener.Close()

	log.Printf("Gopher3270 proxy listening on %s", addr)
	log.Printf("Default gopher host: %s:%d", *defaultHost, *gopherPort)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		log.Printf("New connection from %s", conn.RemoteAddr())
		session := NewSession(conn, *defaultHost, *gopherPort, *verbose)
		go session.Run()
	}
}
