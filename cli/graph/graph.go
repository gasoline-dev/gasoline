package graph

import "fmt"

type Graph struct {
	NodeToInDegrees NodeToInDegrees
}

func New(nodeToDeps map[string][]string) *Graph {
	g := &Graph{}
	return g
}

type NodeToInDegrees map[string]int

func (g *Graph) SetNodeToInDegrees() {
	g.NodeToInDegrees = make(NodeToInDegrees)
	g.NodeToInDegrees["test"] = 0
	fmt.Println("HI")
}
