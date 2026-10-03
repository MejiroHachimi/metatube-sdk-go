package service

import (
	"context"
	"net/http"
)

// Authorization is part of the key: a successful authenticated response must
// never be shared with a request presenting different credentials.
type metadataKey struct {
	url           string
	authorization [32]byte
}

type metadataWork struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	users  int      // protected by metadataMu
	value  response // published by closing done
}

// Only overlapping requests share work. Completed responses, including errors,
// are never retained as a metadata cache. Queue space is reserved before a new
// goroutine is started, and queueing counts towards the shared job deadline.
func (g *Gateway) startMetadata(key metadataKey, r *http.Request) *metadataWork {
	g.metadataMu.Lock()
	defer g.metadataMu.Unlock()
	if work := g.metadata[key]; work != nil && work.ctx.Err() == nil {
		work.users++
		return work
	}
	select {
	case g.metadataJobs <- struct{}{}:
	default:
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), g.config.RequestTimeout)
	work := &metadataWork{ctx: ctx, cancel: cancel, done: make(chan struct{}), users: 1}
	g.metadata[key] = work
	request := r.Clone(ctx)
	request.Method = http.MethodGet
	go func() {
		work.value = g.compute(request, false)
		g.metadataMu.Lock()
		if g.metadata[key] == work {
			delete(g.metadata, key)
		}
		// compute holds its SDK slot until the work really stops, even after a
		// timeout or disconnect. Release admission only after it has returned.
		<-g.metadataJobs
		close(work.done)
		g.metadataMu.Unlock()
	}()
	return work
}

func (g *Gateway) leaveMetadata(key metadataKey, work *metadataWork) {
	g.metadataMu.Lock()
	defer g.metadataMu.Unlock()
	work.users--
	if work.users == 0 {
		work.cancel()
		if g.metadata[key] == work {
			delete(g.metadata, key)
		}
	}
}
