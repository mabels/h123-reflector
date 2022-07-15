package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/mabels/h123-reflector/models"
	"github.com/mabels/h123-reflector/reflector"
	"github.com/mabels/h123-reflector/utils"
)

func Test_BackendRegistration(t *testing.T) {
	start := time.Now()
	bd, error := NewBackend(BackendConfig{
		BrokerUrl:      "mqtt://192.168.128.196:1883/",
		RefreshFreq:    100 * time.Millisecond,
		MuxEndPointUrl: "https://127.0.0.1:4710",
		Listen:         "127.0.0.1:4710",
		CertFile:       "../dev.cert",
		KeyFile:        "../dev.key",
	})
	if error != nil {
		t.Error(error)
	}
	received := []mqtt.Message{}
	mqttHandler := func(client mqtt.Client, msg mqtt.Message) {
		// fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
		received = append(received, msg)
	}

	mqClient, err := utils.NewMqttConnection(bd.Config.BrokerUrl)
	if err != nil {
		t.Error(err)
	}
	err = mqClient.Connect()
	if err != nil {
		t.Error(err)
	}
	if err := mqClient.Subscribe(*bd.Config.StatusTopic, mqttHandler); err != nil {
		t.Error(err)
	}
	error = bd.Start()
	if error != nil {
		t.Error(error)
	}
	time.Sleep(time.Millisecond * 550)
	if len(received) != 6 {
		t.Errorf("Expected 5 message, got %d", len(received))
	}
	for i, msg := range received {
		var su models.ServerStatus
		json.Unmarshal(msg.Payload(), &su)
		if i != su.Loop {
			t.Errorf("Expected message %d, got %d", i, su.Loop)
		}
		if start.After(su.Now) {
			t.Errorf("Expected message %d to be newer than %s, got %s", i, start, su.Now)
		}
		start = su.Now
	}
	// received = []mqtt.Message{}
	bd.Stop()
	mqClient.Close()
}

func Test_RemoveBackendConnection(t *testing.T) {
	bd, err := NewBackend(BackendConfig{
		BrokerUrl:      "mqtt://192.168.128.196:1883/h123/backend",
		RefreshFreq:    100 * time.Millisecond,
		MuxEndPointUrl: "https://127.0.0.1:4709",
		Listen:         "127.0.0.1:4709",
		CertFile:       "../dev.cert",
		KeyFile:        "../dev.key",
	})
	if err != nil {
		t.Error(err)
	}
	err = bd.Start()
	if err != nil {
		t.Error(err)
	}
	if len(bd.UplinkConnections) != 0 {
		t.Error("Expected 0 uplink connections, got ", len(bd.UplinkConnections))
	}
	cnt := 3
	cons := make([]*utils.Quicer, 0, cnt)
	for i := 0; i < cnt; i++ {
		con, err := utils.QuicConnect(nil)
		if err != nil {
			t.Error(err)
		}
		cons = append(cons, con)
	}

	for _, con := range cons {
		presize, prereqs := bd.lenAndRequests()
		_, _, err := con.GetBody("https://dev.adviser.com:4709/", nil)
		if err != nil {
			t.Error(err)
		}
		// t.Error(string(body))
		size, reqs := bd.lenAndRequests()
		if size != presize+1 {
			t.Errorf("Expected %d uplink connections, got %d", presize+1, size)
		}
		if reqs != prereqs+1 {
			t.Errorf("Expected %d requests, got %d", prereqs+1, reqs)
		}
	}

	for i := 0; i < cnt*cnt; i++ {
		for j, con := range cons {
			presize, prereqs := bd.lenAndRequests()
			_, _, err := con.GetBody("https://dev.adviser.com:4709/", nil)
			if err != nil {
				t.Error(err)
			}
			// t.Error(string(body))
			size, reqs := bd.lenAndRequests()
			if size != presize {
				t.Errorf("%d:%d Expected %d uplink connections, got %d", i, j, presize, size)
			}
			if reqs != prereqs+1 {
				t.Errorf("%d:%d Expected %d requests, got %d", i, j, prereqs+1, reqs)
			}
		}
	}
	for _, con := range cons {
		presize, prereqs := bd.lenAndRequests()
		_, _, err = con.GetBody("https://dev.adviser.com:4709/", &http.Header{
			"X-H123-Uplink-Close": []string{bd.Config.Listen},
		})
		if err != nil {
			t.Error(err)
		}
		con.Close()
		size, reqs := bd.lenAndRequests()
		if size != presize-1 {
			t.Errorf("Expected %d uplink connections, got %d", presize, size)
		}
		if reqs != prereqs-uint64(cnt*cnt+1) {
			t.Errorf("Expected %d requests, got %d", prereqs, reqs)
		}
	}

	if len(bd.UplinkConnections) != 0 {
		t.Error("Expected 0 uplink connections, got ", len(bd.UplinkConnections))
	}
	bd.Stop()
}

