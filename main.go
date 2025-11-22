package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"runtime/debug"
	"runtime/pprof"
	"slices"
	"strconv"
	"sync"
	"time"
)

const (
	readBufferSize = 4 * 1024 * 1024 // 4 MiB pages
	educatedJump   = 3               // {city-name; 2:+};[-]{0-9},{0-99}

	workerNum = 16
)

func panicHandler() {
	r := recover()
	if r != nil {
		log.Printf("Something went wrong...\n%v\n%s\n", r, debug.Stack())
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

// From the rules:
// > Temperature value: non null double between -99.9 (inclusive) and 99.9 (inclusive), always with one fractional digit
func fastParseFloat64(b []byte) float64 {
	num := 0
	i := 0
	neg := false
	if b[i] == '-' {
		neg = true
		i++ // skip '-'
	}
	for {
		if b[i] == '.' {
			break
		}
		num *= 10
		num += int(b[i]) - 48

		i++
	}
	i++ // skip '.'
	dec := .1 * float64(int(b[i])-48)

	if neg {
		return -(float64(num) + dec)
	}

	return float64(num) + dec
}

type solutionItem struct {
	min   float64
	max   float64
	count int
	acc   float64
}

func solveLine(line []byte, solution map[string]*solutionItem) error {
	i := 0
	for {
		if line[i] == ';' {
			break
		}
		i++
	}

	name := string(line[:i])
	s, ok := solution[name]
	if !ok {
		s = &solutionItem{}
		solution[name] = s
	}

	i++ // skip the ;
	num := fastParseFloat64(line[i:])
	s.acc += num
	s.count += 1
	if s.max < num {
		s.max = num
	}
	if s.min > num {
		s.min = num
	}
	return nil
}

func processBuffer(b []byte, solution map[string]*solutionItem) {
	fi := 0 // line front-index
	ri := 0 // line rear-index
	for {
		if fi >= len(b) {
			break
		}
		if b[fi] == '\n' {
			err := solveLine(b[ri:fi], solution)
			if err != nil {
				log.Printf("[ERROR] %v", err)
				return
			}
			ri = fi + 1 // skip \n
			fi += educatedJump
		}
		fi++
	}
}

// Emit to stdout sorted alphabetically by station name, and the result values
// per station in the format <min>/<mean>/<max>, rounded to one fractional digit.
func printSolutions(solutions []map[string]*solutionItem) {
	solution := make(map[string]*solutionItem, workerNum)
	for _, s := range solutions {
		for k, v := range s {
			item, ok := solution[k]
			if !ok {
				item = v
				solution[k] = item
			}

			// merge the maps
			item.acc += v.acc
			item.count += v.count
			if item.max < v.max {
				item.max = v.max
			}
			if item.min > v.min {
				item.min = v.min
			}
		}
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
}

type workItem struct {
	bufferIndex int
	bufferLen   int
}

func solve1brc(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}

	var (
		readBuffer    = make([]byte, readBufferSize)
		wg            = sync.WaitGroup{}
		solutions     = make([]map[string]*solutionItem, workerNum)
		workerBuffers = make([][]byte, workerNum)
		toProcess     = make(chan *workItem, workerNum+1)
		doneProcess   = make(chan int, workerNum+1)
	)

	for n := range workerNum {
		solutions[n] = make(map[string]*solutionItem)
		workerBuffers[n] = make([]byte, readBufferSize)
		doneProcess <- n // signal ready

		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				item, ok := <-toProcess
				if !ok {
					return
				}
				b := workerBuffers[item.bufferIndex][:item.bufferLen]
				processBuffer(b, solutions[item.bufferIndex])
				doneProcess <- item.bufferIndex
			}

		}()
	}
	remain := 0

	log.Printf("starting to read file %s by chunks of %v bytes\n", filename, readBufferSize)
	for {
		n, err := f.Read(readBuffer[remain:])
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		blen := remain + n // buffer len after read
		li := blen - 1     // last line break index
		for {
			if readBuffer[li] == '\n' {
				break
			}
			li--
		}

		i := <-doneProcess
		copy(workerBuffers[i][:li], readBuffer[:li])          // copy data
		toProcess <- &workItem{bufferIndex: i, bufferLen: li} // signal worker
		remain = blen - li - 1                                // carry over last partial line and continue reading
		if remain > 0 {
			copy(readBuffer[:remain], []byte(readBuffer[li+1:blen]))
		}
	}

	close(toProcess)
	wg.Wait()

	printSolutions(solutions)
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
