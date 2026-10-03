package service

import (
	"context"
	"net/http"
)

type imageWork struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	users  int      // protected by imageMu
	value  response // published by closing done
}

// startImage coalesces identical requests before reserving queue space. A
// waiting job has no downloaded image or decoded pixels. The job deadline starts
// on admission, so time in both queues counts towards REQUEST_TIMEOUT.
func (g *Gateway) startImage(key string, r *http.Request) *imageWork {
	g.imageMu.Lock()
	defer g.imageMu.Unlock()
	if work := g.images[key]; work != nil {
		work.users++
		return work
	}
	select {
	case g.imageJobs <- struct{}{}:
	default:
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), g.config.RequestTimeout)
	work := &imageWork{ctx: ctx, cancel: cancel, done: make(chan struct{}), users: 1}
	g.images[key] = work
	request := r.Clone(ctx)
	request.Method = http.MethodGet
	go func() {
		defer func() { <-g.imageJobs }()
		if value, ok := g.cache.get(key); ok {
			work.value = value
		} else {
			work.value = g.compute(request, true)
			if work.value.status == http.StatusOK {
				g.cache.put(key, work.value)
			}
		}
		g.imageMu.Lock()
		if g.images[key] == work {
			delete(g.images, key)
		}
		close(work.done)
		g.imageMu.Unlock()
	}()
	return work
}

func (g *Gateway) leaveImage(key string, work *imageWork) {
	g.imageMu.Lock()
	defer g.imageMu.Unlock()
	work.users--
	if work.users == 0 {
		work.cancel()
		// A future client must not join already-cancelled work. Running native
		// code still holds its admission slot and pixel reservation until done.
		if g.images[key] == work {
			delete(g.images, key)
		}
	}
}
