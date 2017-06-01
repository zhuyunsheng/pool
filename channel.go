package pool

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

//poll conf
type PoolConf struct {
	MinCap      int //最小连接数
	MaxCap      int //最大连接数
	New         func() (interface{}, error)
	Close       func(interface{}) error
	IdleTimeout time.Duration
}

//channelPool 存放链接信息
type pool struct {
	mx          sync.Mutex
	conns       chan *idleConn
	maxCap      int
	active      int
	new         func() (interface{}, error)
	close       func(interface{}) error
	idleTimeout time.Duration
}

type idleConn struct {
	conn interface{}
	t    time.Time
}

//NewChannelPool 初始化链接
func NewPool(poolConf *PoolConf) (Pool, error) {
	c := &pool{
		conns:       make(chan *idleConn, poolConf.MaxCap),
		new:         poolConf.New,
		maxCap:      poolConf.MaxCap,
		close:       poolConf.Close,
		idleTimeout: poolConf.IdleTimeout,
	}

	if poolConf.MinCap < 0 || poolConf.MaxCap <= 0 || poolConf.MinCap > poolConf.MaxCap {
		return c, errors.New("invalid capacity settings")
	}

	for i := 0; i < poolConf.MinCap; i++ {
		conn, err := c.new()
		if err != nil {
			c.Release()
			return c, fmt.Errorf("new is not able to fill the pool: %s", err)
		}
		c.conns <- &idleConn{conn: conn, t: time.Now()}
	}
	return c, nil
}

//getConns 获取所有连接
func (c *pool) getConnAll() chan *idleConn {
	c.mx.Lock()
	conns := c.conns
	c.mx.Unlock()
	return conns
}

//Get 从pool中取一个连接
func (c *pool) Get() (interface{}, error) {
	connAll := c.getConnAll()
	for {
		select {
		case wrapConn := <-connAll:
			if wrapConn == nil {
				return nil, ErrClosed
			}
			if timeout := c.idleTimeout; timeout > 0 {
				if wrapConn.t.Add(timeout).Before(time.Now()) {
					//close conn
					c.mx.Lock()
					c.active--
					c.mx.Unlock()
					c.Close(wrapConn.conn)
					continue
				}
			}
			c.mx.Lock()
			c.active--
			c.mx.Unlock()
			return wrapConn.conn, nil
		default:
			c.mx.Lock()
			if c.active > c.maxCap {
				c.mx.Unlock()
				time.Sleep(time.Millisecond * 50)
				fmt.Println("连接池已用完,请等待:", time.Now())
				continue
			}
			defer c.mx.Unlock()
			conn, err := c.new()
			if err != nil {
				return conn, err
			}
			c.active++
			return conn, nil
		}
	}
}

//Put 将连接放回pool中
func (c *pool) Put(conn interface{}) error {

	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	if c.conns == nil {
		return c.Close(conn)
	}

	select {
	case c.conns <- &idleConn{conn: conn, t: time.Now()}:
		c.active++
		return nil
	default:
		//连接池已满，直接关闭该链接
		return c.Close(conn)
	}
	return nil
}

//Close 关闭单条连接
func (c *pool) Close(conn interface{}) error {
	c.mx.Lock()
	c.active--
	c.mx.Unlock()
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}
	return c.close(conn)
}

//Release 释放连接池中所有链接
func (c *pool) Release() {
	c.mx.Lock()
	conns := c.conns
	closeFun := c.close
	c.active = 0
	c.mx.Unlock()

	if conns == nil {
		return
	}

	if len(conns) > 0 {
		for wrapConn := range conns {
			closeFun(wrapConn.conn)
		}
	}
}

//获取连接池中可用连接数
func (c *pool) Len() int {
	c.mx.Lock()
	conns := c.conns
	c.mx.Unlock()
	return len(conns)
}
