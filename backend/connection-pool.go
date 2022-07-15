package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/mabels/h123-reflector/utils"
)

type Connection struct {
	Client   http.Client  `json:"-"`
	Creating sync.RWMutex `json:"-"`
	Schema   string
	Host     string
	IsQuic   bool
	Request  int
}

type Action string

const (
	Add    Action = "add"
	Delete        = "delete"
)

type ConnectionPool struct {
	poolMutex sync.RWMutex
	pool      map[string]*Connection
	events    func(Action, string, *Connection)
}

func poolKey(schema string, host string) string {
	return schema + "://" + host
}

func (cp *ConnectionPool) addConnection(pKey string, conn *Connection) *Connection {
	cp.poolMutex.Lock()
	con, found := cp.pool[pKey]
	if !found {
		if conn == nil {
			conn = &Connection{}
		}
		con = conn
		cp.pool[pKey] = con
		// fmt.Printf("New connection %s\n", pKey)
	} // else {
	// 	// fmt.Printf("Existing connection %s\n", pKey)
	// }
	cp.poolMutex.Unlock()
	if !found {
		go func(con *Connection) {
			// add Connection to pool
			cp.events(Add, pKey, con)
		}(con)
	}
	return con
}

func (cp *ConnectionPool) tryQuic(schema string, host string, req *http.Request, con *Connection) {
	// I'm that creating the Quic connection
	myUrl := *req.URL
	myUrl.Scheme = "http"
	if schema[len(schema)-1] == 's' {
		myUrl.Scheme = "https"
	}
	headReq := http.Request{
		Method: "HEAD",
		URL:    &myUrl,
		Header: req.Header,
	}
	res, err := con.Client.Do(&headReq)
	if err != nil {
		con.Creating.Unlock()
		// fallback to h2
		return
	}
	altSrv := res.Header.Get("alt-srv")
	for _, p := range strings.Split(altSrv, ";") {
		if strings.HasPrefix(p, "h3=") {
			var quicHost string
			err = json.Unmarshal([]byte(p[len("h3="):]), &quicHost)
			if err != nil {
				con.Creating.Unlock()
				// fallback to h2
				return
			}
			if strings.HasPrefix(quicHost, ":") {
				// relative to host
				quicHost = myUrl.Hostname() + quicHost
			}
			// What If Alt-Srv is set but not working
			qcon, err := utils.QuicConnect(nil)
			if err != nil {
				con.Creating.Unlock()
				// fallback to h2
				return
			}
			// Setup the quic connection
			con.Host = quicHost
			con.Client = *qcon.Client
			con.Creating.Unlock()
			return
		}
	}
}

func (c *Connection) Do(req *http.Request) (*http.Response, error) {
	if !(req.URL.Scheme == c.Schema && req.URL.Host == c.Host) {
		return nil, fmt.Errorf("Connection not setup missmatch for %s://%s %s://%s", c.Schema, c.Host, req.URL.Scheme, req.URL.Host)
	}
	res, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	_, found := res.Header["alt-srv"]
	if !c.IsQuic && found {
		// is not implemented yet
	}
	return res, err
}

func (cp *ConnectionPool) Setup(schema string, host string) (*Connection, error) {
	pKey := poolKey(schema, host)
	cp.poolMutex.RLock()
	con, found := cp.pool[pKey]
	cp.poolMutex.RUnlock()
	if !found {
		switch schema {
		case "http":
		case "https":

		default:
			return nil, fmt.Errorf("UNKNOWN SCHEMA %s", schema)
			// case "h3":
			// 	// do http HEAD request and look for Alt-Srv
			// case "h3s":
			// 	// do https HEAD request and look for Alt-Srv
			// default:
			// 	ccon := &Connection{
			// 		Schema: schema,
			// 		Host:   host,
			// 	}
			// 	ccon.Creating.Lock()
			// 	con = cp.addConnection(pKey, ccon)
			// 	if con == ccon {
			// 		cp.tryQuic(schema, host, con)
			// 	}
		}
		con = cp.addConnection(pKey, &Connection{
			Schema: schema,
			Host:   host,
		})
	}
	con.Creating.RLock()
	defer con.Creating.RUnlock()
	return con, nil
}

type EventFn = func(Action, string, *Connection)

func NewConnectionPool(fns ...EventFn) *ConnectionPool {
	fn := func(a Action, pKey string, c *Connection) {}
	if len(fns) > 0 {
		fn = fns[0]
	}
	return &ConnectionPool{
		pool:   map[string]*Connection{},
		events: fn,
	}
}
