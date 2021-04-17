package main

import (
	"fmt"
	"sync"
	"time"
)

type ItemCache struct {
	numStories      int
	cache           []item
	expiration      time.Time
	duration        time.Duration
	refreshSubtract time.Duration
	mutex           sync.Mutex
}

func (c *ItemCache) items() ([]item, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	now := time.Now()
	if now.Sub(c.expiration) < 0 {
		return c.cache, nil
	}
	stories, err := getTopStories(c.numStories)
	if err != nil {
		return nil, err
	}
	c.expiration = now.Add(c.duration)
	c.cache = stories
	return c.cache, nil
}

func (c *ItemCache) Refresh() {
	fmt.Println("refresh every", c.duration-c.refreshSubtract)
	ticker := time.NewTicker(c.duration - c.refreshSubtract)
	for {
		tmp := ItemCache{
			numStories: c.numStories,
			duration:   c.duration,
		}
		_, _ = tmp.items()
		c.mutex.Lock()
		fmt.Println("update cache from ticker")
		c.cache = tmp.cache
		c.expiration = time.Now().Add(c.duration)
		c.mutex.Unlock()
		<-ticker.C
	}
}
