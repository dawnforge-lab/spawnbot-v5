package autonomy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>Article One</title>
      <link>https://example.com/1</link>
      <guid>guid-1</guid>
      <description>First article</description>
    </item>
    <item>
      <title>Article Two</title>
      <link>https://example.com/2</link>
      <guid>guid-2</guid>
      <description>Second article</description>
    </item>
  </channel>
</rss>`

func TestFeedPoller_DetectsNewItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testRSSFeed))
	}))
	defer server.Close()

	var received []FeedItem
	poller := NewFeedPoller(
		[]FeedConfig{{URL: server.URL, CheckIntervalMinutes: 1}},
		func(items []FeedItem, cfg FeedConfig) { received = append(received, items...) },
	)

	err := poller.PollOnce(context.Background())
	require.NoError(t, err)
	assert.Len(t, received, 2)
	assert.Equal(t, "Article One", received[0].Title)
}

func TestFeedPoller_SkipsSeenItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testRSSFeed))
	}))
	defer server.Close()

	var received []FeedItem
	poller := NewFeedPoller(
		[]FeedConfig{{URL: server.URL, CheckIntervalMinutes: 1}},
		func(items []FeedItem, cfg FeedConfig) { received = append(received, items...) },
	)

	poller.PollOnce(context.Background())
	assert.Len(t, received, 2)

	// Second poll — same feed, no new items
	received = nil
	poller.PollOnce(context.Background())
	assert.Len(t, received, 0)
}

func TestFeedPoller_EmptyFeedList(t *testing.T) {
	poller := NewFeedPoller(nil, func(items []FeedItem, cfg FeedConfig) {})
	err := poller.PollOnce(context.Background())
	assert.NoError(t, err)
}
