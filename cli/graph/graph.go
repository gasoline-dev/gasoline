package graph

import "gas/helpers"

type Graph struct {
	nodeToDeps               NodeToDeps
	nodeToInDegrees          nodeToInDegrees
	nodesWithInDegreesOfZero nodesWithInDegreesOfZero
	nodeToIntermediates      nodeToIntermediates
	nodeToGroup              nodeToGroup
	depthToNode              depthToNode
	nodeToDepth              nodeToDepth
	groupToDepthToNodes      groupToDepthToNodes
}

type NodeToDeps map[string][]string

func New(nodeToDeps NodeToDeps) *Graph {
	g := &Graph{nodeToDeps: nodeToDeps}
	g.setNodeToInDegrees()
	g.setNodesWithInDegreesOfZero()
	g.setNodeToIntermediates()
	g.setNodeToGroup()
	g.setDepthToNode()
	g.setNodeToDepth()
	g.setGroupToDepthToNodes()
	return g
}

type nodeToInDegrees map[string]int

/*
In degrees is how many incoming edges a target node has.
*/
func (g *Graph) setNodeToInDegrees() {
	g.nodeToInDegrees = make(nodeToInDegrees)

	// Loop over nodes and their dependencies.
	for _, deps := range g.nodeToDeps {
		// Increment node's in degrees everytime it's
		// found to be a dependency of another resource.
		for _, dep := range deps {
			g.nodeToInDegrees[dep]++
		}
	}

	for node := range g.nodeToDeps {
		// Node has to have an in degrees of 0 if it
		// hasn't been placed yet.
		if _, ok := g.nodeToInDegrees[node]; !ok {
			g.nodeToInDegrees[node] = 0
		}
	}
}

type nodesWithInDegreesOfZero []string

func (g *Graph) setNodesWithInDegreesOfZero() {
	for node, inDegree := range g.nodeToInDegrees {
		if inDegree == 0 {
			g.nodesWithInDegreesOfZero = append(g.nodesWithInDegreesOfZero, node)
		}
	}
}

type nodeToIntermediates map[string][]string

/*
Intermediate nodes are nodes within the source resource's
directed graph path.

For example, given a graph of A->B, B->C, and X->C, B and C are
intermediates of A, C is an intermediate of B, and C is an
intermediate of X.

Finding intermediate nodes is necessary for grouping related nodes.
It wouldn't be possible to know A and X are relatives in the above
example without them.
*/
func (g *Graph) setNodeToIntermediates() {
	g.nodeToIntermediates = make(nodeToIntermediates)
	memo := make(map[string][]string)
	for node := range g.nodeToDeps {
		g.nodeToIntermediates[node] = walkDeps(node, g.nodeToDeps, memo)
	}
}

func walkDeps(node string, nodeToDeps NodeToDeps, memo map[string][]string) []string {
	if result, found := memo[node]; found {
		return result
	}

	result := make([]string, 0)
	if deps, ok := nodeToDeps[node]; ok {
		for _, dep := range deps {
			if !helpers.IsInSlice(result, dep) {
				result = append(result, dep)
				for _, transitiveDep := range walkDeps(dep, nodeToDeps, memo) {
					if !helpers.IsInSlice(result, transitiveDep) {
						result = append(result, transitiveDep)
					}
				}
			}
		}
	}
	memo[node] = result

	return result
}

type nodeToGroup map[string]int

/*
A group is an integer assigned to nodes that share
at least one common relative.
*/
func (g *Graph) setNodeToGroup() {
	g.nodeToGroup = make(nodeToGroup)

	group := 0
	for _, sourceNode := range g.nodesWithInDegreesOfZero {
		if _, ok := g.nodeToGroup[sourceNode]; !ok {
			// Initialize source node's group.
			g.nodeToGroup[sourceNode] = group

			// Set group for source node's intermediates.
			for _, intermediateNode := range g.nodeToIntermediates[sourceNode] {
				if _, ok := g.nodeToGroup[intermediateNode]; !ok {
					g.nodeToGroup[intermediateNode] = group
				}
			}

			// Set group for distant relatives of source node.
			// For example, given a graph of A->B, B->C, & X->C,
			// A & X both have an in degrees of 0. When walking the graph
			// downward from their positions, neither will gain knowledge of the
			// other's existence because they don't have incoming edges. To account
			// for that, all nodes with an in degrees of 0 need to be checked
			// with one another to see if they have a common relative (common
			// intermediate nodes in each's direct path). In this case, A & X
			// share a common relative in "C". Therefore, A & X should be assigned
			// to the same group.
			for _, possibleDistantRelativeNode := range g.nodesWithInDegreesOfZero {
				// Skip source node from the main for loop.
				if possibleDistantRelativeNode != sourceNode {
					// Loop over possible distant relative's intermediates.
					for _, possibleDistantRelativeIntermediateNode := range g.nodeToIntermediates[possibleDistantRelativeNode] {
						// Check if possible distant relative's intermediate
						// is also an intermediate of source node.
						if helpers.IncludesString(g.nodeToIntermediates[sourceNode], possibleDistantRelativeIntermediateNode) {
							// If so, possibl distant relative and source node
							// are distant relatives and belong to the same group.
							g.nodeToGroup[possibleDistantRelativeNode] = group
						}
					}
				}
			}
			group++
		}
	}
}

type depthToNode map[int][]string

/*
Depth is an integer that describes how far down the graph
a resource is.

For example, given a graph of A->B, B->C, A has a depth
of 0, B has a depth of 1, and C has a depth of 2.
*/
func (g *Graph) setDepthToNode() {
	g.depthToNode = make(depthToNode)

	numOfNodesToProcess := len(g.nodeToDeps)

	depth := 0

	for _, nodeWithInDegreesOfZero := range g.nodesWithInDegreesOfZero {
		g.depthToNode[depth] = append(g.depthToNode[depth], nodeWithInDegreesOfZero)
		numOfNodesToProcess--
	}

	for numOfNodesToProcess > 0 {
		for _, nodeAtDepth := range g.depthToNode[depth] {
			for _, depNode := range g.nodeToDeps[nodeAtDepth] {
				g.depthToNode[depth+1] = append(g.depthToNode[depth+1], depNode)
				numOfNodesToProcess--
			}
		}
		depth++
	}
}

type nodeToDepth map[string]int

func (g *Graph) setNodeToDepth() {
	g.nodeToDepth = make(nodeToDepth)
	for depth, nodes := range g.depthToNode {
		for _, node := range nodes {
			g.nodeToDepth[node] = depth
		}
	}
}

type groupToDepthToNodes map[int]map[int][]string

func (g *Graph) setGroupToDepthToNodes() {
	g.groupToDepthToNodes = make(groupToDepthToNodes)
	for node, group := range g.nodeToGroup {
		if _, ok := g.groupToDepthToNodes[group]; !ok {
			g.groupToDepthToNodes[group] = make(map[int][]string)
		}
		depth := g.nodeToDepth[node]
		if _, ok := g.groupToDepthToNodes[group][depth]; !ok {
			g.groupToDepthToNodes[group][depth] = make([]string, 0)
		}
		g.groupToDepthToNodes[group][depth] = append(g.groupToDepthToNodes[group][depth], node)
	}
}
