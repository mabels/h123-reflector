package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/mabels/h123-reflector/models"
	"github.com/mabels/h123-reflector/utils"
)

type BackendConfig struct {
	BrokerUrl           string
	StatusTopic         *string
	BaseConnectionTopic *string
	RefreshFreq         time.Duration
	MuxEndPointUrl      string
	Listen              string
	CertFile            string
	KeyFile             string
	CloseAfterInactive  time.Duration
	MqttCfg             *mqtt.ClientOptions
}

type WaitForClose struct {
	Request     http.Request // First request that started the close.
	Backend     *Backend
	LastRequest time.Time
	// RemoteAddr string
	Requests uint64
}

type Backend struct {
	Config                BackendConfig
	Mqtt                  *utils.MqttConnection
	Srv                   *http3.Server
	UplinkConnectionMutex sync.Mutex
	UplinkConnections     map[string]*WaitForClose
	ConnectionPool        *ConnectionPool
}

// func (bd *Backend) SetupFrontendStream() error {
// 	cu, err := url.Parse(bd.Config.BrokerUrl)
// 	if err != nil {
// 		return err
// 	}
// 	if cu.Scheme == "mqtt" {
// 		cu.Scheme = "tcp"
// 	}
// 	mux, err := url.Parse(bd.Config.MuxEndPointUrl)
// 	if err != nil {
// 		return err
// 	}
// 	bd.Subscription.MqttPath = strings.ReplaceAll(path.Join(cu.Path[1:], mux.Host), ":", "_")
// 	cu.Path = "/"
// 	bd.Subscription.MqttCStr = cu.String()
// 	if bd.Config.RefreshFreq == 0 {
// 		bd.Config.RefreshFreq = time.Second
// 	}
// 	return nil
// }

func (bd *Backend) lenAndRequests() (size int, request uint64) {
	bd.UplinkConnectionMutex.Lock()
	size = len(bd.UplinkConnections)
	request = uint64(0)
	for _, v := range bd.UplinkConnections {
		request += v.Requests
	}
	bd.UplinkConnectionMutex.Unlock()
	return size, request
}

func (bd *Backend) Stop() {
	bd.Mqtt.Stop()
}

func (bd *Backend) StartBackendConfigStream() error {
	bd.Mqtt.Connect()
	go func() {
		bd.Mqtt.State.MuxEndPointUrl = bd.Config.MuxEndPointUrl
		c := 0
		for ; !bd.Mqtt.ToStop; c++ {
			// fmt.Printf("mux-online: %s:%d\n", bd.Subscription.MqttPath, c)
			len, request := bd.lenAndRequests()
			bd.Mqtt.State.FrontendConnections = len
			bd.Mqtt.State.Requests = request
			bd.Mqtt.State.Now = time.Now()
			bd.Mqtt.State.Status = "online"
			bd.Mqtt.State.Loop = c
			out, err := json.Marshal(bd.Mqtt.State)
			if err != nil {
				log.Fatal(err)
			}
			err = bd.Mqtt.Publish(*bd.Config.StatusTopic, 1, false, out)
			if err != nil {
				log.Println(err)
			}
			time.Sleep(bd.Config.RefreshFreq)
		}
		// fmt.Printf("mux-offline: %s:%d\n", bd.Subscription.MqttPath, c)
		len, request := bd.lenAndRequests()
		// for _, _x := range bd.UplinkConnections {
		// We need to close the connection here, otherwise the client will keep trying to reconnect
		// continue
		// }
		bd.Mqtt.State.FrontendConnections = len
		bd.Mqtt.State.Requests = request
		bd.Mqtt.State.Now = time.Now()
		bd.Mqtt.State.Status = "offline"
		out, err := json.Marshal(bd.Mqtt.State)
		if err != nil {
			log.Fatal(err)
		}
		err = bd.Mqtt.Publish(*bd.Config.StatusTopic, 1, false, out)
		if err != nil {
			log.Println(err)
		}
		bd.Mqtt.Close()
	}()
	return nil
}

func (bd *Backend) mqttAddConnection() func(action Action, key string, value *Connection) {

	return func(action Action, key string, value *Connection) {
		key = strings.ReplaceAll(key, ":", "_")
		key = strings.ReplaceAll(key, "//", "")
		topic := path.Join(*bd.Config.BaseConnectionTopic, key)
		out, err := json.Marshal(value)
		if err != nil {
			log.Printf("Error marshalling connection: %s\n", err)
			return
		}
		bd.Mqtt.Publish(topic, 1, false, out)
	}
}

func NewBackend(config BackendConfig) (*Backend, error) {
	mqtt, err := utils.NewMqttConnection(config.BrokerUrl, config.MqttCfg)
	if err != nil {
		return nil, err
	}
	if config.StatusTopic == nil {
		my := fmt.Sprintf("h123/backend/%s/status", config.Listen)
		my = strings.ReplaceAll(my, ":", "_")
		config.StatusTopic = &my
	}
	if config.BaseConnectionTopic == nil {
		my := path.Join(path.Dir(*config.StatusTopic), "connections")
		config.BaseConnectionTopic = &my
	}
	bd := Backend{
		Config:            config,
		UplinkConnections: map[string]*WaitForClose{},
		Mqtt:              mqtt,
	}
	bd.ConnectionPool = NewConnectionPool(bd.mqttAddConnection())
	return &bd, nil
}

type connectionPoolHandler struct {
	backend *Backend
}

