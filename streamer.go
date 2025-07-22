package main

import (
	"bytes"
	"context"
	"errors"
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
	streamBuffer int

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
		// TODO: update to 96k after opus impl
		streamBuffer: 16,
		logf:         log.Printf,
		streams:      make(map[*stream]struct{}),
		playLimiter:  rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
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
	stm.streams[s] = struct{}{}
	stm.streamsMu.Unlock()
}

func (stm *streamer) closeStream(s *stream) {
	stm.streamsMu.Lock()
	delete(stm.streams, s)
	stm.streamsMu.Unlock()
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.Write(ctx, websocket.MessageBinary, msg)
}

func (stm *streamer) startStreaming(s *stream) error {
	stm.streamsMu.Lock()
	defer stm.streamsMu.Unlock()

	stm.playLimiter.Wait(context.Background())

	opusBuffer, err := stm.getOpusBuffer()
	packetSize := 64

	go func() {
		defer close(s.buffer) // Cerrar el canal al terminar
		for i := 0; i < len(opusBuffer); i += packetSize {
			end := i + packetSize
			if end > len(opusBuffer) {
				end = len(opusBuffer) // Ajustar el final para el último fragmento
			}
			// Enviar el fragmento al canal
			s.buffer <- opusBuffer[i:end]
		}
	}()

	if err != nil {
		return err
	}
	return nil
}

func (stm *streamer) getOpusBuffer() ([]byte, error) {
	// Crear un buffer para capturar la salida
	var buf bytes.Buffer

	// Configurar el flujo de FFmpeg
	err := ffmpeg_go.Input("input.wav").
		Output("pipe:", ffmpeg_go.KwArgs{"c:a": "libopus", "f": "opus"}).
		WithOutput(&buf). // Redirigir la salida al buffer
		Run()
	if err != nil {
		stm.logf("Error al convertir el archivo: %v\n", err)
		return nil, err
	}
	return buf.Bytes(), nil
}
