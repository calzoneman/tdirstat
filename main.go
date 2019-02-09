package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

const (
	kibibyte = 1 << 10
	mebibyte = 1 << 20
	gibibyte = 1 << 30
)

type sizeinfo struct {
	path string
	size int64
}

func (s *sizeinfo) formattedSize() string {
	var n int64
	var unit string

	if s.size > gibibyte {
		n = s.size / gibibyte
		unit = "GiB"
	} else if s.size > mebibyte {
		n = s.size / mebibyte
		unit = "MiB"
	} else if s.size > kibibyte {
		n = s.size / kibibyte
		unit = "KiB"
	} else {
		n = s.size
		unit = "B"
	}

	return fmt.Sprintf("%3d %3s", n, unit)
}

type byName []*tview.TreeNode

func (b byName) Len() int {
	return len(b)
}
func (b byName) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}
func (b byName) Less(i, j int) bool {
	ri := b[i].GetReference().(*sizeinfo)
	rj := b[j].GetReference().(*sizeinfo)

	// Attempt case-insensitive comparison, fall back to case sensitive
	li := strings.ToLower(ri.path)
	lj := strings.ToLower(rj.path)

	if li == lj {
		return ri.path < rj.path
	} else {
		return li < lj
	}
}

type bySizeDesc []*tview.TreeNode

func (b bySizeDesc) Len() int {
	return len(b)
}
func (b bySizeDesc) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}
func (b bySizeDesc) Less(i, j int) bool {
	ri := b[i].GetReference().(*sizeinfo)
	rj := b[j].GetReference().(*sizeinfo)

	return ri.size > rj.size
}

const (
	sortByName = iota
	sortBySizeDesc
)

var currentSortMethod = sortBySizeDesc

func main() {
	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	if len(os.Args) > 1 {
		root = os.Args[1]
		info, err := os.Stat(root)
		if err != nil {
			fmt.Printf("Cannot open %s: %s\n", root, err.Error())
			os.Exit(1)
		} else if !info.IsDir() {
			fmt.Printf("Cannot open %s: is not a directory\n", root)
			os.Exit(1)
		}
	}

	root = filepath.Clean(root)

	fmt.Printf("Scanning %s... (may take a while)\n", root)

	nodes := map[string]*tview.TreeNode{}
	exists := func(path string) bool {
		_, ok := nodes[path]
		return ok
	}
	// Propagate filesize to parent dirs
	propagateSize := func(leaf string, size int64) {
		for cur := filepath.Dir(leaf); exists(cur); cur = filepath.Dir(cur) {
			nodes[cur].GetReference().(*sizeinfo).size += size
		}
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		path = filepath.Clean(path)
		node := tview.NewTreeNode(path).
			SetExpanded(false).
			SetSelectable(info.IsDir())

		nodes[path] = node

		parent, ok := nodes[filepath.Dir(path)]
		if ok {
			parent.AddChild(node)
		} else if path != root {
			panic(fmt.Errorf("unrooted child path at %s", path))
		}

		if info.IsDir() {
			node.SetReference(&sizeinfo{path: path, size: 0})
			node.SetColor(tcell.ColorGreen)
		} else {
			node.SetReference(&sizeinfo{path: path, size: info.Size()})
			node.SetColor(tcell.ColorSilver)
			propagateSize(path, info.Size())
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Unable to populate filesystem tree: %s", err.Error())
		os.Exit(1)
	}

	var expand func(node *tview.TreeNode)
	expand = func(node *tview.TreeNode) {
		node.SetExpanded(true)
		if len(node.GetChildren()) == 1 {
			expand(node.GetChildren()[0])
		}
	}

	tree := tview.NewTreeView().
		SetRoot(nodes[root]).
		SetCurrentNode(nodes[root]).
		SetGraphicsColor(tcell.ColorSilver).
		SetSelectedFunc(func(node *tview.TreeNode) {
			if node.IsExpanded() {
				node.SetExpanded(false)
			} else {
				expand(node)
			}
		})

	sortTree := func() {
		nodes[root].Walk(func(node, parent *tview.TreeNode) bool {
			switch currentSortMethod {
			case sortByName:
				sort.Sort(byName(node.GetChildren()))
			case sortBySizeDesc:
				sort.Sort(bySizeDesc(node.GetChildren()))
			}

			return true
		})
	}

	// Render size information into the tree
	nodes[root].Walk(func(node, parent *tview.TreeNode) bool {
		info := node.GetReference().(*sizeinfo)
		node.SetText(info.formattedSize() + " " + info.path)
		return true
	})

	sortTree()
	expand(nodes[root])

	app := tview.NewApplication().
		SetRoot(tree, true).
		SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Rune() == 's' {
				switch currentSortMethod {
				case sortByName:
					currentSortMethod = sortBySizeDesc
				case sortBySizeDesc:
					currentSortMethod = sortByName
				}

				sortTree()
			}

			return event
		})

	if err := app.Run(); err != nil {
		panic(err)
	}
}
