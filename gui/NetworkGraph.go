package gui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type NodeConfig struct {
	ID    string
	Addr  string
	Peers []string
}

type NodeVisualization struct {
	nodes []NodeConfig
	th    *material.Theme
}

func NewNodeVisualization() *NodeVisualization {
	th := material.NewTheme()
	return &NodeVisualization{
		nodes: make([]NodeConfig, 0),
		th:    th,
	}
}

func (nv *NodeVisualization) AddNode(id string, addr string, peers ...string) {
	nv.nodes = append(nv.nodes, NodeConfig{
		ID:    id,
		Addr:  addr,
		Peers: peers,
	})
}

func (nv *NodeVisualization) Run() error {
	w := new(app.Window)
	w.Option(
		app.Title("Jetstream - P2P Topology"),
		app.Size(unit.Dp(700), unit.Dp(500)),
	)

	var ops op.Ops

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			nv.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (nv *NodeVisualization) layout(gtx layout.Context) layout.Dimensions {
	// 1. Draw solid dark background
	paint.Fill(gtx.Ops, color.NRGBA{R: 24, G: 28, B: 36, A: 255})

	// 2. Draw Title Header
	layout.Inset{Top: unit.Dp(15), Left: unit.Dp(15)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		title := material.H6(nv.th, "P2P Network Topology Graph")
		title.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		return title.Layout(gtx)
	})

	// 3. Compute Node Positions in a Ring Topology
	bounds := gtx.Constraints.Max
	centerX, centerY := float64(bounds.X)/2.0, float64(bounds.Y)/2.0+20
	radius := math.Min(float64(bounds.X), float64(bounds.Y)) * 0.3

	nodeCount := len(nv.nodes)
	nodePositions := make(map[string]image.Point)

	for i, node := range nv.nodes {
		angle := float64(i) * (2 * math.Pi / float64(nodeCount))
		x := centerX + radius*math.Cos(angle)
		y := centerY + radius*math.Sin(angle)
		nodePositions[node.ID] = image.Pt(int(x), int(y))
	}

	// 4. Draw Peer Connection Lines
	for _, node := range nv.nodes {
		startPt, ok := nodePositions[node.ID]
		if !ok {
			continue
		}
		for _, peerAddr := range node.Peers {
			// Find matching peer by address or ID
			for _, targetNode := range nv.nodes {
				if targetNode.Addr == peerAddr || targetNode.ID == peerAddr {
					endPt := nodePositions[targetNode.ID]
					drawLine(gtx.Ops, startPt, endPt, color.NRGBA{R: 70, G: 130, B: 240, A: 200}, 3)
				}
			}
		}
	}

	// 5. Draw Circle Nodes & Text Labels
	nodeRadius := 28
	for _, node := range nv.nodes {
		pos := nodePositions[node.ID]

		// Outer Circle (Node Body)
		circle := clip.Ellipse{
			Min: image.Pt(pos.X-nodeRadius, pos.Y-nodeRadius),
			Max: image.Pt(pos.X+nodeRadius, pos.Y+nodeRadius),
		}.Op(gtx.Ops)

		paint.FillShape(gtx.Ops, color.NRGBA{R: 50, G: 180, B: 120, A: 255}, circle)

		// Inner Circle (Border Outline)
		stroke := clip.Stroke{
			Path:  clip.Ellipse{Min: image.Pt(pos.X-nodeRadius, pos.Y-nodeRadius), Max: image.Pt(pos.X+nodeRadius, pos.Y+nodeRadius)}.Path(gtx.Ops),
			Width: 3,
		}.Op()
		paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, stroke)

		// Render Label Inside Circle
		label := material.Body1(nv.th, node.ID)
		label.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}

		// Offset macro to draw text centered over node
		op.Offset(image.Pt(pos.X-6, pos.Y-10)).Add(gtx.Ops)
		label.Layout(gtx)
		op.Offset(image.Pt(-(pos.X - 6), -(pos.Y - 10))).Add(gtx.Ops)
	}

	return layout.Dimensions{Size: bounds}
}

// Utility function to render edge lines between nodes
func drawLine(ops *op.Ops, from, to image.Point, col color.NRGBA, width int) {
	var path clip.Path
	path.Begin(ops)
	path.MoveTo(layout.FPt(from))
	path.LineTo(layout.FPt(to))
	stroke := clip.Stroke{
		Path:  path.End(),
		Width: float32(width),
	}.Op()
	paint.FillShape(ops, col, stroke)
}
