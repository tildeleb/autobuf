// Copyright (C) 2013 Randall McPherson
// Copyright (C) 2015 Lawrence E Bakst

// gentle rework of nice buffering code from s3gof3r

package autobuf

import (
	"container/list"
	"fmt"
	"time"
)

type Pool struct {
	Take    chan interface{} // take, client takes buffer
	Give    chan interface{} // give, client gives buffer back
	Size    chan int64       // change size of buffers
	Quit    chan struct{}
	Exit    chan struct{}
	Cnt     int
	timeout time.Duration // free unused buffers after timeout
	size    int64         // size of buffers
	acap    int64         // cap
}

type qb struct {
	when time.Time
	b    interface{}
}

func New(alloc func(size, acap int64) interface{}, size, acap int64, timeout time.Duration) (as *Pool) {
	as = &Pool{Take: make(chan interface{}), Give: make(chan interface{}),
		Size: make(chan int64), Quit: make(chan struct{}), Exit: make(chan struct{}), size: size, acap: acap}
	//		timeout: timeout) // size: size, acap: acap}
	// start buffer manager
	go func() {
		q := new(list.List)
		for {
			if q.Len() == 0 {
				b := alloc(as.size, as.acap)
				//fmt.Printf("Make: len=%d, cap=%d, cnt=%d, qlen=%d\n", len(s), cap(s), as.Cnt, q.Len())
				q.PushFront(qb{when: time.Now(), b: b})
				as.Cnt++
			}
			e := q.Front()
			timeout := time.NewTimer(as.timeout)
			select {
			case b := <-as.Give:
				timeout.Stop()
				//fmt.Printf("Give: len=%d, cap=%d, cnt=%d, qlen=%d\n", len(b), cap(b), as.Cnt, q.Len())
				q.PushFront(qb{when: time.Now(), b: b})
			case as.Take <- e.Value.(qb).b:
				//fmt.Printf("Take: cnt=%d, qlen=%d\n", as.Cnt, q.Len())
				timeout.Stop()
				q.Remove(e)
			case <-timeout.C:
				// free unused slices older than timeout
				e := q.Front()
				for e != nil {
					n := e.Next()
					if time.Since(e.Value.(qb).when) > as.timeout {
						q.Remove(e)
						e.Value = nil
					}
					e = n
				}
			case sz := <-as.Size: // update buffer size, free buffers
				as.size = sz
			case <-as.Quit:
				fmt.Printf("autobuf: Cnt=%d\n", as.Cnt)
				as.Exit <- struct{}{}
				return
			}
		}
	}()
	return as
}
