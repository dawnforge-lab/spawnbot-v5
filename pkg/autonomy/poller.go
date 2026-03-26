package autonomy

import (
	"context"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
)

// FeedCallback is called with new items and the originating feed config.
type FeedCallback func(items []FeedItem, cfg FeedConfig)

// FeedPoller fetches RSS/Atom feeds on a schedule and invokes the callback
// only for items not previously seen. The poller activates the agent rather
// than the agent doing the polling, which conserves tokens.
type FeedPoller struct {
	feeds     []FeedConfig
	callback  FeedCallback
	seen      map[string]bool // GUID or link → already processed
	mu        sync.Mutex
	lastPoll  map[string]time.Time // URL → last fetch time
	cancelFn  context.CancelFunc
}

// NewFeedPoller creates a new FeedPoller. feeds may be nil or empty.
func NewFeedPoller(feeds []FeedConfig, callback FeedCallback) *FeedPoller {
	return &FeedPoller{
		feeds:    feeds,
		callback: callback,
		seen:     make(map[string]bool),
		lastPoll: make(map[string]time.Time),
	}
}

// PollOnce fetches all configured feeds synchronously and invokes the callback
// for any items not previously seen.
func (p *FeedPoller) PollOnce(ctx context.Context) error {
	parser := gofeed.NewParser()
	for _, cfg := range p.feeds {
		if err := p.pollFeed(ctx, parser, cfg); err != nil {
			return err
		}
	}
	return nil
}

// pollFeed fetches a single feed and calls the callback for new items.
func (p *FeedPoller) pollFeed(ctx context.Context, parser *gofeed.Parser, cfg FeedConfig) error {
	feed, err := parser.ParseURLWithContext(cfg.URL, ctx)
	if err != nil {
		return err
	}

	p.mu.Lock()
	var newItems []FeedItem
	for _, item := range feed.Items {
		key := item.GUID
		if key == "" {
			key = item.Link
		}
		if key == "" {
			continue
		}
		if !p.seen[key] {
			p.seen[key] = true
			fi := FeedItem{
				Title:       item.Title,
				Link:        item.Link,
				Description: item.Description,
			}
			if item.PublishedParsed != nil {
				fi.Published = item.PublishedParsed.Format(time.RFC3339)
			} else if item.Published != "" {
				fi.Published = item.Published
			}
			newItems = append(newItems, fi)
		}
	}
	p.lastPoll[cfg.URL] = time.Now()
	p.mu.Unlock()

	if len(newItems) > 0 {
		p.callback(newItems, cfg)
	}
	return nil
}

// Start launches a background goroutine that checks all feeds every minute,
// only actually fetching a feed when its CheckIntervalMinutes has elapsed.
// It stops when ctx is cancelled.
func (p *FeedPoller) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	p.mu.Lock()
	p.cancelFn = cancel
	p.mu.Unlock()

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				parser := gofeed.NewParser()
				p.mu.Lock()
				due := make([]FeedConfig, 0, len(p.feeds))
				for _, cfg := range p.feeds {
					interval := time.Duration(cfg.CheckIntervalMinutes) * time.Minute
					if interval <= 0 {
						interval = time.Minute
					}
					last, ok := p.lastPoll[cfg.URL]
					if !ok || now.Sub(last) >= interval {
						due = append(due, cfg)
					}
				}
				p.mu.Unlock()

				for _, cfg := range due {
					// Context cancellation check between feeds.
					select {
					case <-ctx.Done():
						return
					default:
					}
					p.pollFeed(ctx, parser, cfg) //nolint:errcheck
				}
			}
		}
	}()
}

// Stop cancels the background goroutine started by Start.
func (p *FeedPoller) Stop() {
	p.mu.Lock()
	fn := p.cancelFn
	p.mu.Unlock()
	if fn != nil {
		fn()
	}
}
