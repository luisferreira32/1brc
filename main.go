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
	"strings"
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

func solveLine(line string, solution map[string]*solutionItem) error {
	values := strings.Split(line, ";")
	s, ok := solution[values[0]]
	if !ok {
		s = &solutionItem{}
		solution[values[0]] = s
	}

	temp, err := strconv.ParseFloat(values[1], 32)
	if err != nil {
		return fmt.Errorf("on line: %s, got err: %w", line, err)
	}

	s.acc += temp
	s.count += 1
	if s.max < temp {
		s.max = temp
	}
	if s.min > temp {
		s.min = temp
	}
	return nil
}

const (
	bufferSize = 1_048_576
)

// Emit to stdout sorted alphabetically by station name, and the result values
// per station in the format <min>/<mean>/<max>, rounded to one fractional digit.
func solve1brc(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}

	solution := make(map[string]*solutionItem)
	b := make([]byte, bufferSize)
	p := 0

	log.Printf("starting to read file %s by chunks of %v bytes\n", filename, bufferSize)
	for {
		n, err := f.Read(b[p:])
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		lines := strings.Split(string(b[:p+n]), "\n")
		for _, line := range lines[:len(lines)-1] { // skip last, most likely incomplete line
			err := solveLine(line, solution)
			if err != nil {
				return err
			}
		}

		lastLine := lines[len(lines)-1]
		p = len(lastLine)
		if p > 0 {
			copy(b[:len(lastLine)], []byte(lastLine))
		}
	}
	if p != 0 { // edge case: we actually had a full line in the buffer that we just carried over
		solveLine(string(b[:p]), solution)
	}

	keys := make([]string, 0, len(solution))
	for k := range solution {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		item := solution[k]
		mean := math.Round(10*item.acc/float64(item.count)) / 10 // rounded to 1 decimal point
		fmt.Printf("%s=%.1f/%.1f/%.1f\n", k, item.min, mean, item.max)
	}
	return nil
}

func main() {
	defer panicHandler()

	a, err := parseArgs()
	gracefullyHanldeErrors(err)

	if a.profile {
		f, err := os.Create("cpu.prof")
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
