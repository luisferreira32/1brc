# The One Billion Row Challenge

This is a Go implementation to solve "The One Billion Row Challenge".

DISCLAIMER: Not an official run. Not in an official machine. Not with the official language. Definitely late. However, it seemed like a fun optimization challenge when I stumbled uppon it.

---

Credits to the challenge creator and all contributors can be found in: https://github.com/gunnarmorling/1brc

# Progress 📈

| Iteration | User Time | Notes                                                                                          |
| --------- | --------- | ---------------------------------------------------------------------------------------------- |
| #0        | 1m53s     | Naive setup for baseline                                                                       |
| #1        | 1m24s     | Optimize the initial string parsing given the challenge constraints                            |
| #2        | 1m5s      | Remove cast to string to do a converstion to float64 and just optimize that within constraints |

# The process

Being completely unofficial one has to have a baseline, so a (possibly) correct solution was first obtained with iteration **\#0**. Then the process is as follows:

1. Run

```
time go run . ./data/measurements.txt | tee ./out/run_X.txt
```

2. Validate

```
diff ./out/run_X.txt ./out/run_0.txt
```

3. Profile

```
go run . -p ./data/measurements.txt
go tool pprof -http :8080 cpu<timestamp>.prof
```

4. Optimize!

# Worklog

**\#0**: The main objective would be to do a naive single threaded approach, already with some opinionated ways of coding, such that I could get pprof running on it and start some real optimizations.

**\#1**: Looking at the initial profile there is an initial surprise: we don't actually waste that much time reading the file and have ~80% of the processing spent on string manipulation and accessing the solution map. There is a need for a new data structure and a new parsing. Since ~39% of the time was done in the `strings.Split` function, let's optimize that first.

The optimization process here was simple: we have some strict constraints on the format of each line, so work with it to iterate over the read buffer slice fiding the characters that break the line or divide the city from the temperature reading.

**\#2**: The removal of `strings.Split` was a success! Based on the baseline profiling and reinforced by the last profile, we need to figure out a better for a couple of things:

- Figuring out a data structure where accessing it does not take ~35% of the time
- Avoid as much as we can casting slice bytes into strings! This is taking ~22% of the time with most of it being in a runtime allocation of memory.
- Parsing the float numbers (~13%)

Let's pick the low hanging fruit: avoid unnecessary cast to string and a proper speed up of the float64 parsing.

**\#3**: Other 20 seconds shaved off in the implementation! Now the profiling still points to the same initial issue: most of the time is spent accessing the data structure. In a first approach, both slices and tries do not offer "out-of-the-box" improvements, but they can be the way forwards as accessing data within those structures is less opaque in implementation to the user. However, since we can see the map access is bottlenecked by compute power, it is an easy step to split this into worker routines to fully utilize the CPU cores.
