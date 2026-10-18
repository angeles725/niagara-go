package navbuild

import "runtime"

// parseBatch is how many files the workers parse between two merges, so at
// most that many parsed files are held at once.
const parseBatch = 256

// parseWorkers is the number of goroutines that read and parse sources.
func parseWorkers() int { return min(4, runtime.GOMAXPROCS(0)) }

// inOrder runs parse(i) for i in [0, n) on parseWorkers goroutines and calls
// merge(i, result) on the calling goroutine in index order, a batch at a time,
// so a builder parses in parallel and merges exactly as the Python loop does.
// The first merge error stops it.
func inOrder[T any](n int, parse func(i int) T, merge func(i int, r T) error) error {
	results := make([]T, parseBatch)
	workers := parseWorkers()
	for start := 0; start < n; start += parseBatch {
		size := min(parseBatch, n-start)
		jobs := make(chan int)
		done := make(chan struct{})
		for range min(workers, size) {
			go func() {
				for i := range jobs {
					results[i] = parse(start + i)
				}
				done <- struct{}{}
			}()
		}
		for i := range size {
			jobs <- i
		}
		close(jobs)
		for range min(workers, size) {
			<-done
		}
		for i := range size {
			if err := merge(start+i, results[i]); err != nil {
				return err
			}
			var zero T
			results[i] = zero
		}
	}
	return nil
}
