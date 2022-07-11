package h123backend

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func Test_BackendRegistration(t *testing.T) {
	start := time.Now()
	fe, error := NewBackend(BackendConfig{
		SubscribeUrl: "mqtt://192.168.128.196:1883/h123/backend",
		RefreshFreq:  100 * time.Millisecond,
		MuxEndPoint:  "Test",
	})
	if error != nil {
		t.Error(error)
	}
	received := []mqtt.Message{}
	var msgHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
		received = append(received, msg)
	}

	opts := mqtt.NewClientOptions().
		AddBroker(fe.Subscription.MqttCStr).SetClientID("h123-frontend-test")
	opts.SetKeepAlive(2 * time.Second)
	opts.SetDefaultPublishHandler(msgHandler)
	opts.SetPingTimeout(1 * time.Second)
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Error(token.Error())
	}
	if token := c.Subscribe(fe.Subscription.MqttPath, 0, nil); token.Wait() && token.Error() != nil {
		t.Error(token.Error())
	}
	error = fe.Start()
	if error != nil {
		t.Error(error)
	}
	time.Sleep(time.Millisecond * 550)
	if len(received) != 6 {
		t.Errorf("Expected 5 message, got %d", len(received))
	}
	for i, msg := range received {
		var su ServerStatus
		json.Unmarshal(msg.Payload(), &su)
		if i != su.Loop {
			t.Errorf("Expected message %d, got %d", i, su.Loop)
		}
		if start.After(su.Now) {
			t.Errorf("Expected message %d to be newer than %s, got %s", i, start, su.Now)
		}
		start = su.Now
	}
	c.Disconnect(0)
}
