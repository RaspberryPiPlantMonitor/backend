package main

import (
	"container/list"
	"fmt"
)

// T is for key, value types
type T interface{}

// Map uses map to hold cache items
type Map map[T]T

// FIFO cache
type FIFO struct {
	cache Map
	q     *list.List
	size  int
}

// New returns new FIFO cache of the given size
func NewFIFO(size int) *FIFO {
	return &FIFO{
		cache: make(Map, size),
		q:     list.New(),
		size:  size,
	}

}

// Set sets the given key,value pair in the cache.
func (c *FIFO) Set(key, val T) {
	// defer c.dump()
	// if it already exists
	if val, ok := c.cache[key]; ok {
		c.cache[key] = val
		return
	}

	// when cache is not full
	if len(c.cache) < c.size {
		c.cache[key] = val
		c.q.PushBack(val)
		return

	}
	e := c.q.Front()
	delete(c.cache, e.Value)
	c.q.Remove(e)
	c.cache[key] = val
	c.q.PushBack(val)
	return
}

// Get gets a value from the cache
func (c *FIFO) Get(key T) (T, bool) {
	// defer c.dump()
	val, ok := c.cache[key]
	return val, ok
}

func (c *FIFO) Avg() float64 {
	var sum float64 = 0.0
	e := c.q.Front()
	for i := 0; i < c.size; i++ {
		if e != nil {
			sum += e.Value.(float64)
			e = e.Next()
		}
	}
	return sum / float64(c.size)
}

// dump dumps cache content for debugging
func (c *FIFO) dump() {
	e := c.q.Front()
	fmt.Printf("|")
	for i := 0; i < c.size; i++ {
		var val T
		val = " "
		if e != nil {
			val = e.Value
			e = e.Next()
		}
		fmt.Printf("  %v  |", val)
	}
	fmt.Println()
}
