package eventbus

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Event struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

type HandlerFunc func(event Event)

var (
	global     *Bus
	globalOnce sync.Once
)

func Global() *Bus {
	globalOnce.Do(func() { global = New() })
	return global
}

type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]HandlerFunc
}

func New() *Bus {
	return &Bus{handlers: make(map[string][]HandlerFunc)}
}

func (b *Bus) Subscribe(eventType string, handler HandlerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

func (b *Bus) Publish(event Event) {
	b.mu.RLock()
	handlers := b.handlers[event.Type]
	b.mu.RUnlock()
	for _, h := range handlers {
		go func(fn HandlerFunc) {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("eventbus: handler panic: %v", r)
				}
			}()
			fn(event)
		}(h)
	}
}
