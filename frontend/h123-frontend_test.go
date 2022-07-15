package frontend

import (
	"fmt"
	"testing"
	"time"

	"github.com/mabels/h123-reflector/backend"
)

func Test_FrontendBackendConnect(t *testing.T) {
	backends := []*backend.Backend{}
	frontends := []*Frontend{}
	for i := 0; i < 10; i++ {
		bcfg := backend.BackendConfig{
			BrokerUrl:      "mqtt://192.168.128.196:1883/h123/backend",
			MuxEndPointUrl: fmt.Sprintf("https://dev.adviser.com:%d/", 4711+i),
			Listen:         fmt.Sprintf("127.0.0.1:%d", 4711+i),
			CertFile:       "../dev.cert",
			KeyFile:        "../dev.key",
		}
		backend, err := backend.NewBackend(bcfg)
		if err != nil {
			t.Error(err)
		}
		err = backend.Start()
		if err != nil {
			t.Error(err)
		}
		backends = append(backends, backend)

		fcfg := FrontendConfig{
			BrokerUrl: "mqtt://192.168.128.196:1883",
			Listen:    fmt.Sprintf("127.0.0.1:%d", 4611+i),
		}
		frontend, err := NewFrontend(fcfg)
		if err != nil {
			t.Error(err)
		}
		frontends = append(frontends, frontend)
		err = frontend.Start()
		if err != nil {
			t.Error(err)
		}

	}
	t.Error("TODO: Test_FrontendBackendConnect")
	time.Sleep(10 * time.Second)
	for i, backend := range backends {
		backend.Stop()
		frontends[i].Stop()
	}
	time.Sleep(2 * time.Second)
}
