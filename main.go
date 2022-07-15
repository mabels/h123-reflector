package main

import (
	"net/http"
	"os"
	"sync"

	"github.com/mabels/h123-reflector/reflector"
)

func main() {
	host := "localhost:3000"
	cert := "./dev.cert"
	key := "./dev.key"
	wg := sync.WaitGroup{}
	var handler http.Handler = reflector.ReflectorHandler{}
	if os.Args[len(os.Args)-1] == "muxfront" {
		// handler = muxFrontendHandler{}
	}
	reflector.Start(&wg, host, cert, key, handler)
	wg.Wait()
}
