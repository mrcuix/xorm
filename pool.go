// Copyright 2013 The XORM Authors. All rights reserved.
// Use of this source code is governed by a BSD
// license that can be found in the LICENSE file.

// Package xorm provides is a simple and powerful ORM for Go. It makes your
// database operation simple.

// This file contains a connection pool interafce and two implements. One is
// NoneConnectionPool is for direct connecting, another is a simple connection
// pool by lock. Attention, the driver may has provided connection pool itself.
// So the default pool is NoneConnectionPool.
package xorm

import (
	"database/sql"
	//"fmt"
	"sync"
	//"sync/atomic"
	"time"
)

// Interface IConnecPool is a connection pool interface, all implements should implement
// Init, RetrieveDB, ReleaseDB and Close methods.
// Init for init when engine be created or invoke SetPool
// RetrieveDB for requesting a connection to db;
// ReleaseDB for releasing a db connection;
// Close for invoking when engine.Close
type IConnectPool interface {
	Init(engine *Engine) error
	RetrieveDB(engine *Engine) (*sql.DB, error)
	ReleaseDB(engine *Engine, db *sql.DB)
	Close(engine *Engine) error
	SetMaxIdleConns(conns int)
	MaxIdleConns() int
}

// Struct NoneConnectPool is a implement for IConnectPool. It provides directly invoke driver's
// open and release connection function
type NoneConnectPool struct {
}

// NewNoneConnectPool new a NoneConnectPool.
func NewNoneConnectPool() IConnectPool {
	return &NoneConnectPool{}
}

// Init do nothing
func (p *NoneConnectPool) Init(engine *Engine) error {
	return nil
}

// RetrieveDB directly open a connection
func (p *NoneConnectPool) RetrieveDB(engine *Engine) (db *sql.DB, err error) {
	db, err = engine.OpenDB()
	return
}

// ReleaseDB directly close a connection
func (p *NoneConnectPool) ReleaseDB(engine *Engine, db *sql.DB) {
	db.Close()
}

// Close do nothing
func (p *NoneConnectPool) Close(engine *Engine) error {
	return nil
}

func (p *NoneConnectPool) SetMaxIdleConns(conns int) {
}

func (p *NoneConnectPool) MaxIdleConns() int {
	return 0
}

// Struct SysConnectPool is a simple wrapper for using system default connection pool.
// About the system connection pool, you can review the code database/sql/sql.go
// It's currently default Pool implments.
type SysConnectPool struct {
	db           *sql.DB
	maxIdleConns int
}

// NewSysConnectPool new a SysConnectPool.
func NewSysConnectPool() IConnectPool {
	return &SysConnectPool{}
}

// Init create a db immediately and keep it util engine closed.
func (s *SysConnectPool) Init(engine *Engine) error {
	db, err := engine.OpenDB()
	if err != nil {
		return err
	}
	s.db = db
	s.maxIdleConns = 2
	return nil
}

// RetrieveDB just return the only db
func (p *SysConnectPool) RetrieveDB(engine *Engine) (db *sql.DB, err error) {
	return p.db, nil
}

// ReleaseDB do nothing
func (p *SysConnectPool) ReleaseDB(engine *Engine, db *sql.DB) {
}

// Close closed the only db
func (p *SysConnectPool) Close(engine *Engine) error {
	return p.db.Close()
}

func (p *SysConnectPool) SetMaxIdleConns(conns int) {
	p.db.SetMaxIdleConns(conns)
	p.maxIdleConns = conns
}

func (p *SysConnectPool) MaxIdleConns() int {
	return p.maxIdleConns
}

// NewSimpleConnectPool new a SimpleConnectPool
func NewSimpleConnectPool() IConnectPool {
	return &SimpleConnectPool{releasedConnects: make([]*sql.DB, 10),
		usingConnects:  map[*sql.DB]time.Time{},
		cur:            -1,
		maxWaitTimeOut: 14400,
		maxIdleConns:   10,
		mutex:          &sync.Mutex{},
	}
}

// Struct SimpleConnectPool is a simple implementation for IConnectPool.
// It's a custom connection pool and not use system connection pool.
// Opening or Closing a database connection must be enter a lock.
// This implements will be improved in furture.
type SimpleConnectPool struct {
	releasedConnects []*sql.DB
	cur              int
	usingConnects    map[*sql.DB]time.Time
	maxWaitTimeOut   int
	mutex            *sync.Mutex
	maxIdleConns     int
}

func (s *SimpleConnectPool) Init(engine *Engine) error {
	return nil
}

// RetrieveDB get a connection from connection pool
func (p *SimpleConnectPool) RetrieveDB(engine *Engine) (*sql.DB, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	var db *sql.DB = nil
	var err error = nil
	//fmt.Printf("%x, rbegin - released:%v, using:%v\n", &p, p.cur+1, len(p.usingConnects))
	if p.cur < 0 {
		db, err = engine.OpenDB()
		if err != nil {
			return nil, err
		}
		p.usingConnects[db] = time.Now()
	} else {
		db = p.releasedConnects[p.cur]
		p.usingConnects[db] = time.Now()
		p.releasedConnects[p.cur] = nil
		p.cur = p.cur - 1
	}

	//fmt.Printf("%x, rend - released:%v, using:%v\n", &p, p.cur+1, len(p.usingConnects))
	return db, nil
}

// ReleaseDB release a db from connection pool
func (p *SimpleConnectPool) ReleaseDB(engine *Engine, db *sql.DB) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	//fmt.Printf("%x, lbegin - released:%v, using:%v\n", &p, p.cur+1, len(p.usingConnects))
	if p.cur >= p.maxIdleConns-1 {
		db.Close()
	} else {
		p.cur = p.cur + 1
		p.releasedConnects[p.cur] = db
	}
	delete(p.usingConnects, db)
	//fmt.Printf("%x, lend - released:%v, using:%v\n", &p, p.cur+1, len(p.usingConnects))
}

// Close release all db
func (p *SimpleConnectPool) Close(engine *Engine) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for len(p.releasedConnects) > 0 {
		p.releasedConnects[0].Close()
		p.releasedConnects = p.releasedConnects[1:]
	}

	return nil
}

func (p *SimpleConnectPool) SetMaxIdleConns(conns int) {
	p.maxIdleConns = conns
}

func (p *SimpleConnectPool) MaxIdleConns() int {
	return p.maxIdleConns
}
