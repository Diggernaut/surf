package jar

import (
	"net/http"
	"sync"

	"github.com/diggernaut/goquery"
)

// State represents a point in time.
type State struct {
	Request  *http.Request
	Response *http.Response
	Dom      *goquery.Document
}

// NewHistoryState creates and returns a new *State type.
func NewHistoryState(req *http.Request, resp *http.Response, dom *goquery.Document) *State {
	return &State{
		Request:  req,
		Response: resp,
		Dom:      dom,
	}
}

// History is a type that records browser state.
type History interface {
	Len() int
	SetCapacity(int)
	Push(p *State) int
	Pop() *State
	Top() *State
}

// Node holds stack values and points to the next element.
// type Node struct {
//	Value *State
//	Next  *Node
// }

// MemoryHistory is an in-memory implementation of the History interface.
type MemoryHistory struct {
	sync.Mutex
	states   []*State
	Capacity int
}

// NewMemoryHistory creates and returns a new *StateHistory type.
func NewMemoryHistory() *MemoryHistory {
	return &MemoryHistory{}
}

// Len returns the number of states in the history.
func (his *MemoryHistory) Len() int {
	his.Lock()
	defer his.Unlock()
	return len(his.states)
}

// Len returns the number of states in the history.
func (his *MemoryHistory) SetCapacity(capacity int) {
	his.Lock()
	defer his.Unlock()
	his.Capacity = capacity
}

// Push adds a new State at the front of the history.
func (his *MemoryHistory) Push(p *State) int {
	his.Lock()
	defer his.Unlock()
	if his.Capacity > 0 {
		his.states = append(his.states, p)
		if len(his.states) > his.Capacity {
			his.states = his.states[1:]
		}
	}
	return len(his.states)
}

// Pop removes and returns the State at the front of the history.
func (his *MemoryHistory) Pop() *State {
	his.Lock()
	defer his.Unlock()
	if len(his.states) > 0 {
		value := his.states[len(his.states)-1]
		if len(his.states) > 1 {
			his.states[len(his.states)-1] = nil
			his.states = his.states[0 : len(his.states)-1]
		}
		return value
	}

	return nil
}

// Top returns the State at the front of the history without removing it.
func (his *MemoryHistory) Top() *State {
	his.Lock()
	defer his.Unlock()
	if len(his.states) == 0 {
		return nil
	}
	return his.states[len(his.states)-1]
}
