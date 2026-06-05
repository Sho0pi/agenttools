// Package askim provides an async instant-messaging provider for the ask_user
// tool. It decouples the outbound send (Telegram, WhatsApp, Slack, etc.) from
// the inbound reply, correlating them by a conversation ID chosen by the caller
// (chat ID, phone number, user ID — whatever uniquely identifies the thread).
//
// Usage:
//
//	p := askim.New(func(ctx context.Context, q ask.Question) error {
//	    return bot.Send(chatID, formatQuestion(q))
//	})
//	tool, _ := ask.New(p.Ask(strconv.FormatInt(chatID, 10)))
//
//	// in your webhook handler:
//	p.Reply(strconv.FormatInt(update.Message.Chat.ID, 10), update.Message.Text)
package askim

import (
	"context"
	"sync"

	"github.com/sho0pi/agenttools/ask"
)

// Sender delivers a question to the user via any messaging channel.
// It must not block for the user's reply — just fire the outbound message.
type Sender func(ctx context.Context, q ask.Question) error

// Provider correlates outbound questions with inbound webhook replies.
// One active ask per conversation ID at a time; concurrent asks on different
// IDs are fully independent and race-safe.
type Provider struct {
	send    Sender
	mu      sync.Mutex
	pending map[string]chan string
}

// New returns a Provider backed by send. send must not be nil.
func New(send Sender) *Provider {
	return &Provider{send: send, pending: make(map[string]chan string)}
}

// Ask returns an ask.Ask bound to conversationID. The returned func sends the
// question via Sender and then blocks until Reply is called with the same ID,
// or ctx is cancelled. On cancellation or Sender error, q.Default is returned.
func (p *Provider) Ask(conversationID string) ask.Ask {
	return func(ctx context.Context, q ask.Question) (string, error) {
		ch := make(chan string, 1)

		p.mu.Lock()
		p.pending[conversationID] = ch
		p.mu.Unlock()

		defer func() {
			p.mu.Lock()
			delete(p.pending, conversationID)
			p.mu.Unlock()
		}()

		if err := p.send(ctx, q); err != nil {
			return q.Default, err
		}

		select {
		case <-ctx.Done():
			return q.Default, ctx.Err()
		case text := <-ch:
			if text == "" {
				return q.Default, nil
			}
			return text, nil
		}
	}
}

// Reply routes an incoming message to the Ask call waiting on conversationID.
// No-op if no ask is pending for that ID (e.g. user replies late).
func (p *Provider) Reply(conversationID, text string) {
	p.mu.Lock()
	ch := p.pending[conversationID]
	p.mu.Unlock()

	if ch == nil {
		return
	}
	select {
	case ch <- text:
	default:
	}
}
