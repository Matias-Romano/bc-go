package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/u2takey/ffmpeg-go"
	"golang.org/x/time/rate"
)

type streamer struct {
	streamPacketSize int

	//function for logs, default log.Printf
	logf func(f string, v ...interface{})

	playLimiter *rate.Limiter
	//router for the endpoints
	serveMux http.ServeMux

	streamsMu sync.Mutex
	streams   map[*stream]struct{}
}

type stream struct {
	//Current file playing
	current int
	//buffer to send the file
	buffer    chan []byte
	closeSlow func()
}

func newStreamer() *streamer {
	stm := &streamer{
		streamPacketSize: 512,
		logf:             log.Printf,
		streams:          make(map[*stream]struct{}),
		playLimiter:      rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
	}
	stm.serveMux.Handle("/", http.FileServer(http.Dir(".")))
	stm.serveMux.HandleFunc("/play", stm.playHandler)

	return stm
}

func (stm *streamer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stm.serveMux.ServeHTTP(w, r)
}

func (stm *streamer) playHandler(w http.ResponseWriter, r *http.Request) {
	err := stm.stream(w, r)
	if errors.Is(err, context.Canceled) {
		return
	}
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return
	}
	if err != nil {
		stm.logf("%v", err)
		return
	}
}

func (stm *streamer) stream(w http.ResponseWriter, r *http.Request) error {
	var mu sync.Mutex
	var c *websocket.Conn
	var closed bool
	s := &stream{
		current: 0,
		buffer:  make(chan []byte),
		closeSlow: func() {
			mu.Lock()
			defer mu.Unlock()
			closed = true
			if c != nil {
				c.Close(websocket.StatusPolicyViolation, "connection too slow to keep up with messages")
			}
		},
	}
	stm.openStream(s)
	defer stm.closeStream(s)

	c2, err := websocket.Accept(w, r, nil)
	if err != nil {
		return err
	}
	mu.Lock()
	if closed {
		mu.Unlock()
		return net.ErrClosed
	}
	c = c2
	mu.Unlock()
	defer c.CloseNow()

	ctx := c.CloseRead(context.Background())
	stm.startStreaming(s)

	for {
		select {
		case buffer := <-s.buffer:
			err := writeTimeout(ctx, time.Second*5, c, buffer)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (stm *streamer) openStream(s *stream) {
	stm.streamsMu.Lock()
	stm.logf("stream opened")
	stm.streams[s] = struct{}{}
	stm.streamsMu.Unlock()
}

func (stm *streamer) closeStream(s *stream) {
	stm.streamsMu.Lock()
	stm.logf("stream closed")
	delete(stm.streams, s)
	stm.streamsMu.Unlock()
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.Write(ctx, websocket.MessageBinary, msg)
}

func (stm *streamer) startStreaming(s *stream) error {
	var err error
	reader, writer := io.Pipe()

	stm.streamsMu.Lock()
	defer stm.streamsMu.Unlock()

	stm.playLimiter.Wait(context.Background())

	go func() {
		defer close(s.buffer)
		defer reader.Close() // Cerrar el reader al terminar

		buf := make([]byte, stm.streamPacketSize)
		for {
			n, errReader := reader.Read(buf)
			if errReader == io.EOF {
				// Si hay datos pendientes antes del EOF, enviarlos
				if n > 0 {
					s.buffer <- buf[:n]
				}
				break
			}
			if errReader != nil {
				stm.logf("Error when reading pipe: %v\n", errReader)
				err = errReader
				break
			}
			if n > 0 {
				stm.logf("data is being sent!")
				// Enviar el fragmento leído al canal
				s.buffer <- append([]byte{}, buf[:n]...) // Copia para evitar reutilización del buffer
			}
		}
	}()

	go func() {
		defer writer.Close() // Cerrar el writer al terminar
		errWriter := ffmpeg_go.Input("test.wav").
			Output("pipe:", ffmpeg_go.KwArgs{"c:a": "libopus", "f": "opus", "b:a": "96k"}).
			WithOutput(writer).
			Run()
		if errWriter != nil {
			stm.logf("Error in FFmpeg: %v\n", errWriter)
			err = errWriter
		}
	}()

	if err != nil {
		return err
	}
	return nil
}
