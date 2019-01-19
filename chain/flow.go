package chain

/*
	Chain flow is list of event listeners organized by type for Telegram
	Author: Daniil Furmanov
	License: MIT
*/

import (
	"github.com/pkg/errors"
	tb "gopkg.in/tucnak/telebot.v2"
	"sync"
)

type FlowCallback func(e *Node, c *tb.Message) *Node

/*
	A flow is chain or double-linked list of events organized by type
*/
type Flow struct {
	flowId         string
	root           *Node
	bot            *tb.Bot
	defaultLocale  string
	positions      map[string]*Node
	defaultHandler FlowCallback
	mx             sync.RWMutex
}

var ErrChainIsEmpty = errors.New("chain has zero handlers")

/*
	Creates a new chain flow
*/
func NewFlow(flowId string, bot *tb.Bot) (*Flow, error) {
	f := &Flow{
		bot:            bot,
		positions:      make(map[string]*Node),
		defaultHandler: nil,
		mx:             sync.RWMutex{},
	}
	f.root = &Node{id: flowId, flow: f, endpoint: nil, prev: nil, next: nil}
	return f, nil
}

/*
	Get flow's unique identificator
*/
func (f *Flow) GetFlowId() string {
	return f.flowId
}

/*
	Get attached Telegram bot
*/
func (f *Flow) GetBot() *tb.Bot {
	return f.bot
}

/*
	Get the root node
*/
func (f *Flow) GetRoot() *Node {
	return f.root
}

/*
	Gets the user position in the flow
*/
func (f *Flow) GetPosition(of tb.Recipient) (*Node, bool) {
	f.mx.RLock()
	node, ok := f.positions[of.Recipient()]
	f.mx.RUnlock()
	return node, ok
}

/*
	Sets the user current position in the flow
*/
func (f *Flow) SetPosition(of tb.Recipient, node *Node) {
	f.mx.Lock()
	f.positions[of.Recipient()] = node
	f.mx.Unlock()
}

/*
	Deletes the user current position in the flow
*/
func (f *Flow) DeletePosition(of tb.Recipient) {
	f.mx.Lock()
	delete(f.positions, of.Recipient())
	f.mx.Unlock()
}

/*
	Search for a node with ID
*/
func (f *Flow) Search(nodeId string) (*Node, bool) {
	return f.root.SearchDown(nodeId)
}

/*
	Get the root node
*/
func (f *Flow) DefaultHandler(endpoint FlowCallback) *Flow {
	f.defaultHandler = endpoint
	return f
}

func (f *Flow) Start(to tb.Recipient, text string, options ...interface{}) (err error) {
	if f.root.next == nil {
		return ErrChainIsEmpty
	}
	if len(options) > 0 {
		// a workaround for nil options
		// otherwise the message will not be sent
		_, err = f.GetBot().Send(to, text, options)
	} else {
		_, err = f.GetBot().Send(to, text)
	}
	if err == nil {
		f.mx.Lock()
		f.positions[to.Recipient()] = f.root.next
		f.mx.Unlock()
	}
	return
}

/*
	Process with the next flow iteration
	Returns true only if the iteration was successful
*/
func (f *Flow) Process(m *tb.Message) bool {
	if m == nil {
		return false
	}
	sender := m.Sender
	node, ok := f.GetPosition(sender)
	if !ok {
		// the flow hasn't started for the user
		return false
	}
	if node == nil {
		f.DeletePosition(sender)
		return false
	}
	if !node.CheckEvent(m) || node.endpoint == nil {
		// input is invalid for the particular node
		if f.defaultHandler != nil {
			next := f.defaultHandler(node, m)
			if next != node {
				f.SetPosition(sender, next)
			}
			return true
		}
		return false
	}
	next := node.endpoint(node, m)
	if next != node {
		f.SetPosition(sender, next)
	}
	return true
}
