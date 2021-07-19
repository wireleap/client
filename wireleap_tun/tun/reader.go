// Copyright (c) 2021 Wireleap

package tun

import "log"

type Reader struct{ queue chan []byte }

func NewReader(tunif *T) *Reader {
	t := &Reader{queue: make(chan []byte, 1024)}
	buf := make([]byte, 65535) // max IP packet size
	go func() {
		for {
			n, err := tunif.Read(buf)
			if err != nil {
				log.Println("error reading packet data:", err)
				continue
			}
			// raw packet data copied here
			data := make([]byte, n)
			copy(data, buf[:n])
			t.queue <- data
		}
	}()
	return t
}

func (t *Reader) Recv() []byte { return <-t.queue }
