// Copyright (c) 2021 Wireleap

package tun

import "log"

type Writer struct{ queue chan []byte }

func NewWriter(tunif *T) *Writer {
	t := &Writer{queue: make(chan []byte, 1024)}
	go func() {
		for data := range t.queue {
			if _, err := tunif.Write(data); err != nil {
				log.Println("error writing packet data:", err)
				continue
			}
		}
	}()
	return t
}

func (t *Writer) Send(data []byte) { t.queue <- data }