func (cph connectionPoolHandler) handleWaitConnection(w *http.ResponseWriter, r *http.Request) {
	cph.backend.UplinkConnectionMutex.Lock()
	my, found := cph.backend.UplinkConnections[r.RemoteAddr]
	if !found {
		my = &WaitForClose{
			Request: *r,
			Backend: cph.backend,
		}
		cph.backend.UplinkConnections[r.RemoteAddr] = my
	}
	my.LastRequest = time.Now()
	cph.backend.UplinkConnectionMutex.Unlock()
	atomic.AddUint64(&my.Requests, 1)
	_, found = r.Header["X-H123-Uplink-Close"]
	if found {
		my.Backend.DeleteUplinkConnectionWithLock(r.RemoteAddr)
	}
}

func (cph connectionPoolHandler) reflectorResponse(w http.ResponseWriter, r *http.Request, status int, err error) {
	var errStr *string
	if err != nil {
		my := err.Error()
		errStr = &my
	}

	out, _ := json.MarshalIndent(models.ReflectorResponse{
		RemoteAddr:     r.RemoteAddr,
		Protocol:       r.Proto,
		Url:            r.URL.String(),
		Header:         r.Header,
		Error:          errStr,
		MuxEndPointUrl: cph.backend.Config.MuxEndPointUrl,
	}, "", "  ")
	w.Header().Add("Content-Type", "application/json")
	rstat := 200
	if status > 0 {
		rstat = status
	}
	w.WriteHeader(rstat)
	w.Write(out)
}

func (cph connectionPoolHandler) proxy(bSchema string, bHost string, w http.ResponseWriter, r *http.Request) {
	myUrl := *r.URL
	myUrl.Scheme = bSchema
	myUrl.Host = bHost

	req := &http.Request{
		Method: r.Method,
		URL:    &myUrl,
		Header: r.Header,
		Body:   r.Body,
	}
	conn, err := cph.backend.ConnectionPool.Setup(bSchema, bHost)
	if err != nil {
		cph.reflectorResponse(w, r, http.StatusInternalServerError, err)
		return
	}
	resp, err := conn.Do(req)
	if err != nil {
		cph.reflectorResponse(w, r, http.StatusInternalServerError, err)
		return
	}
	// resR := models.ReflectorResponse{
	// 	Header: r.Header,
	// }
	// resOut, _ := json.Marshal(resR)
	// resp := &http.Response{
	// 	StatusCode: http.StatusOK,
	// 	Header:     make(http.Header),
	// 	Body:       io.NopCloser(bytes.NewBufferString(string(resOut))),
	// }
	w.WriteHeader(resp.StatusCode)
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		cph.reflectorResponse(w, r, http.StatusInsufficientStorage, err)
		return
	}
	w.Write(body)
}

func (cph connectionPoolHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cph.handleWaitConnection(&w, r)
	backCon, found := r.Header["X-H123-Backend-Host"]
	if !found || len(backCon) == 0 {
		cph.reflectorResponse(w, r, http.StatusBadRequest, fmt.Errorf("X-H123-Backend-Host header is missing"))
		return
	}
	_, found = r.Header["X-H123-Txn"]
	if !found {
		cph.reflectorResponse(w, r, http.StatusBadRequest, fmt.Errorf("X-H123-Txn header is missing"))
		return
	}
	backend, err := url.Parse(backCon[0])
	if err != nil {
		cph.reflectorResponse(w, r, http.StatusBadGateway, err)
		return
	}
	cph.proxy(backend.Scheme, backend.Host, w, r)
	// cph.reflectorResponse(w, r, http.StatusOK, nil)
}

func (bd *Backend) DeleteUplinkConnectionWithLock(remoteAddr string) {
	bd.UplinkConnectionMutex.Lock()
	bd.DeleteUplinkConnection(remoteAddr)
	bd.UplinkConnectionMutex.Unlock()
}

func (bd *Backend) DeleteUplinkConnection(remoteAddr string) {
	delete(bd.UplinkConnections, remoteAddr)
	// log.Printf("Remove MuxFrontendConnection: %s\n", remoteAddr)
}

func (bd *Backend) Start() error {
	err := bd.StartBackendConfigStream()
	if err != nil {
		return err
	}
	handler := connectionPoolHandler{backend: bd}

	bd.Srv = &http3.Server{
		Addr:    bd.Config.Listen,
		Handler: handler,
		StreamHijacker: func(_f http3.FrameType, c quic.Connection, _s quic.Stream, _err error) (hijacked bool, err error) {
			log.Fatalln("StreamHijacker", c.Context())
			return false, nil
		},
		UniStreamHijacker: func(_s http3.StreamType, c quic.Connection, _r quic.ReceiveStream, _err error) (hijacked bool) {
			log.Fatalln("UniStreamHijacker", c.Context())
			return false
		},
	}
	done := false
	go func() {
		if err := bd.Srv.ListenAndServeTLS(bd.Config.CertFile, bd.Config.KeyFile); err != quic.ErrServerClosed {
			log.Fatalf("ListenAndServeTLS(): %v", err)
		}
		done = true
	}()
	go func() {
		if bd.Config.CloseAfterInactive == 0 {
			bd.Config.CloseAfterInactive = 60 * time.Second
		}
		for !done {
			time.Sleep(bd.Config.CloseAfterInactive)
			bd.UplinkConnectionMutex.Lock()
			now := time.Now()
			for _, uc := range bd.UplinkConnections {
				if uc.LastRequest.Equal(time.Time{}) {
					continue
				}
				// fmt.Printf("%s:%d:%d\n", uc.Backend.Config.Listen, now.Sub(uc.LastRequest), bd.Config.CloseAfterInactive)
				if uc.LastRequest.Add(bd.Config.CloseAfterInactive).Before(now) {
					bd.DeleteUplinkConnection(uc.Request.RemoteAddr)
				}
			}
			bd.UplinkConnectionMutex.Unlock()
		}
	}()
	return nil
}
