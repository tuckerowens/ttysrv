package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/tarm/serial"
)

var (
	dev     = flag.String("dev", "/dev/ttyUSB0", "Serial device to listen on")
	baud    = flag.Int("baud", 115200, "Baud rate")
	logFile = flag.String("log", "ttysrv.log", "Log file used to capture serial output")
	port    = flag.Int("port", 666, "Port to run ttysrv server on")
)

type ChannelManager struct {
	master chan []byte
	clones map[chan []byte]struct{}
}

func main() {
	flag.Parse()
	c, err := captureSerial(*dev, *baud)
	if err != nil {
		log.Fatal(err)
	}
	cm := New(c)
	err = logOutput(*logFile, cm.Clone())
	if err != nil {
		log.Fatal(err)
	}
	serveLogs(*port, cm)
}

func captureSerial(dev string, baud int) (chan []byte, error) {
	c := &serial.Config{Name: dev, Baud: baud}

	stream, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}

	d := make([]byte, 64)
	out := make(chan []byte)
	go func() {
		for {
			_, err := stream.Read(d)
			if err != nil {
				close(out)
				return
			}
			out <- d
		}
	}()
	return out, nil
}

func serveLogs(port int, m *ChannelManager) {
	l, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Started listening on port %v", port)
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go serveClient(conn, m)
	}
}

func serveClient(conn net.Conn, m *ChannelManager) {
	c := m.Clone()
	for {
		d := <-c
		_, err := conn.Write(d)
		if err != nil {
			conn.Close()
			m.Close(c)
			return
		}
	}
}

func logOutput(logFile string, c chan []byte) error {
	f, err := os.Create(logFile)
	if err != nil {
		return err
	}
	go func() {
		for {
			d := <-c
			fmt.Printf("%s", string(d))
			_, err := f.Write(d)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	return nil
}

func New(master chan []byte) *ChannelManager {
	cm := &ChannelManager{
		master: master,
		clones: make(map[chan []byte]struct{}),
	}
	go func() {
		for {
			d, ok := <-cm.master
			if !ok {
				return
			}
			for s := range cm.clones {
				s <- d
			}
		}
	}()
	return cm
}

func (c *ChannelManager) Clone() chan []byte {
	out := make(chan []byte)
	c.clones[out] = struct{}{}
	return out
}

func (c *ChannelManager) Close(s chan []byte) {
	if _, ok := c.clones[s]; ok {
		close(s)
		delete(c.clones, s)
	}
}
