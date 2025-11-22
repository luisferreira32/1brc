package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"runtime/pprof"
	"slices"
	"strconv"
	"time"
)

const (
	readBufferSize        = 1_048_576
	educatedJump          = 4 // {city-name; 1:+};[-]{0-9},{0-99}
	maxStationNameSize    = 100
	maxUniqueStationNames = 10_000
)

func panicHandler() {
	r := recover()
	if r != nil {
		log.Printf("Something went wrong...\n%v\n", r)
	}
}

func gracefullyHanldeErrors(err error) {
	if err != nil {
		panic(err.Error())
	}
}

type args struct {
	filename string
	profile  bool
}

func parseArgs() (args, error) {
	a := args{}
	flag.BoolVar(&a.profile, "p", false, "enable profiling")
	flag.Parse()

	flag.Usage = func() {
		fmt.Println(`This is a Go implementation for 1brc. To run it try with:
		<executable> <filename>

You can also enable profiling with
		<executable> -p <filename>`)
		flag.PrintDefaults()
	}

	sysargs := flag.Args()
	if len(sysargs) < 1 {
		flag.Usage()
		return a, errors.New("no filename was provided! executable is expected to run with: <bin> <filename>")
	}
	a.filename = sysargs[0]
	return a, nil
}

type solutionItem struct {
	min   float64
	max   float64
	count int
	acc   float64
}

type trieNode struct {
	letter   byte
	branches []*trieNode
	solution *solutionItem
}

func solveLine(line []byte, head *trieNode) error {
	node := head
	i := 0
	for {
		if line[i] == ';' || i >= len(line) {
			break
		}
		branchIndex := slices.IndexFunc(node.branches, func(n *trieNode) bool { return n.letter == line[i] })
		if branchIndex == -1 {
			newBranch := &trieNode{letter: line[i], branches: make([]*trieNode, 0, 50)}
			node.branches = append(node.branches, newBranch)
			node = newBranch
		} else {
			node = node.branches[branchIndex]
		}
		i++
	}

	if node.solution == nil {
		node.solution = &solutionItem{}
	}

	temp, err := strconv.ParseFloat(string(line[i+1:]), 32)
	if err != nil {
		return fmt.Errorf("on line: %s, got err: %w", line, err)
	}

	node.solution.acc += temp
	node.solution.count += 1
	if node.solution.max < temp {
		node.solution.max = temp
	}
	if node.solution.min > temp {
		node.solution.min = temp
	}
	return nil
}

func parseReadBuffer(b []byte, head *trieNode) (int, error) {
	var (
		err error
		i   = 0
		p   = 0
	)
	for {
		if i >= len(b) {
			break
		}
		if b[i] == '\n' {
			err = solveLine(b[p:i], head)
			if err != nil {
				return 0, err
			}
			p = i + 1 // skip \n
			i += educatedJump
		}
		i++
	}

	return len(b) - p, nil
}

func printStationData(b []byte, i int, node *trieNode) {
	b[i] = node.letter
	if node.solution != nil {
		mean := math.Round(10*node.solution.acc/float64(node.solution.count)) / 10 // rounded to 1 decimal point
		fmt.Printf("%s=%.1f/%.1f/%.1f\n", b[:i+1], node.solution.min, mean, node.solution.max)
		return
	}
	slices.SortFunc(node.branches, func(a, b *trieNode) int {
		return int(a.letter) - int(b.letter)
	})
	for _, branch := range node.branches {
		printStationData(b, i+1, branch)
	}
}

// Emit to stdout sorted alphabetically by station name, and the result values
// per station in the format <min>/<mean>/<max>, rounded to one fractional digit.
func solve1brc(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}

	head := &trieNode{branches: make([]*trieNode, 0, 50)}
	b := make([]byte, readBufferSize)
	p := 0

	log.Printf("starting to read file %s by chunks of %v bytes\n", filename, readBufferSize)
	for {
		n, err := f.Read(b[p:])
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		pn := p + n
		p, err = parseReadBuffer(b[:pn], head)
		if err != nil {
			return err
		}

		if p > 0 { // carry over last partial line
			copy(b[:p], []byte(b[pn-p:pn]))
		}
	}

	printBuffer := make([]byte, maxStationNameSize)
	slices.SortFunc(head.branches, func(a, b *trieNode) int {
		return int(a.letter) - int(b.letter)
	})
	for _, branch := range head.branches {
		printStationData(printBuffer, 0, branch)
	}
	return nil
}

func main() {
	defer panicHandler()

	a, err := parseArgs()
	gracefullyHanldeErrors(err)

	if a.profile {
		f, err := os.Create("cpu" + strconv.FormatInt(time.Now().Unix(), 10) + ".prof")
		if err != nil {
			gracefullyHanldeErrors(err)
		}
		defer func() {
			err = f.Close()
			if err != nil {
				log.Printf("[ERROR] %v\n", err)
			}
		}()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Printf("[ERROR] could not start cpu profile %v\n", err)
		}
		defer pprof.StopCPUProfile()

	}

	err = solve1brc(a.filename)
	gracefullyHanldeErrors(err)
}
