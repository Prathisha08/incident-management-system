package ingestion

import (
	"errors"
	"sync/atomic"

	"github.com/aravindhsrbk/ims/internal/models"
)

var ErrBufferFull = errors.New("signal buffer full: apply backpressure")

// SignalBuffer is a non-blocking in-memory channel buffer.
// It decouples HTTP ingestion from persistence so a slow DB never stalls the API.
type SignalBuffer struct {
	ch      chan *models.Signal
	dropped atomic.Int64
}

func NewSignalBuffer(capacity int) *SignalBuffer {
	return &SignalBuffer{ch: make(chan *models.Signal, capacity)}
}

// Push enqueues a signal without blocking. Returns ErrBufferFull when at capacity.
func (b *SignalBuffer) Push(signal *models.Signal) error {
	select {
	case b.ch <- signal:
		return nil
	default:
		b.dropped.Add(1)
		return ErrBufferFull
	}
}

func (b *SignalBuffer) Chan() <-chan *models.Signal { return b.ch }
func (b *SignalBuffer) Len() int                    { return len(b.ch) }
func (b *SignalBuffer) Cap() int                    { return cap(b.ch) }
func (b *SignalBuffer) Dropped() int64              { return b.dropped.Load() }