func Test_RemoveInactiveFrontendConnection(t *testing.T) {
	bd, err := NewBackend(BackendConfig{
		BrokerUrl:          "mqtt://192.168.128.196:1883/h123/backend",
		RefreshFreq:        100 * time.Millisecond,
		MuxEndPointUrl:     "https://127.0.0.1:4708",
		Listen:             "127.0.0.1:4708",
		CertFile:           "../dev.cert",
		KeyFile:            "../dev.key",
		CloseAfterInactive: 100 * time.Millisecond,
	})
	if err != nil {
		t.Error(err)
	}
	err = bd.Start()
	if err != nil {
		t.Error(err)
	}
	con, err := utils.QuicConnect(nil)
	if err != nil {
		t.Error(err)
	}
	_, _, err = con.GetBody("https://dev.adviser.com:4708/", nil)
	if err != nil {
		t.Error(err)
	}
	if len(bd.UplinkConnections) != 1 {
		t.Error("Expected 1 uplink connections, got ", len(bd.UplinkConnections))
	}
	time.Sleep(220 * time.Millisecond)
	fmt.Println("Slept for 220ms", len(bd.UplinkConnections))
	if len(bd.UplinkConnections) != 0 {
		t.Error("Expected 0 uplink connections, got ", len(bd.UplinkConnections))
	}
	con.Close()
	bd.Stop()
}

func PostProxyRequest(t *testing.T, con *utils.Quicer, i int) {
	bodyReader := io.NopCloser(strings.NewReader("This Funky Body"))
	headers := http.Header{
		"User-Agent":      []string{"UserAgent"},
		"Accept-Encoding": []string{"gzip"},
		"X-MyTest-1":      []string{"test1"},
		"X-MyTest-2":      []string{"test2"},
		"X-H123-Txn":      []string{"Txn1"},
	}
	res, err := con.ProxyRequest("https://dev.adviser.com:4708/",
		"POST",
		fmt.Sprintf("https://dev.adviser.com:3000/realback-end/path?query=%d", i),
		&headers, bodyReader)
	if res == nil {
		t.Error(err)
		return
	}
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 200 {
		t.Error("Expected 200 OK, got ", res.Status)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Error(err)
	}
	refRes := models.ReflectorResponse{}
	err = json.Unmarshal(body, &refRes)
	if err != nil {
		t.Error(err)
	}
	if refRes.Error != nil {
		t.Error(*refRes.Error)
	}
	if refRes.Body == nil {
		t.Error("Expected body, got nil")
	}
	if refRes.Body != nil && *refRes.Body != "This Funky Body" {
		t.Errorf("Expected 'This Funky Body', got '%s'", *refRes.Body)
	}
	// t.Error(string(body))
	// if refRes.Header["Content-Length"][0] != fmt.Sprintf("%d", len("This Funky Body")) {
	// t.Errorf("Expected '%d', got '%s'", len("This Funky Body"), refRes.Header["Content-Length"][0])
	// }
	if refRes.Protocol != "HTTP/2.0" {
		t.Error("Expected HTTP/2.0, got ", refRes.Protocol)
	}
	if refRes.Method != "POST" {
		t.Error("Expected POST, got ", refRes.Method)
	}
	if refRes.Url != fmt.Sprintf("/realback-end/path?query=%d", i) {
		t.Errorf("Expected /realback-end/path?query=%d, got %s", i, refRes.Url)
	}
	// -1 because of the Content-Length header
	if len(refRes.Header) != len(headers) {
		t.Errorf("Expected %d headers, got %d", len(refRes.Header), len(headers))
	}
	if refRes.Header["X-H123-Txn"][0] != "Txn1" {
		t.Error("Expected Txn1, got ", refRes.Header["X-H123-Txn"][0])
	}
	if refRes.Header[http.CanonicalHeaderKey("X-MyTest-1")][0] != "test1" {
		t.Error("Expected Test-1, got ", refRes.Header[http.CanonicalHeaderKey("X-MyTest-1")][0])
	}
	if refRes.Header[http.CanonicalHeaderKey("X-MyTest-2")][0] != "test2" {
		t.Error("Expected Test-2, got ", refRes.Header[http.CanonicalHeaderKey("X-MyTest-2")][0])
	}

}

