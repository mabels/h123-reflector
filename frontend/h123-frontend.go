package frontend

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/mabels/h123-reflector/models"
	"github.com/mabels/h123-reflector/utils"
)

type FrontendConfig struct {
	BrokerUrl      string
	BackendTopic   *string
	ReclaimFreq    time.Duration
	Listen         string
	MaxBackends    int
	BackendQuicCfg quic.Config
	MqttCfg        mqtt.ClientOptions
}

type BackendConnection struct {
	http *http.Client
}

func (bc *BackendConnection) Close() {

}

type MuxConnection struct {
	state         models.ServerStatus
	connection    *BackendConnection
	MuxDownStream *MuxDownStream
}

type MuxDownStream struct {
	Config           *FrontendConfig
	toStop           bool
	updateMutex      sync.Mutex
	updated          map[string]models.ServerStatus
	active           map[string]*MuxConnection
	connectToBackend chan *MuxConnection
}

func NewMuxDownStream(cfg *FrontendConfig) *MuxDownStream {
	if cfg.ReclaimFreq == 0 {
		cfg.ReclaimFreq = time.Second
	}
	if cfg.MaxBackends == 0 {
		cfg.MaxBackends = 64
	}
	return &MuxDownStream{
		Config:           cfg,
		updated:          map[string]models.ServerStatus{},
		active:           map[string]*MuxConnection{},
		connectToBackend: make(chan *MuxConnection, cfg.MaxBackends),
	}
}

func (mds *MuxDownStream) updateState(state *models.ServerStatus) {
	mds.updateMutex.Lock()
	mds.updated[state.MuxEndPointUrl] = *state
	mds.updateMutex.Unlock()
}

func (mds *MuxDownStream) stop() {
	mds.connectToBackend <- nil
	mds.toStop = true
}

func NewBackendConnection(cfg *FrontendConfig, mxc *MuxConnection) (*http.Client, error) {
	roundTripper := &http3.RoundTripper{
		QuicConfig: &cfg.BackendQuicCfg,
	}
	defer roundTripper.Close()
	client := &http.Client{
		Transport: roundTripper,
	}
	resp, err := client.Get(mxc.state.MuxEndPointUrl)
	if err != nil {
		return client, err
	}
	if resp.StatusCode != 200 {
		return client, fmt.Errorf("Returned %s: %s", mxc.state.MuxEndPointUrl, resp.Status)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return client, err
	}
	res := models.ReflectorResponse{}
	json.Unmarshal(body, &res)
	if res.Protocol != "HTTP/3.0" {
		return client, fmt.Errorf("Expected HTTP/3.0: %s", string(body))
	}
	if res.MuxEndPointUrl != mxc.state.MuxEndPointUrl {
		return client, fmt.Errorf("Expected %s == %s", res.MuxEndPointUrl, mxc.state.MuxEndPointUrl)
	}
	return client, nil
}

func (mds *MuxDownStream) start() {
	go func() {
		for !(mds.toStop) {
			updateState := make([]models.ServerStatus, 0, len(mds.updated))
			mds.updateMutex.Lock()
			for _, state := range mds.updated {
				updateState = append(updateState, state)
			}
			mds.updated = map[string]models.ServerStatus{}
			mds.updateMutex.Unlock()

			for _, state := range updateState {
				mxc, found := mds.active[state.MuxEndPointUrl]
				if !found {
					// fmt.Printf("New backend %s\n", state.MuxEndPointUrl)
					mxc = &MuxConnection{state: state, MuxDownStream: mds}
					mds.active[state.MuxEndPointUrl] = mxc
					mds.connectToBackend <- mxc
				}
				mxc.state = state
			}
			now := time.Now()
			for _, mxc := range mds.active {
				if mxc.state.Status != "online" ||
					mxc.state.Now.Add(mds.Config.ReclaimFreq*2).Before(now) {
					fmt.Printf("Removing %s\n", mxc.state.MuxEndPointUrl)
					mxc.connection.Close()
					delete(mds.active, mxc.state.MuxEndPointUrl)
				}
			}
			time.Sleep(mds.Config.ReclaimFreq)
		}
		for _, mxc := range mds.active {
			mxc.connection.Close()
		}
	}()
	go func() {
		for !(mds.toStop) {
			mxc := <-mds.connectToBackend
			if mxc == nil {
				break
			}
			fmt.Printf("Connecting to %s:%s\n", mxc.MuxDownStream.Config.Listen, mxc.state.MuxEndPointUrl)
			connection, err := NewBackendConnection(mds.Config, mxc)
			if err != nil {
				log.Printf("Error connecting to backend: %s", err)
				continue
			}
			mxc.connection = &BackendConnection{http: connection}
		}
	}()
}

type Frontend struct {
	Config        FrontendConfig
	Mqtt          utils.MqttConnection
	muxDownStream *MuxDownStream
}

func (fe *Frontend) Stop() {
	fe.Mqtt.Stop()
	fe.muxDownStream.stop()
}

func (fe *Frontend) Setup() error {
	return nil
}

func (fe *Frontend) receiveBackendTopic() func(client mqtt.Client, msg mqtt.Message) {
	return func(client mqtt.Client, msg mqtt.Message) {
		state := models.ServerStatus{}
		err := json.Unmarshal(msg.Payload(), &state)
		if err != nil {
			log.Println(err)
			return
		}
		fe.muxDownStream.updateState(&state)
	}
}

func NewFrontend(config FrontendConfig) (*Frontend, error) {
	mqtt, err := utils.NewMqttConnection(config.BrokerUrl)
	if err != nil {
		return nil, err
	}
	fe := Frontend{
		Config: config,
		Mqtt:   *mqtt,
	}
	fe.muxDownStream = NewMuxDownStream(&fe.Config)
	return &fe, nil
}

func (fe *Frontend) Start() error {
	fe.muxDownStream.start()

	if fe.Config.BackendTopic == nil {
		my := "h123/backend/#"
		fe.Config.BackendTopic = &my
	}
	err := fe.Mqtt.Subscribe(*fe.Config.BackendTopic, fe.receiveBackendTopic())
	if err != nil {
		return err
	}
	return nil
}

// type muxFrontendHandler struct {
// }

// func (_ muxFrontendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
// 	out, _ := json.MarshalIndent(Response{
// 		Protocol: r.Proto,
// 		Url:      r.URL.String(),
// 		Header:   r.Header,
// 	}, "", "  ")
// 	w.Header().Add("Content-Type", "application/json")
// 	w.WriteHeader(200)
// 	w.Write(out)
// }
