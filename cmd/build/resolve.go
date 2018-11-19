package main

// Graph of Nodes that resolves dependencies.
type Graph struct {
	Nodes      []Node
	unresolved *NodeMap
	resolved   *NodeMap
}

// Append a node to the graph.
func (g *Graph) Append(n Node) {
	g.Nodes = append(g.Nodes, n)
}

// Resolve the graph.
func (g *Graph) Resolve() []Node {
	g.unresolved = &NodeMap{m: map[string]int{}}
	g.resolved = &NodeMap{m: map[string]int{}}
	for _, n := range g.Nodes {
		g.resolve(n)
	}
	return g.resolved.List()
}

func (g *Graph) resolve(n Node) {
	g.unresolved.Append(n)
	defer g.resolved.Append(n)
	defer g.unresolved.Remove(n)
	for _, edge := range n.Requires() {
		if _, resolved := g.resolved.Lookup(edge.ID()); !resolved {
			if _, seen := g.unresolved.Lookup(edge.ID()); seen {
				panic("Circular reference")
			} else {
				g.resolve(edge)
			}
		}
	}
}

// Node on a graph.
type Node interface {
	ID() string
	Requires() []Node
}

// NodeMap is a map of nodes.
type NodeMap struct {
	l []Node
	m map[string]int
}

// List returns an ordered list of nodes.
func (m *NodeMap) List() []Node {
	return m.l
}

// Append a node.
func (m *NodeMap) Append(n Node) {
	m.l = append(m.l, n)
	m.m[n.ID()] = len(m.l) - 1
}

// Remove a node.
func (m *NodeMap) Remove(n Node) {
	ii := m.m[n.ID()]
	m.l = append(m.l[:ii], m.l[ii+1:]...)
	delete(m.m, n.ID())
}

// Lookup a node for the given id.
func (m *NodeMap) Lookup(id string) (Node, bool) {
	ii, ok := m.m[id]
	if !ok {
		return nil, false
	}
	return m.l[ii], true
}

// taskNode wraps a Task to implement the Node interface.
// Index allows us to return a Task object for a given Task name.
type taskNode struct {
	Task  Task
	Index map[string]Task
}

func (n taskNode) ID() string {
	return n.Task.Name
}

func (n taskNode) Requires() (list []Node) {
	for _, r := range n.Task.Requires {
		list = append(list, taskNode{Task: n.Index[r], Index: n.Index})
	}
	return list
}
