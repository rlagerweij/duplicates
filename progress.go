package main

import (
	"fmt"
	"sync/atomic"
)

type Progress struct {
	display  *bool
	pattern  string
	previous string
	count    int64
}

func (pg *Progress) delete() {
	if *pg.display {
		for j := 0; j <= len(pg.previous); j++ {
			fmt.Print("\b")
		}
	}
}

func (pg *Progress) displayToConsole() {
	if *pg.display {
		pg.previous = fmt.Sprintf(pg.pattern, pg.count)
		fmt.Print(pg.previous)
	}
}

func (pg *Progress) increment() {
	atomic.AddInt64(&pg.count, 1)
	if *pg.display {
		pg.delete()
		pg.displayToConsole()
	}
}

func creatProgress(pattern string, display *bool) (pg *Progress) {
	pg = &Progress{
		display:  display,
		pattern:  pattern,
		previous: "",
		count:    0,
	}
	return pg
}
