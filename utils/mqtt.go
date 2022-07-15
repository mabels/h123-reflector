package utils

import (
	"fmt"
	"log"
	"net/url"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/mabels/h123-reflector/models"
)

type MqttConnection struct {
	c             mqtt.Client
	ConnectionUrl string
	// MqttPath      string
	ToStop        bool
	State         models.ServerStatus
	ClientOptions *mqtt.ClientOptions
}

func (mq *MqttConnection) Stop() {
	mq.ToStop = true
}

func NewMqttConnection(brokerUrl string, opts ...*mqtt.ClientOptions) (*MqttConnection, error) {
	cu, err := url.Parse(brokerUrl)
	if err != nil {
		return nil, err
	}
	if cu.Scheme == "mqtt" {
		cu.Scheme = "tcp"
	}
	ret := MqttConnection{}
	// ret.MqttPath = path.Join(cu.Path[1:], "#")
	cu.Path = "/"
	ret.ConnectionUrl = cu.String()
	if len(opts) == 0 || opts[0] == nil {
		ret.ClientOptions = mqtt.NewClientOptions().
			AddBroker(ret.ConnectionUrl).
			SetClientID(fmt.Sprintf("h123-%s", uuid.New().String()))
		// make this configurable
		ret.ClientOptions.SetKeepAlive(2 * time.Second)
		ret.ClientOptions.SetPingTimeout(1 * time.Second)
	} else {
		my := *opts[0]
		ret.ClientOptions = &my
	}
	ret.c = mqtt.NewClient(ret.ClientOptions)
	return &ret, nil
}

func (mq *MqttConnection) Connect() error {
	if token := mq.c.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (mq *MqttConnection) Subscribe(subPath string, pubFn func(client mqtt.Client, msg mqtt.Message)) error {
	log.Printf("Mqtt Subscribe on %s:%s", mq.ConnectionUrl, subPath)
	if token := mq.c.Subscribe(subPath, 1, pubFn); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (mq *MqttConnection) Publish(topic string, qos byte, retained bool, payload []byte) error {
	if token := mq.c.Publish(topic, qos, retained, payload); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (mq *MqttConnection) Close() {
	mq.c.Disconnect(0)
}
