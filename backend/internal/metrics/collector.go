package metrics

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

// Collector tracks signal throughput with lock-free atomic counters.
type Collector struct {
	received  atomic.Int64
	processed atomic.Int64
	dropped   atomic.Int64
	logger    *log.Logger
}

func NewCollector(logger *log.Logger) *Collector {
	return &Collector{logger: logger}
}

func (c *Collector) RecordReceived()  { c.received.Add(1) }
func (c *Collector) RecordProcessed() { c.processed.Add(1) }
func (c *Collector) RecordDropped()   { c.dropped.Add(1) }

func (c *Collector) Snapshot() map[string]int64 {
	return map[string]int64{
		"received":  c.received.Load(),
		"processed": c.processed.Load(),
		"dropped":   c.dropped.Load(),
	}
}

// Run prints throughput metrics every 5 seconds until ctx is cancelled.
func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var prevReceived, prevProcessed int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r := c.received.Load()
			p := c.processed.Load()
			d := c.dropped.Load()
			rps := float64(r-prevReceived) / 5.0
			pps := float64(p-prevProcessed) / 5.0
			prevReceived, prevProcessed = r, p

			c.logger.Printf(
				fmt.Sprintf("[METRICS] ingested=%.0f/s  processed=%.0f/s  total_received=%d  total_processed=%d  dropped=%d  buffer_lag=%d",
					rps, pps, r, p, d, r-p,
				),
			)
		}
	}
}
