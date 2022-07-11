package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
)

func h12server(stopper *sync.WaitGroup,
	listen string,
	certFile string,
	keyFile string,
	handler http.Handler) *http.Server {
	srv := &http.Server{
		Addr:    listen,
		Handler: handler,
	}
	stopper.Add(1)
	go func() {
		defer stopper.Done()
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != http.ErrServerClosed {
			log.Fatalf("ListenAndServeTLS(): %v", err)
		}
	}()

	// returning reference so caller can call Shutdown()
	return srv
}

func h3server(stopper *sync.WaitGroup, listen string, certFile string, keyFile string, handler http.Handler) *http3.Server {
	srv := &http3.Server{Addr: listen, Handler: handler}
	stopper.Add(1)
	go func() {
		defer stopper.Done() // let main know we are done cleaning up
		// return http3.ListenAndServeQUIC(listen, certFile, keyFile, nil)
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != quic.ErrServerClosed {
			log.Fatalf("ListenAndServeTLS(): %v", err)
		}
	}()
	return srv
}

type Response struct {
	RemoteAddr string
	Protocol   string
	Url        string
	Header     http.Header
}

type reflectorHandler struct {
}

func (_x reflectorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	out, _ := json.MarshalIndent(Response{
		RemoteAddr: r.RemoteAddr,
		Protocol:   r.Proto,
		Url:        r.URL.String(),
		Header:     r.Header,
	}, "", "  ")
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(out)
}

func Start(wg *sync.WaitGroup, host string, cert string, key string, handler http.Handler) func() {
	fmt.Printf("Starting:%s\n", host)
	// go func() {
	h12 := h12server(wg, host, cert, key, handler)
	// }()
	fmt.Printf("Listening H12 on %s\n", host)
	h3 := h3server(wg, host, cert, key, handler)
	/*
		go func() {
			fmt.Printf("Waiting H12\n")
			time.Sleep(time.Second * 5)
			fmt.Printf("Shutting down H12\n")
			h12.Shutdown(context.TODO())
		}()
	*/
	return func() {
		h12.Shutdown(context.TODO())
		h3.Close()
		// http.HandleFunc("/", nil)
	}
}

func main() {
	host := "localhost:3000"
	cert := "./dev.cert"
	key := "./dev.key"
	wg := sync.WaitGroup{}
	var handler http.Handler = reflectorHandler{}
	if os.Args[len(os.Args)-1] == "muxfront" {
		// handler = muxFrontendHandler{}
	}
	Start(&wg, host, cert, key, handler)
	wg.Wait()
}
