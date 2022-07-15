package utils

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"path"

	"github.com/google/uuid"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
)

type Quicer struct {
	Cfg          quic.Config
	RoundTripper *http3.RoundTripper
	Client       *http.Client
}

func QuicConnect(cfg *quic.Config) (*Quicer, error) {
	quicer := Quicer{}
	if cfg != nil {
		quicer.Cfg = *cfg
	}
	// func(FrameType, quic.Connection, quic.Stream, error) (hijacked bool, err error)

	quicer.RoundTripper = &http3.RoundTripper{
		QuicConfig: &quicer.Cfg,
		StreamHijacker: func(ftype http3.FrameType, conn quic.Connection, stream quic.Stream, _err error) (hijacked bool, err error) {
			log.Printf("HIJACKED:%s", conn.RemoteAddr().String())
			return false, nil
		},
	}
	quicer.Client = &http.Client{
		Transport: quicer.RoundTripper,
	}
	return &quicer, nil
}

func (q *Quicer) Close() {
	q.RoundTripper.Close()
}

func (q *Quicer) GetBody(urlstr string, header *http.Header) ([]byte, *http.Response, error) {
	u, err := url.Parse(urlstr)
	if err != nil {
		return nil, nil, err
	}
	if header == nil {
		header = &http.Header{}
	}
	req := &http.Request{
		Method: "GET",
		URL:    u,
		Header: *header,
	}
	resp, err := q.Client.Do(req)
	if err != nil {
		return nil, resp, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, err
	}
	return body, resp, nil
}

func (q *Quicer) ProxyRequest(
	proxyUrlStr string,
	method string,
	backendUrlStr string,
	header *http.Header,
	body io.ReadCloser) (*http.Response, error) {
	backendUrl, err := url.Parse(backendUrlStr)
	if err != nil {
		return nil, err
	}
	if header == nil {
		header = &http.Header{}
	}
	_, found := (*header)["X-H123-Backend-Host"]
	if !found {
		(*header)["X-H123-Backend-Host"] = []string{"https://" + backendUrl.Host}
	}
	_, found = (*header)["X-H123-Txn"]
	if !found {
		(*header)["X-H123-Txn"] = []string{uuid.New().String()}
	}
	proxyUrl, err := url.Parse(proxyUrlStr)
	if err != nil {
		return nil, err
	}
	backendUrl.Scheme = proxyUrl.Scheme
	backendUrl.Host = proxyUrl.Host
	backendUrl.Path = path.Join(proxyUrl.Path, backendUrl.Path)
	req := &http.Request{
		Method: method,
		URL:    backendUrl,
		Header: *header,
		Body:   body,
	}
	resp, err := q.Client.Do(req)
	return resp, err
	// return nil, fmt.Errorf("NOT IMPLEMENTED")
}
