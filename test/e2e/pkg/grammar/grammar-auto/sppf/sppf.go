// Package sppf is generated by gogll. Do not edit.

/*
Package sppf implements a Shared Packed Parse Forest as defined in:

	Elizabeth Scott, Adrian Johnstone
	GLL parse-tree generation
	Science of Computer Programming (2012), doi:10.1016/j.scico.2012.03.005
*/
package sppf

import (
	"bytes"
	"fmt"

	"github.com/goccmack/goutil/ioutil"

	"github.com/cometbft/cometbft/v2/test/e2e/pkg/grammar/grammar-auto/parser/symbols"
)

type Node interface {
	isNode()
	dot(*dotBuilder)
	Label() string
	String() string
}

type IntermediateNode struct {
	NT         symbols.NT
	Body       symbols.Symbols
	Pos        int
	Lext, Rext int
	Children   []*PackedNode
}

type SymbolNode struct {
	Symbol     string
	Lext, Rext int
	Children   []*PackedNode
}

type PackedNode struct {
	NT                symbols.NT
	Body              symbols.Symbols
	Pos               int
	Lext, Pivot, Rext int

	LeftChild  Node // Either an intermediate or Symbol node
	RightChild *SymbolNode
}

func (*IntermediateNode) isNode() {}
func (*SymbolNode) isNode()       {}
func (*PackedNode) isNode()       {}

func slotString(nt symbols.NT, body symbols.Symbols, pos int) string {
	w := new(bytes.Buffer)
	fmt.Fprintf(w, "%s:", nt)
	for i, sym := range body {
		fmt.Fprint(w, " ")
		if i == pos {
			fmt.Fprint(w, "•")
		}
		fmt.Fprint(w, sym)
	}
	if len(body) == pos {
		fmt.Fprint(w, "•")
	}
	return w.String()
}

func (n *IntermediateNode) Label() string {
	return fmt.Sprintf("\"%s:,%d,%d\"", slotString(n.NT, n.Body, n.Pos), n.Lext, n.Rext)
}

func (n *SymbolNode) Label() string {
	return fmt.Sprintf("\"%s,%d,%d\"", n.Symbol, n.Lext, n.Rext)
}

func (n *PackedNode) Label() string {
	return fmt.Sprintf("\"%s,%d,%d,%d\"", slotString(n.NT, n.Body, n.Pos), n.Lext, n.Pivot, n.Rext)
}

func (n *IntermediateNode) String() string {
	return "IN: " + n.Label()
}

func (n *SymbolNode) String() string {
	return "SN: " + n.Label()
}

func (n *PackedNode) String() string {
	return "PN: " + n.Label()
}

// ---- Dot ----

type dotBuilder struct {
	nodes map[string]bool // index = node.Label()
	w     *bytes.Buffer
}

func (bld *dotBuilder) add(n Node) {
	// fmt.Printf("dotBuilder.add: %s\n", n.Label())
	if bld.done(n) {
		panic(fmt.Sprintf("duplicate %s", n.Label()))
	}
	// fmt.Println(" Before:")
	// bld.dumpNodes()

	bld.nodes[n.Label()] = true

	// fmt.Println(" After:")
	// bld.dumpNodes()
	// fmt.Println()
}

func (bld *dotBuilder) done(n Node) bool {
	return bld.nodes[n.Label()]
}

func (bld *dotBuilder) dumpNodes() {
	for n, t := range bld.nodes {
		fmt.Printf("  %s = %t\n", n, t)
	}
}

// DotFile writes a graph representation of the SPPF in dot notation to file
func (root *SymbolNode) DotFile(file string) {
	bld := &dotBuilder{
		nodes: make(map[string]bool),
		w:     new(bytes.Buffer),
	}
	fmt.Fprintln(bld.w, "digraph SPPF {")
	root.dot(bld)
	fmt.Fprintln(bld.w, "}")
	ioutil.WriteFile(file, bld.w.Bytes())
}

func (n *IntermediateNode) dot(bld *dotBuilder) {
	// fmt.Println("in.dot", n.Label())

	if bld.done(n) {
		return
	}
	bld.add(n)

	fmt.Fprintf(bld.w, "%s [shape=box]\n", n.Label())

	for _, c := range n.Children {
		fmt.Fprintf(bld.w, "%s -> %s\n", n.Label(), c.Label())
		if !bld.done(c) {
			c.dot(bld)
		}
	}
}

func (n *PackedNode) dot(bld *dotBuilder) {
	// fmt.Println("pn.dot", n.Label(), "exist", bld.nodes[n.Label()])

	if bld.done(n) {
		return
	}
	bld.add(n)

	fmt.Fprintf(bld.w, "%s [shape=box,style=rounded,penwidth=3]\n", n.Label())
	if n.LeftChild != nil {
		if !bld.done(n.LeftChild) {
			n.LeftChild.dot(bld)
		}
		fmt.Fprintf(bld.w, "%s -> %s\n", n.Label(), n.LeftChild.Label())
	}
	if n.RightChild != nil {
		if !bld.done(n.RightChild) {
			n.RightChild.dot(bld)
		}
		fmt.Fprintf(bld.w, "%s -> %s\n", n.Label(), n.RightChild.Label())
	}
	if n.LeftChild != nil && n.RightChild != nil {
		fmt.Fprintf(bld.w, "%s,%s\n", n.LeftChild.Label(), n.RightChild.Label())
	}
}

func (n *SymbolNode) dot(bld *dotBuilder) {
	// fmt.Println("sn.dot", n.Label(), "done=", bld.done(n))

	if bld.done(n) {
		return
	}
	bld.add(n)

	fmt.Fprintln(bld.w, n.Label())
	for _, pn := range n.Children {
		// fmt.Printf("  child: %s\n", pn.Label())
		fmt.Fprintf(bld.w, "%s -> %s\n", n.Label(), pn.Label())
		if !bld.done(pn) {
			pn.dot(bld)
		}
	}
	for i, pn := range n.Children {
		if i > 0 {
			fmt.Fprint(bld.w, ";")
		}
		fmt.Fprintf(bld.w, "%s", pn.Label())
	}
	fmt.Fprintln(bld.w)
}
