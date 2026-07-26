package discord

import (
	"log"
	"sync"
)

type EventHandler func() error
type Unsubscriber func()

type listenerRecord[T any] struct {
	eventName T
	fn        any
}

type EventEmitter[T comparable] struct {
	events    map[T][]uint64
	listeners map[uint64]listenerRecord[T]
	nextID    uint64
	mu        sync.RWMutex
}

func NewEventEmitter[T comparable]() *EventEmitter[T] {
	return &EventEmitter[T]{
		events:    make(map[T][]uint64),
		listeners: make(map[uint64]listenerRecord[T]),
	}
}

func (e *EventEmitter[T]) On(event T, handler EventHandler) Unsubscriber {
	e.mu.Lock()
	defer e.mu.Unlock()

	id := e.nextID
	e.nextID++

	e.events[event] = append(e.events[event], id)
	e.listeners[id] = listenerRecord[T]{
		eventName: event,
		fn:        handler,
	}

	return func() {
		e.off(id)
	}
}

func (e *EventEmitter[T]) off(id uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, exists := e.listeners[id]
	if !exists {
		return
	}

	ids := e.events[record.eventName]
	for i, currentID := range ids {
		if currentID == id {
			e.events[record.eventName] = append(ids[:i], ids[i+1:]...)
			break
		}
	}

	if len(e.events[record.eventName]) == 0 {
		delete(e.events, record.eventName)
	}
	delete(e.listeners, id)
}

func (e *EventEmitter[T]) Emit(event T) {
	e.mu.RLock()
	ids, exists := e.events[event]
	if !exists {
		e.mu.RUnlock()
		return
	}

	// Copy the handlers to a local slice before executing.
	// This prevents deadlocks if a handler calls On() or .off() inside itself!
	handlers := make([]EventHandler, 0, len(ids))
	for _, id := range ids {
		handlers = append(handlers, e.listeners[id].fn.(EventHandler))
	}
	e.mu.RUnlock()

	// Execute handlers without holding the lock
	for _, handler := range handlers {
		err := handler()
		if err != nil {
			log.Printf("[EventEmitter] Error handling event '%v': %v\n", event, err)
		}
	}
}
