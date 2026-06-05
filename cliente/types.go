package main

import (
	"sync"
	"sync/atomic"
	"time"
)

type IssueType string

const (
	MECH     IssueType = "Mecánica"
	ELECTRIC IssueType = "Eléctrica"
	BODY     IssueType = "Carrocería"
)

const (
	DOCPHASE      = 1
	REPAIRPHASE   = 2
	CLEANPHASE    = 3
	DELIVERYPHASE = 4
)

type Car struct {
	id       int
	issue    IssueType
	duration time.Duration
	curphase int
	start    time.Time
}

type Event struct {
	elapsed time.Duration
	car     int
	phase   int
	status  string
	issue   IssueType
}

type Garage struct {
	mu        sync.RWMutex
	cars      map[int]*Car
	sts       atomic.Int32
	freeSlots chan struct{}
	wg        sync.WaitGroup
}
