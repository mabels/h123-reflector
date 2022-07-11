package h123backend

import (
	"encoding/json"
	"log"
	"net/url"
	"path"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type BackendConfig struct {
	SubscribeUrl string
	RefreshFreq  time.Duration
	MuxEndPoint  string
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

type Backend struct {
	Config       BackendConfig
	Subscription MqttConnection
}

func (bd *Backend) Stop() {
	bd.Subscription.toStop = true
}

func (bd *Backend) SetupFrontendStream() error {
	cu, err := url.Parse(bd.Config.SubscribeUrl)
	if err != nil {
		return err
	}
	if cu.Scheme == "mqtt" {
		cu.Scheme = "tcp"
	}
	bd.Subscription.MqttPath = path.Join(cu.Path[1:], bd.Config.MuxEndPoint)
	cu.Path = "/"
	bd.Subscription.MqttCStr = cu.String()
	if bd.Config.RefreshFreq == 0 {
		bd.Config.RefreshFreq = time.Second
	}
	return nil
}

func (bd *Backend) StartFrontendStream() error {
	log.Printf("Backend listening on %s", bd.Subscription.MqttCStr)
	opts := mqtt.NewClientOptions().
		AddBroker(bd.Subscription.MqttCStr).SetClientID("h123-backend")
	opts.SetKeepAlive(2 * time.Second)
	// opts.SetDefaultPublishHandler(f)
	opts.SetPingTimeout(1 * time.Second)
	bd.Subscription.C = mqtt.NewClient(opts)
	if token := bd.Subscription.C.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	// if token := bd.Frontend.Subscribe(u.Path[1:], 0, nil); token.Wait() && token.Error() != nil {
	// 	fmt.Println(token.Error())
	// 	os.Exit(1)
	// }

	go func() {
		bd.Subscription.State.MuxEndPoint = bd.Config.MuxEndPoint
		for c := 0; !bd.Subscription.toStop; c++ {
			bd.Subscription.State.Now = time.Now()
			bd.Subscription.State.Status = "online"
			bd.Subscription.State.Loop = c
			out, err := json.Marshal(bd.Subscription.State)
			if err != nil {
				log.Fatal(err)
			}
			bd.Subscription.C.Publish(bd.Subscription.MqttPath, 0, false, out)
			time.Sleep(bd.Config.RefreshFreq)
		}
		bd.Subscription.State.Now = time.Now()
		bd.Subscription.State.Status = "offline"
		out, err := json.Marshal(bd.Subscription.State)
		if err != nil {
			log.Fatal(err)
		}
		bd.Subscription.C.Publish(bd.Subscription.MqttPath, 0, false, out)
		bd.Subscription.C.Disconnect(0)
	}()
	return nil
}

func (bd *Backend) HandleBackendStream() error {
	// u, err := url.Parse(bd.Config.BackendUrl)
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
	// bd.Frontend.C = mqtt.NewClient(opts)
	// if token := bd.Frontend.C.Connect(); token.Wait() && token.Error() != nil {
	// 	return token.Error()
	// }

	// // if token := bd.Frontend.Subscribe(u.Path[1:], 0, nil); token.Wait() && token.Error() != nil {
	// // 	fmt.Println(token.Error())
	// // 	os.Exit(1)
	// // }

	// go func() {
	// 	for !bd.Frontend.toStop {
	// 		bd.Frontend.State.Now = time.Now()
	// 		bd.Frontend.State.Status = "online"
	// 		out, err := json.Marshal(bd.Frontend.State)
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		bd.Frontend.C.Publish(u.Path[1:], 0, false, out)
	// 		time.Sleep(time.Second)
	// 	}
	// 	bd.Frontend.C.Disconnect(0)
	// }()
	return nil
}

func NewBackend(config BackendConfig) (Backend, error) {
	bd := Backend{
		Config: config,
	}

	error := bd.SetupFrontendStream()
	// bd.HandleBackendStream()

	return bd, error
}

func (bd *Backend) Start() error {
	err := bd.StartFrontendStream()
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

// import (
// 	"encoding/json"
// 	"log"
// 	"net/url"
// 	"time"

// 	mqtt "github.com/eclipse/paho.mqtt.golang"
// )

// type BackendConfig struct {
// 	FrontendUrl string
// }

// type Backend struct {
// 	Config BackendConfig
// }

// func (fe *Backend) HandleFrontendStream() error {
// 	u, err := url.Parse(bd.Config.FrontendUrl)
// 	if err != nil {
// 		return err
// 	}
// 	cu := u
// 	if cu.Scheme == "mqtt" {
// 		cu.Scheme = "tcp"
// 	}
// 	// cu.Path = "/"
// 	log.Printf("Frontend listening on %s", cu.String())
// 	opts := mqtt.NewClientOptions().
// 		AddBroker(cu.String()).SetClientID("h123-frontend")
// 	opts.SetKeepAlive(2 * time.Second)
// 	// opts.SetDefaultPublishHandler(f)
// 	opts.SetPingTimeout(1 * time.Second)
// 	bd.Frontend.C = mqtt.NewClient(opts)
// 	if token := bd.Frontend.C.Connect(); token.Wait() && token.Error() != nil {
// 		return token.Error()
// 	}

// 	// if token := bd.Frontend.Subscribe(u.Path[1:], 0, nil); token.Wait() && token.Error() != nil {
// 	// 	fmt.Println(token.Error())
// 	// 	os.Exit(1)
// 	// }

// 	go func() {
// 		for !bd.Frontend.toStop {
// 			bd.Frontend.State.Now = time.Now()
// 			bd.Frontend.State.Status = "online"
// 			out, err := json.Marshal(bd.Frontend.State)
// 			if err != nil {
// 				log.Fatal(err)
// 			}
// 			bd.Frontend.C.Publish(u.Path[1:], 0, false, out)
// 			time.Sleep(time.Second)
// 		}
// 		bd.Frontend.C.Disconnect(0)
// 	}()
// 	return nil
// }

// func NewBackend(bc BackendConfig) (Backend, error) {

// 	return Backend{}
// }
