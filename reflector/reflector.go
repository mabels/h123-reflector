package reflector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/mabels/h123-reflector/models"
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

type ReflectorHandler struct {
}

func (_x ReflectorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// fmt.Fprintf(os.Stderr, "reflector: %s:%s\n", r.RemoteAddr, r.URL.RawQuery)
	var bodyStr *string
	var errStr *string
	body, err := io.ReadAll(r.Body)
	if err != nil {
		my := fmt.Errorf("Error reading body: %v", err).Error()
		errStr = &my
	} else if len(body) > 0 {
		my := string(body)
		bodyStr = &my
	}

	if err != nil {
		my := err.Error()
		errStr = &my
	}
	out, _ := json.MarshalIndent(models.ReflectorResponse{
		RemoteAddr: r.RemoteAddr,
		Protocol:   r.Proto,
		Url:        r.URL.String(),
		Header:     r.Header,
		Method:     r.Method,
		Body:       bodyStr,
		Error:      errStr,
	}, "", "  ")
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(out)
}

func Start(wg *sync.WaitGroup, host string, cert string, key string, handler http.Handler) func() {
	h12 := h12server(wg, host, cert, key, handler)
	h3 := h3server(wg, host, cert, key, handler)
	return func() {
		ctx, _ := context.WithDeadline(context.Background(), time.Now())
		h12.Shutdown(ctx)
		h3.Close()
	}
}
