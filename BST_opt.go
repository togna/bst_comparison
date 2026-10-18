package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
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

// hashPair associates a tree's hash with its ID.
type hashPair struct {
	hash, id int
}

// buildGroupsSequential maps each hash to the IDs of trees with that hash,
// using the main thread only.
func buildGroupsSequential(hashes []int) map[int][]int {
	groups := make(map[int][]int)
	for id, h := range hashes {
		groups[h] = append(groups[h], id)
	}
	return groups
}

// buildGroupsChannel has `workers` goroutines send (hash, id) pairs to a single
// manager goroutine that owns and updates the map.
func buildGroupsChannel(hashes []int, workers int) map[int][]int {
	n := len(hashes)
	if workers <= 1 {
		return buildGroupsSequential(hashes)
	}
	if workers > n {
		workers = n
	}

	ch := make(chan hashPair)
	groups := make(map[int][]int)
	done := make(chan struct{})
	go func() {
		for p := range ch {
			groups[p.hash] = append(groups[p.hash], p.id)
		}
		close(done)
	}()

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(worker int) {
			defer wg.Done()
			for id := worker; id < n; id += workers {
				ch <- hashPair{hash: hashes[id], id: id}
			}
		}(w)
	}
	wg.Wait()
	close(ch)
	<-done
	return groups
}

// buildGroupsMutex has `workers` goroutines update a shared map, each acquiring
// a single mutex around its insertion.
func buildGroupsMutex(hashes []int, workers int) map[int][]int {
	n := len(hashes)
	if workers <= 1 {
		return buildGroupsSequential(hashes)
	}
	if workers > n {
		workers = n
	}

	groups := make(map[int][]int)
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(worker int) {
			defer wg.Done()
			for id := worker; id < n; id += workers {
				h := hashes[id]
				mu.Lock()
				groups[h] = append(groups[h], id)
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	return groups
}

// printHashGroups prints every hash group with more than one tree, sorted by
// hash and then by ID, in the form "<hash>: <id> <id> ...".
func printHashGroups(groups map[int][]int) {
	keys := make([]int, 0, len(groups))
	for h, ids := range groups {
		if len(ids) > 1 {
			keys = append(keys, h)
		}
	}
	sort.Ints(keys)
	for _, h := range keys {
		ids := groups[h]
		sort.Ints(ids)
		parts := make([]string, len(ids))
		for i, id := range ids {
			parts[i] = strconv.Itoa(id)
		}
		fmt.Printf("%d: %s\n", h, strings.Join(parts, " "))
	}
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

	if *dataWorkers >= 1 {
		var groups map[int][]int
		switch {
		case *hashWorkers <= 1:
			groups = buildGroupsSequential(hashes)
		case *dataWorkers == 1:
			groups = buildGroupsChannel(hashes, *hashWorkers)
		default:
			groups = buildGroupsMutex(hashes, *dataWorkers)
		}
		fmt.Printf("hashGroupTime: %f\n", time.Since(start).Seconds())
		printHashGroups(groups)
	}

	_ = compWorkers
}
