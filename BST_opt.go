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

// printTreeGroups prints equivalence classes with more than one tree, sorted by
// their smallest ID, in the form "group <k>: <id> <id> ...".
func printTreeGroups(classes [][]int) {
	sort.Slice(classes, func(i, j int) bool { return classes[i][0] < classes[j][0] })
	k := 0
	for _, ids := range classes {
		sort.Ints(ids)
		parts := make([]string, len(ids))
		for i, id := range ids {
			parts[i] = strconv.Itoa(id)
		}
		fmt.Printf("group %d: %s\n", k, strings.Join(parts, " "))
		k++
	}
}

// unionFind tracks equivalence classes of tree IDs.
type unionFind struct {
	parent []int
}

func newUnionFind(n int) *unionFind {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	return &unionFind{parent: p}
}

func (u *unionFind) find(x int) int {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]]
		x = u.parent[x]
	}
	return x
}

func (u *unionFind) union(a, b int) {
	ra, rb := u.find(a), u.find(b)
	if ra != rb {
		u.parent[ra] = rb
	}
}

// equalTrees reports whether two trees have the same in-order traversal.
func equalTrees(a, b *Node) bool {
	var sa, sb []*Node
	pushLeft := func(stack *[]*Node, n *Node) {
		for n != nil {
			*stack = append(*stack, n)
			n = n.left
		}
	}
	pushLeft(&sa, a)
	pushLeft(&sb, b)
	for len(sa) > 0 && len(sb) > 0 {
		an := sa[len(sa)-1]
		sa = sa[:len(sa)-1]
		bn := sb[len(sb)-1]
		sb = sb[:len(sb)-1]
		if an.val != bn.val {
			return false
		}
		pushLeft(&sa, an.right)
		pushLeft(&sb, bn.right)
	}
	return len(sa) == 0 && len(sb) == 0
}

// collectClasses groups the IDs that belong to a multi-tree hash group by their
// union-find root, returning equivalence classes with more than one tree.
func collectClasses(uf *unionFind, groups map[int][]int, n int) [][]int {
	inGroup := make(map[int]bool)
	for _, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		for _, id := range ids {
			inGroup[id] = true
		}
	}

	byRoot := make(map[int][]int)
	for id := 0; id < n; id++ {
		if inGroup[id] {
			r := uf.find(id)
			byRoot[r] = append(byRoot[r], id)
		}
	}

	var classes [][]int
	for _, ids := range byRoot {
		if len(ids) > 1 {
			classes = append(classes, ids)
		}
	}
	return classes
}

// compareTreesSequential compares all pairs within each multi-tree hash group.
func compareTreesSequential(roots []*Node, groups map[int][]int) [][]int {
	uf := newUnionFind(len(roots))
	for _, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				if equalTrees(roots[ids[i]], roots[ids[j]]) {
					uf.union(ids[i], ids[j])
				}
			}
		}
	}
	return collectClasses(uf, groups, len(roots))
}

// compareTreesGoroutinePerPair spawns one goroutine per candidate pair within
// each multi-tree hash group. Results are merged under a mutex.
func compareTreesGoroutinePerPair(roots []*Node, groups map[int][]int) [][]int {
	uf := newUnionFind(len(roots))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				wg.Add(1)
				go func(a, b int) {
					defer wg.Done()
					if equalTrees(roots[a], roots[b]) {
						mu.Lock()
						uf.union(a, b)
						mu.Unlock()
					}
				}(ids[i], ids[j])
			}
		}
	}
	wg.Wait()
	return collectClasses(uf, groups, len(roots))
}

// comparisonPair is a pair of tree IDs to compare.
type comparisonPair struct {
	a, b int
}

// workBuffer is a bounded buffer of comparison pairs. Producers block when the
// buffer is full and consumers block when it is empty; both wait on condition
// variables rather than spinning.
type workBuffer struct {
	mu       sync.Mutex
	notFull  *sync.Cond
	notEmpty *sync.Cond
	items    []comparisonPair
	capacity int
	closed   bool
}

func newWorkBuffer(capacity int) *workBuffer {
	b := &workBuffer{capacity: capacity}
	b.notFull = sync.NewCond(&b.mu)
	b.notEmpty = sync.NewCond(&b.mu)
	return b
}

// push adds a pair, blocking while the buffer is full.
func (b *workBuffer) push(p comparisonPair) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for len(b.items) == b.capacity {
		b.notFull.Wait()
	}
	b.items = append(b.items, p)
	b.notEmpty.Signal()
}

// pop removes a pair, blocking while the buffer is empty and not yet closed.
// It returns ok=false once the buffer is closed and drained.
func (b *workBuffer) pop() (comparisonPair, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for len(b.items) == 0 && !b.closed {
		b.notEmpty.Wait()
	}
	if len(b.items) == 0 {
		return comparisonPair{}, false
	}
	p := b.items[0]
	b.items = b.items[1:]
	b.notFull.Signal()
	return p, true
}

// close marks the buffer closed and wakes all waiting consumers.
func (b *workBuffer) close() {
	b.mu.Lock()
	b.closed = true
	b.notEmpty.Broadcast()
	b.mu.Unlock()
}

// compareTreesPool feeds candidate pairs through a bounded buffer of capacity
// `workers` to a pool of `workers` goroutines. Results are merged under a
// mutex.
func compareTreesPool(roots []*Node, groups map[int][]int, workers int) [][]int {
	if workers <= 1 {
		return compareTreesSequential(roots, groups)
	}

	buf := newWorkBuffer(workers)
	uf := newUnionFind(len(roots))
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for {
				p, ok := buf.pop()
				if !ok {
					return
				}
				if equalTrees(roots[p.a], roots[p.b]) {
					mu.Lock()
					uf.union(p.a, p.b)
					mu.Unlock()
				}
			}
		}()
	}

	for _, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				buf.push(comparisonPair{a: ids[i], b: ids[j]})
			}
		}
	}
	buf.close()
	wg.Wait()

	return collectClasses(uf, groups, len(roots))
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

	var groups map[int][]int
	if *dataWorkers >= 1 || *compWorkers >= 1 {
		switch {
		case *hashWorkers <= 1:
			groups = buildGroupsSequential(hashes)
		case *dataWorkers == 1:
			groups = buildGroupsChannel(hashes, *hashWorkers)
		default:
			groups = buildGroupsMutex(hashes, *dataWorkers)
		}
	}

	if *dataWorkers >= 1 {
		fmt.Printf("hashGroupTime: %f\n", time.Since(start).Seconds())
		printHashGroups(groups)
	}

	if *compWorkers >= 1 {
		compStart := time.Now()
		var classes [][]int
		if *compWorkers == 1 {
			classes = compareTreesSequential(roots, groups)
		} else {
			classes = compareTreesPool(roots, groups, *compWorkers)
		}
		fmt.Printf("compareTreeTime: %f\n", time.Since(compStart).Seconds())
		printTreeGroups(classes)
	}
}
