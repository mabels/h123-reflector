package h123frontend

import (
	"fmt"
	"log"
	"net/url"
	"path"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type FrontendConfig struct {
	FrontendUrl string
	BackendUrl  string
	RefreshFreq time.Duration
	MuxEndPoint string
}

type ServerStatus struct {
	Status      string
	Now         time.Time
	MuxEndPoint string
	Loop        int
}

type MqttConnection struct {
	C        mqtt.Client
	MqttCStr string
	MqttPath string
	toStop   bool
	State    ServerStatus
}

type MuxDownStream struct {
}

func (mds *MuxDownStream) receive() {

}

type Frontend struct {
	Config        FrontendConfig
	Frontend      MqttConnection
	MuxDownStream MuxDownStream
}

func (fe *Frontend) Stop() {
	fe.Frontend.toStop = true
}

func (fe *Frontend) SetupFrontendStream() error {
	cu, err := url.Parse(fe.Config.FrontendUrl)
	if err != nil {
		return err
	}
	if cu.Scheme == "mqtt" {
		cu.Scheme = "tcp"
	}
	fe.Frontend.MqttPath = path.Join(cu.Path[1:], fe.Config.MuxEndPoint)
	cu.Path = "/"
	fe.Frontend.MqttCStr = cu.String()
	if fe.Config.RefreshFreq == 0 {
		fe.Config.RefreshFreq = time.Second
	}
	return nil
}

func (fe *Frontend) publishHandler() func(client mqtt.Client, msg mqtt.Message) {
	return func(client mqtt.Client, msg mqtt.Message) {
		fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
	}
}

func (fe *Frontend) StartFrontendStream() error {
	log.Printf("Frontend listening on %s", fe.Frontend.MqttCStr)
	opts := mqtt.NewClientOptions().
		AddBroker(fe.Frontend.MqttCStr).SetClientID("h123-frontend")
	opts.SetKeepAlive(2 * time.Second)
	opts.SetDefaultPublishHandler(fe.publishHandler())
	opts.SetPingTimeout(1 * time.Second)
	fe.Frontend.C = mqtt.NewClient(opts)
	if token := fe.Frontend.C.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	if token := fe.Frontend.C.Subscribe(fe.Frontend.MqttPath, 0, nil); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (fe *Frontend) HandleBackendStream() error {
	// u, err := url.Parse(fe.Config.BackendUrl)
	// if err != nil {
	// 	return err
	// }
	// cu := u
	// if cu.Scheme == "mqtt" {
	// 	cu.Scheme = "tcp"
	// }
	// cu.Path = "/"
	// log.Printf("Frontend listening on %s", cu.String())
	// opts := mqtt.NewClientOptions().
	// 	AddBroker(cu.String()).SetClientID("h123-frontend")
	// opts.SetKeepAlive(2 * time.Second)
	// // opts.SetDefaultPublishHandler(f)
	// opts.SetPingTimeout(1 * time.Second)
	// fe.Frontend.C = mqtt.NewClient(opts)
	// if token := fe.Frontend.C.Connect(); token.Wait() && token.Error() != nil {
	// 	return token.Error()
	// }

	// // if token := fe.Frontend.Subscribe(u.Path[1:], 0, nil); token.Wait() && token.Error() != nil {
	// // 	fmt.Println(token.Error())
	// // 	os.Exit(1)
	// // }

	// go func() {
	// 	for !fe.Frontend.toStop {
	// 		fe.Frontend.State.Now = time.Now()
	// 		fe.Frontend.State.Status = "online"
	// 		out, err := json.Marshal(fe.Frontend.State)
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		fe.Frontend.C.Publish(u.Path[1:], 0, false, out)
	// 		time.Sleep(time.Second)
	// 	}
	// 	fe.Frontend.C.Disconnect(0)
	// }()
	return nil
}

func NewFrontend(config FrontendConfig) (Frontend, error) {
	fe := Frontend{
		Config: config,
	}

	error := fe.SetupFrontendStream()
	// fe.HandleBackendStream()

	return fe, error
}

func (fe *Frontend) Start() error {
	err := fe.StartFrontendStream()
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
