package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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

// inorderHash computes the in-order traversal hash of root
func inorderHash(root *Node) int {
	h := 1
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		walk(n.left)
		nv := n.val + 2
		h = (h*nv + nv) % 1000
		walk(n.right)
	}
	walk(root)
	return h
}

// hashTrees computes each tree's hash.
func hashTrees(roots []*Node, workers int) []int {
	n := len(roots)
	hashes := make([]int, n)
	if workers <= 1 || n == 0 {
		for i, r := range roots {
			hashes[i] = inorderHash(r)
		}
		return hashes
	}
	if workers > n {
		workers = n
	}

	var next int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for {
				i := int(atomic.AddInt64(&next, 1)) - 1
				if i >= n {
					return
				}
				hashes[i] = inorderHash(roots[i])
			}
		}()
	}
	wg.Wait()
	return hashes
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

	workers := *hashWorkers
	if workers < 1 {
		workers = 1
	}

	start := time.Now()
	hashes := hashTrees(roots, workers)
	hashTime := time.Since(start).Seconds()
	fmt.Printf("hashTime: %f\n", hashTime)

	_ = hashes
	_ = dataWorkers
	_ = compWorkers
}
