package core

import (
	"sync"

	myncer_pb "github.com/hansbala/myncer/proto/myncer"
)

// SyncStatusBroadcaster manages subscriptions to sync status updates
type SyncStatusBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[string][]chan *myncer_pb.SyncRun // syncId -> list of channels
}

// NewSyncStatusBroadcaster creates a new broadcaster instance
func NewSyncStatusBroadcaster() *SyncStatusBroadcaster {
	return &SyncStatusBroadcaster{
		subscribers: make(map[string][]chan *myncer_pb.SyncRun),
	}
}

// Subscribe adds a new subscriber for sync status updates for a specific sync ID
func (b *SyncStatusBroadcaster) Subscribe(syncId string) chan *myncer_pb.SyncRun {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan *myncer_pb.SyncRun, 10) // Buffered channel to prevent blocking
	if b.subscribers[syncId] == nil {
		b.subscribers[syncId] = make([]chan *myncer_pb.SyncRun, 0)
	}
	b.subscribers[syncId] = append(b.subscribers[syncId], ch)
	return ch
}

// Unsubscribe removes a subscriber channel
func (b *SyncStatusBroadcaster) Unsubscribe(syncId string, ch chan *myncer_pb.SyncRun) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subscribers := b.subscribers[syncId]
	for i, subscriber := range subscribers {
		if subscriber == ch {
			// Remove the channel from the slice
			b.subscribers[syncId] = append(subscribers[:i], subscribers[i+1:]...)
			close(ch)
			break
		}
	}

	// Clean up empty sync ID entries
	if len(b.subscribers[syncId]) == 0 {
		delete(b.subscribers, syncId)
	}
}

// Broadcast sends a sync run update to all subscribers of that sync ID
func (b *SyncStatusBroadcaster) Broadcast(syncRun *myncer_pb.SyncRun) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	syncId := syncRun.GetSyncId()
	subscribers, ok := b.subscribers[syncId]
	if !ok {
		return // No hay suscriptores para este syncId
	}

	// Iteramos sobre los suscriptores existentes
	for _, ch := range subscribers {
		// Usamos un select para evitar bloqueos si un canal está lleno
		select {
		case ch <- syncRun:
			// El mensaje se envió correctamente
		default:
			// Si el canal está lleno, se omite este suscriptor para no bloquear a los demás.
			// Esto podría suceder si un cliente es muy lento procesando los mensajes.
			Warningf("Skipping broadcast to full channel for sync %s", syncId)
		}
	}
}

// Close closes all subscriber channels and cleans up
func (b *SyncStatusBroadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for syncId, subscribers := range b.subscribers {
		for _, ch := range subscribers {
			close(ch)
		}
		delete(b.subscribers, syncId)
	}
}
