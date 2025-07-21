package main

import (
	"log"
	"net/http"
	"sync"
)

type streamer struct {
	//Current file playing
	current int
	//function for logs, default log.Printf
	logf func(f string, v ...interface{})

	//router for the endpoints
	serveMux http.ServeMux

	// TODO: verify if necessary
	streamerMu sync.Mutex
}

func newStreamer() *streamer {
	stm := &streamer{
		current: 0,
		logf:    log.Printf,
	}
	stm.serveMux.Handle("/", http.FileServer(http.Dir(".")))
	stm.serveMux.HandleFunc("/next", stm.nextHandler)

	return stm
}

func (stm *streamer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stm.serveMux.ServeHTTP(w, r)
}

func (stm *streamer) nextHandler(w http.ResponseWriter, r *http.Request) {

}
