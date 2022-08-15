package middleware

import (
	"context"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

type Chain struct {
	logger log.Logger

	pre  *node
	post *node
}

func New(client dapr.Client, logger log.Logger, pre, post []string) *Chain {
	chain := &Chain{
		logger: log.With(logger, "Module", "chain"),
		pre:    new(node),
		post:   new(node),
	}

	chain.pre = chain.newChain(client, pre)
	chain.post = chain.newChain(client, post)
	return chain
}

func (c *Chain) newChain(client dapr.Client, types []string) *node {
	root := new(node)
	p := root
	for _, _t := range types {
		node := new(node)
		node.actor = NewActor(client, _t)
		p.next = node
		p = p.next
	}

	return root.next
}

func (c *Chain) Pre(ctx context.Context, req *DoReq) error {
	return c.do(ctx, req, c.pre)
}
func (c *Chain) Post(ctx context.Context, req *DoReq) error {
	return c.do(ctx, req, c.post)
}

func (c *Chain) do(ctx context.Context, req *DoReq, p *node) error {
	for p != nil {
		err := p.actor.Do(ctx, req)
		if err != nil {
			level.Error(c.logger).Log("message", err)
		}
		p = p.next
	}

	return nil
}

type node struct {
	actor *ClientStub
	next  *node
}
