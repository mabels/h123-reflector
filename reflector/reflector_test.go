package reflector

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/mabels/h123-reflector/models"
)

func Test_H1(t *testing.T) {
	wg := sync.WaitGroup{}
	stopper := Start(&wg, "localhost:3000", "./dev.cert", "./dev.key", ReflectorHandler{})

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		},
	}
	keepalived := ""
	for i := 0; i < 10; i++ {
		resp, err := httpClient.Get("https://dev.adviser.com:3000/")
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}
		res := models.ReflectorResponse{}
		json.Unmarshal(body, &res)
		if res.Protocol != "HTTP/1.1" {
			t.Error("Expected HTTP/1.1", string(body))
		}
		if keepalived == "" {
			keepalived = res.RemoteAddr
		} else if keepalived != res.RemoteAddr {
			t.Error(keepalived, res.RemoteAddr)
		}
	}
	stopper()
}

func Test_H2(t *testing.T) {
	wg := sync.WaitGroup{}
	stopper := Start(&wg, "localhost:3000", "./dev.cert", "./dev.key", ReflectorHandler{})
	time.Sleep(time.Millisecond * 200)
	httpClient := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
		},
	}
	keepalived := ""
	for i := 0; i < 10; i++ {
		resp, err := httpClient.Get("https://dev.adviser.com:3000/")
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}
		res := models.ReflectorResponse{}
		json.Unmarshal(body, &res)
		if res.Protocol != "HTTP/2.0" {
			t.Error("Expected HTTP/2.0", string(body))
		}
		if keepalived == "" {
			keepalived = res.RemoteAddr
		} else if keepalived != res.RemoteAddr {
			t.Error(keepalived, res.RemoteAddr)
		}
	}
	stopper()
}

func Test_H3(t *testing.T) {
	wg := sync.WaitGroup{}
	stopper := Start(&wg, "localhost:3000", "./dev.cert", "./dev.key", ReflectorHandler{})

	var qconf quic.Config
	roundTripper := &http3.RoundTripper{
		QuicConfig: &qconf,
	}
	defer roundTripper.Close()
	client := &http.Client{
		Transport: roundTripper,
	}
	keepalived := ""
	for i := 0; i < 10; i++ {
		resp, err := client.Get("https://dev.adviser.com:3000/")
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}
		res := models.ReflectorResponse{}
		json.Unmarshal(body, &res)
		if res.Protocol != "HTTP/3.0" {
			t.Error("Expected HTTP/3.0", string(body))
		}
		if keepalived == "" {
			keepalived = res.RemoteAddr
		} else if keepalived != res.RemoteAddr {
			t.Error(keepalived, res.RemoteAddr)
		}
	}
	stopper()
}
