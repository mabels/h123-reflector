package models

import (
	"net/http"
	"time"
)

type ServerStatus struct {
	Status              string
	Now                 time.Time
	MuxEndPointUrl      string
	FrontendConnections int
	Requests            uint64
	Loop                int
}

type ReflectorResponse struct {
	RemoteAddr     string
	Protocol       string
	Url            string
	MuxEndPointUrl string
	Header         http.Header
	Body           *string `json:",omitempty"`
	Method         string
	Error          *string `json:",omitempty"`
}
