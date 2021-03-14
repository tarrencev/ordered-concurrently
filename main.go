package orderedconcurrently

import (
	"sync"
)

// Options options for Process
type Options struct {
	PoolSize         int
	OutChannelBuffer int
}

// WorkFunction interface
type WorkFunction interface {
	Run() interface{}
}

type processInput struct {
	workFn WorkFunction
	order  uint64
	value  interface{}
}

// Process processes work function based on input.
// It Accepts an WorkFunction read channel, work function and concurrent go routine pool size.
// It Returns an interface{} channel.
func Process(inputChan <-chan WorkFunction, options *Options) <-chan interface{} {
	outputChan := make(chan interface{}, options.OutChannelBuffer)

	go func() {
		if options.PoolSize < 1 {
			// Set a minimum number of processors
			options.PoolSize = 1
		}
		processChan := make(chan *processInput, options.PoolSize)
		aggregatorChan := make(chan *processInput, options.PoolSize)
		doneSemaphoreChan := make(chan bool)

		// Go routine to print data in order
		go func() {
			var current uint64
			outputMap := make(map[uint64]*processInput, options.PoolSize)
			defer func() {
				close(outputChan)
				doneSemaphoreChan <- true
			}()
			for item := range aggregatorChan {
				if item.order != current {
					outputMap[item.order] = item
					continue
				}
				for {
					if item == nil {
						break
					}
					outputChan <- item.value
					delete(outputMap, current)
					current++
					item = outputMap[current]
				}
			}
		}()

		poolWg := sync.WaitGroup{}
		poolWg.Add(options.PoolSize)
		// Create a goroutine pool
		for i := 0; i < options.PoolSize; i++ {
			go func(worker int) {
				defer func() {
					poolWg.Done()
				}()
				for input := range processChan {
					input.value = input.workFn.Run()
					input.workFn = nil
					aggregatorChan <- input
				}
			}(i)
		}

		go func() {
			poolWg.Wait()
			close(aggregatorChan)
		}()

		go func() {
			defer func() {
				close(processChan)
			}()
			var order uint64
			for input := range inputChan {
				processChan <- &processInput{workFn: input, order: order}
				order++
			}
		}()
		<-doneSemaphoreChan
	}()
	return outputChan
}
