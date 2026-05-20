package camoufox

import (
	"context"
	"errors"
	"sync"
)

var ErrPoolClosed = errors.New("browser pool is closed")

type Pool struct {
	size int
	opts *LaunchOptions

	mu      sync.Mutex
	idle    chan *Browser
	slots   chan struct{}
	closeCh chan struct{}
	all     map[*Browser]struct{}
	closed  bool
	launch  func(context.Context, *LaunchOptions) (*Browser, error)
	closeFn func(*Browser) error
}

func NewPool(ctx context.Context, size int, opts *LaunchOptions) (*Pool, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if size <= 0 {
		return nil, errors.New("pool size must be greater than zero")
	}
	return &Pool{
		size:    size,
		opts:    opts,
		idle:    make(chan *Browser, size),
		slots:   make(chan struct{}, size),
		closeCh: make(chan struct{}),
		all:     map[*Browser]struct{}{},
		launch: func(ctx context.Context, opts *LaunchOptions) (*Browser, error) {
			return New(ctx, opts)
		},
		closeFn: func(browser *Browser) error {
			return browser.Close(context.Background())
		},
	}, nil
}

func (p *Pool) Acquire(ctx context.Context) (*Browser, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, nil, ErrPoolClosed
	}
	p.mu.Unlock()

	select {
	case browser := <-p.idle:
		return p.checkoutIdle(browser)
	default:
	}

	select {
	case browser := <-p.idle:
		return p.checkoutIdle(browser)
	case p.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-p.closeCh:
		return nil, nil, ErrPoolClosed
	}

	browser, err := p.launch(ctx, p.opts)
	if err != nil {
		<-p.slots
		return nil, nil, err
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		<-p.slots
		_ = p.closeFn(browser)
		return nil, nil, ErrPoolClosed
	}
	p.all[browser] = struct{}{}
	p.mu.Unlock()
	return browser, p.releaseOnce(browser), nil
}

func (p *Pool) checkoutIdle(browser *Browser) (*Browser, func(), error) {
	p.mu.Lock()
	if p.closed {
		if _, ok := p.all[browser]; !ok {
			p.mu.Unlock()
			return nil, nil, ErrPoolClosed
		}
		delete(p.all, browser)
		p.mu.Unlock()
		_ = p.closeFn(browser)
		<-p.slots
		return nil, nil, ErrPoolClosed
	}
	p.mu.Unlock()
	return browser, p.releaseOnce(browser), nil
}

func (p *Pool) releaseOnce(browser *Browser) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			var closeNow bool
			p.mu.Lock()
			if _, ok := p.all[browser]; !ok {
				p.mu.Unlock()
				return
			}
			if p.closed {
				delete(p.all, browser)
				closeNow = true
			} else {
				select {
				case p.idle <- browser:
				default:
					delete(p.all, browser)
					closeNow = true
				}
			}
			p.mu.Unlock()
			if closeNow {
				_ = p.closeFn(browser)
				<-p.slots
			}
		})
	}
}

func (p *Pool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	close(p.closeCh)
	toClose := make([]*Browser, 0, len(p.all))
	for browser := range p.all {
		toClose = append(toClose, browser)
		delete(p.all, browser)
	}
	p.mu.Unlock()

	var closeErr error
	for _, browser := range toClose {
		if err := p.closeFn(browser); closeErr == nil {
			closeErr = err
		}
		<-p.slots
	}
	return closeErr
}
