package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

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

	// Temporary debug guard
	if os.Getenv("BST_DEBUG") != "" {
		fmt.Printf("trees=%d\n", len(trees))
		if len(trees) > 0 {
			fmt.Printf("first line fields=%d first val=%d\n", len(trees[0]), trees[0][0])
			fmt.Printf("last line fields=%d last val=%d\n", len(trees[len(trees)-1]), trees[len(trees)-1][len(trees[len(trees)-1])-1])
		}
	}

	_ = hashWorkers
	_ = dataWorkers
	_ = compWorkers
}
