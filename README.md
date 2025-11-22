# The One Billion Row Challenge

This is a Go implementation to solve "The One Billion Row Challenge".

DISCLAIMER: Not an official run. Not in an official machine. Not with the official language. Definitely late. However, it seemed like a fun optimization challenge when I stumbled uppon it.

---

Credits to the challenge creator and all contributors can be found in: https://github.com/gunnarmorling/1brc

# Progress 📈

| Iteration | User Time | Notes                                                               |
| --------- | --------- | ------------------------------------------------------------------- |
| #0        | 1m53s     | Naive setup for baseline                                            |
| #1        | 1m24s     | Optimize the initial string parsing given the challenge constraints |

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
