package item

import (
	"fmt"
	"sync"
)

type Status int

const (
	Unactivate Status = iota
	Activate
)

type Item struct {
	mu       sync.RWMutex
	Code     string         `json:"code"`
	Name     string         `json:"name"`
	Metadata map[string]any `json:"metadata"`
	status   Status
}

func (i *Item) String() string {
	return fmt.Sprintf("[%s,%s]", i.Code, i.Name)
}

func (i *Item) UpdateStatus(status Status) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.status = status
}

func (i *Item) Status() Status {
	var status Status
	i.mu.RLock()
	defer i.mu.RUnlock()
	status = i.status
	return status
}
