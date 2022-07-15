package backend

import (
	"fmt"
	"testing"
)

func Test_NewConnectionPool(t *testing.T) {
	for i := 0; i < 10; i++ {
		cp := NewConnectionPool()
		if cp == nil {
			t.Error("Expected non-nil ConnectionPool")
		}
	}
}

type EventHolder struct {
	action Action
	key    string
	conn   *Connection
}

func Test_ConnectionPoolWithEvents(t *testing.T) {
	events := make(chan EventHolder, 10)
	cp := NewConnectionPool(func(a Action, s string, c *Connection) {
		events <- EventHolder{a, s, c}
	})
	conn, err := cp.Setup("http", "dev.adviser.com:3000")
	if err != nil {
		t.Error(err)
	}
	eventAdd := <-events
	if conn != eventAdd.conn {
		t.Errorf("Expected same ConnectionHandler:%p:%v", conn, &eventAdd.conn)
	}
	if eventAdd.key != "http://dev.adviser.com:3000" {
		t.Error("Expected key 'dev.adviser.com:3000'", eventAdd.key)
	}
	if eventAdd.action != Add {
		t.Error("Expected Add action")
	}

}

func Test_GetConnectionPoolSeq(t *testing.T) {
	cp := NewConnectionPool()
	if cp == nil {
		t.Error("Expected non-nil ConnectionPool")
		return
	}
	if len(cp.pool) != 0 {
		t.Error("Expected non-empty ConnectionPool")
	}
	for i := 0; i < 100; i++ {
		host := fmt.Sprintf("dev.adviser.com:%d", 3000+i)
		chttp, err := cp.Setup("http", host)
		if err != nil {
			t.Error(err)
		}
		if len(cp.pool) != i*2+1 {
			t.Errorf("Expected %d connections, got %d", i*2, len(cp.pool))
		}
		chttps, err := cp.Setup("https", host)
		if err != nil {
			t.Error(err)
		}
		if len(cp.pool) != i*2+2 {
			t.Errorf("Expected %d connections, got %d", i+2, len(cp.pool))
		}
		if chttp == chttps {
			t.Error("Expected different ConnectionHandlers")
		}
		// retry
		_, err = cp.Setup("http", host)
		if err != nil {
			t.Error(err)
		}
		if len(cp.pool) != i*2+2 {
			t.Errorf("Expected %d connections, got %d", i+2, len(cp.pool))
		}
		_, err = cp.Setup("https", host)
		if err != nil {
			t.Error(err)
		}
		if len(cp.pool) != i*2+2 {
			t.Errorf("Expected %d connections, got %d", i+2, len(cp.pool))
		}
	}
}

// func Test_GetConnectionPoolConcurrent(t *testing.T) {
// 	cp := NewConnectionPool()
// 	if cp == nil {
// 		t.Error("Expected non-nil ConnectionPool")
// 	}
// 	if len(cp.pool) != 0 {
// 		t.Error("Expected non-empty ConnectionPool")
// 	}
// 	wg := sync.WaitGroup{}
// 	for i := 0; i < 10000; i++ {
// 		wg.Add(1)
// 		go func() {
// 			host := fmt.Sprintf("dev.adviser.com:%d", 3100+i)
// 			cp.Get("http", host)
// 			cp.Get("https", host)
// 			wg.Done()
// 		}()
// 	}
// 	wg.Wait()
// 	if len(cp.pool) != 20000 {
// 		t.Error("Expected non-empty ConnectionPool")
// 	}

// }