func GetProxyRequest(t *testing.T, con *utils.Quicer, i int) {
	headers := http.Header{
		"User-Agent":      []string{"UserAgent"},
		"Accept-Encoding": []string{"gzip"},
		"X-MyTest-1":      []string{"test1"},
		"X-MyTest-2":      []string{"test2"},
		"X-H123-Txn":      []string{"Txn1"},
	}
	res, err := con.ProxyRequest("https://dev.adviser.com:4708/",
		"GET", fmt.Sprintf("https://dev.adviser.com:3000/realback-end/path?query=%d", i),
		&headers, nil)
	if res == nil {
		t.Error(err)
		return
	}
	if err != nil {
		t.Error(err)
	}
	if res.StatusCode != 200 {
		t.Error("Expected 200 OK, got ", res.Status)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Error(err)
	}
	refRes := models.ReflectorResponse{}
	err = json.Unmarshal(body, &refRes)
	if err != nil {
		t.Error(err)
	}
	if refRes.Error != nil {
		t.Error(*refRes.Error)
	}
	if refRes.Body != nil {
		t.Errorf("Expected body, got nil:%s:%s", *refRes.Body, string(body))
	}
	if refRes.Protocol != "HTTP/2.0" {
		t.Error("Expected HTTP/2.0, got ", refRes.Protocol)
	}
	if refRes.Method != "GET" {
		t.Error("Expected GET, got ", refRes.Method)
	}
	if refRes.Url != fmt.Sprintf("/realback-end/path?query=%d", i) {
		t.Errorf("Expected /realback-end/path?query=%d, got %s", i, refRes.Url)
	}
	// -1 because of the Content-Length header
	if len(refRes.Header) != len(headers) {
		t.Errorf("Expected %d headers, got %d", len(refRes.Header), len(headers))
	}
	if refRes.Header["X-H123-Txn"][0] != "Txn1" {
		t.Error("Expected Txn1, got ", refRes.Header["X-H123-Txn"][0])
	}
	if refRes.Header[http.CanonicalHeaderKey("X-MyTest-1")][0] != "test1" {
		t.Error("Expected Test-1, got ", refRes.Header[http.CanonicalHeaderKey("X-MyTest-1")][0])
	}
	if refRes.Header[http.CanonicalHeaderKey("X-MyTest-2")][0] != "test2" {
		t.Error("Expected Test-2, got ", refRes.Header[http.CanonicalHeaderKey("X-MyTest-2")][0])
	}
}

func Test_HandleProxyRequest(t *testing.T) {
	host := "localhost:3000"
	cert := "../dev.cert"
	key := "../dev.key"
	wg := sync.WaitGroup{}
	handler := reflector.ReflectorHandler{}
	closeReflector := reflector.Start(&wg, host, cert, key, handler)

	bd, err := NewBackend(BackendConfig{
		BrokerUrl:      "mqtt://192.168.128.196:1883/h123/backend",
		MuxEndPointUrl: "https://127.0.0.1:4707",
		Listen:         "127.0.0.1:4707",
		CertFile:       "../dev.cert",
		KeyFile:        "../dev.key",
	})
	if err != nil {
		t.Error(err)
	}
	err = bd.Start()
	if err != nil {
		t.Error(err)
	}
	con, err := utils.QuicConnect(nil)
	if err != nil {
		t.Error(err)
	}
	reqWg := sync.WaitGroup{}
	for i := 0; i < 1000; i++ {
		reqWg.Add(1)
		go func(i int) {
			PostProxyRequest(t, con, i)
			GetProxyRequest(t, con, i)
			reqWg.Done()
		}(i)
	}
	reqWg.Wait()

	con.Close()
	bd.Stop()
	closeReflector()

}
