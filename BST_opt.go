package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Node is a binary search tree node.
type Node struct {
	val         int
	left, right *Node
}

// insert adds v to the BST rooted at non-nil root
func insert(root *Node, v int) {
	cur := root
	for {
		if v < cur.val {
			if cur.left == nil {
				cur.left = &Node{val: v}
				return
			}
			cur = cur.left
		} else {
			if cur.right == nil {
				cur.right = &Node{val: v}
				return
			}
			cur = cur.right
		}
	}
}

// buildTree constructs a BST from vals inserted in order.
func buildTree(vals []int) *Node {
	if len(vals) == 0 {
		return nil
	}
	root := &Node{val: vals[0]}
	for _, v := range vals[1:] {
		insert(root, v)
	}
	return root
}

// buildRoots constructs one BST per input tree, preserving tree IDs.
func buildRoots(trees [][]int) []*Node {
	roots := make([]*Node, len(trees))
	for i, vals := range trees {
		roots[i] = buildTree(vals)
	}
	return roots
}

// inorderValues returns the in-order traversal of root.
func inorderValues(root *Node) []int {
	var out []int
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		walk(n.left)
		out = append(out, n.val)
		walk(n.right)
	}
	walk(root)
	return out
}

// parseInput reads path and returns one slice of ints per non-empty line.
// Each line is one BST, values in the order provided.
func parseInput(path string) ([][]int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil, nil
	}
	lines := strings.Split(text, "\n")
	trees := make([][]int, 0, len(lines))
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		vals := make([]int, 0, len(fields))
		for _, f := range fields {
			n, err := strconv.Atoi(f)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid value %q: %w", i+1, f, err)
			}
			vals = append(vals, n)
		}
		trees = append(trees, vals)
	}
	return trees, nil
}

func main() {
	hashWorkers := flag.Int("hash-workers", 0, "number of goroutines to hash BSTs")
	dataWorkers := flag.Int("data-workers", 0, "number of workers to update the hash map")
	compWorkers := flag.Int("comp-workers", 0, "number of goroutines to compare trees")
	inputPath := flag.String("input", "", "path to input file")
	flag.Parse()

	trees, err := parseInput(*inputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	roots := buildRoots(trees)

	// Temporary debug guard
	if os.Getenv("BST_DEBUG") != "" {
		fmt.Printf("trees=%d\n", len(trees))
		if len(trees) > 0 {
			fmt.Printf("first line fields=%d first val=%d\n", len(trees[0]), trees[0][0])
			fmt.Printf("last line fields=%d last val=%d\n", len(trees[len(trees)-1]), trees[len(trees)-1][len(trees[len(trees)-1])-1])
		}
		fmt.Printf("built %d trees\n", len(roots))
		bad := 0
		for i, r := range roots {
			vals := inorderValues(r)
			if len(vals) != len(trees[i]) || !sort.IntsAreSorted(vals) {
				fmt.Printf("tree %d in-order INVALID len=%d\n", i, len(vals))
				bad++
			}
		}
		fmt.Printf("in-order check done, invalid=%d\n", bad)
		if len(roots) > 0 {
			fmt.Printf("tree 0 in-order: %v\n", inorderValues(roots[0]))
		}
	}

	_ = hashWorkers
	_ = dataWorkers
	_ = compWorkers
}
