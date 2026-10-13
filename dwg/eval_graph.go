package dwg

// EVALUATION_GRAPH object (OdDbEvalGraph) — the dependency graph behind
// associative/dynamic block evaluation. Field order recovered from
// `OdDbEvalGraph::dwgInFields` (TD_DynBlocks.tx:0x1d8c3c): two leading
// `rdInt32` words, a node count with that many 7-`rdInt32` node records
// (each also carries an `H` ownership handle in the handle stream, costing
// no data-stream bits on R2007+), then an edge count with that many
// 10-`rdInt32` edge records. The corpus records close with delta 0.

const (
	evalGraphNodeWords = 7
	evalGraphEdgeWords = 10
	evalGraphMaxItems  = 65536
)

// EvalGraph is a decoded EVALUATION_GRAPH record.
type EvalGraph struct {
	Header0 int32
	Header1 int32
	Nodes   [][evalGraphNodeWords]int32
	Edges   [][evalGraphEdgeWords]int32
}

// decodeEvalGraphObject decodes an EVALUATION_GRAPH straight from its raw
// object payload.
func decodeEvalGraphObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (EvalGraph, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return EvalGraph{}, err
	}
	var g EvalGraph
	if g.Header0, err = o.data.ReadBL(); err != nil {
		return g, err
	}
	if g.Header1, err = o.data.ReadBL(); err != nil {
		return g, err
	}
	numNodes, err := o.data.ReadBL()
	if err != nil {
		return g, err
	}
	if numNodes < 0 || int(numNodes) > evalGraphMaxItems {
		return g, newError(ErrSectionMap, "EVALUATION_GRAPH node count %d out of range", numNodes)
	}
	g.Nodes = make([][evalGraphNodeWords]int32, 0, numNodes)
	for k := int32(0); k < numNodes; k++ {
		var node [evalGraphNodeWords]int32
		for j := range node {
			if node[j], err = o.data.ReadBL(); err != nil {
				return g, err
			}
		}
		g.Nodes = append(g.Nodes, node)
	}
	numEdges, err := o.data.ReadBL()
	if err != nil {
		return g, err
	}
	if numEdges < 0 || int(numEdges) > evalGraphMaxItems {
		return g, newError(ErrSectionMap, "EVALUATION_GRAPH edge count %d out of range", numEdges)
	}
	g.Edges = make([][evalGraphEdgeWords]int32, 0, numEdges)
	for k := int32(0); k < numEdges; k++ {
		var edge [evalGraphEdgeWords]int32
		for j := range edge {
			if edge[j], err = o.data.ReadBL(); err != nil {
				return g, err
			}
		}
		g.Edges = append(g.Edges, edge)
	}
	if err := o.finish("EVALUATION_GRAPH"); err != nil {
		return g, err
	}
	return g, nil
}
