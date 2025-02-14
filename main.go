package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const bufferDrainInterval time.Duration = 30 * time.Second

const bufferSize int = 10

type RingIntBuffer struct {
	array []int
	pos   int
	size  int
	m     sync.Mutex
}

func NewRingIntBuffer(size int) *RingIntBuffer {
	return &RingIntBuffer{make([]int, size), -1, size, sync.Mutex{}}
}

func (r *RingIntBuffer) Push(el int) {
	r.m.Lock()
	defer r.m.Unlock()
	if r.pos == r.size-1 {
		for i := 1; i <= r.size-1; i++ {
			r.array[i-1] = r.array[i]
		}
		r.array[r.pos] = el
	} else {
		r.pos++
		r.array[r.pos] = el
	}
}

func (r *RingIntBuffer) Get() []int {
	if r.pos < 0 {
		return nil
	}
	r.m.Lock()
	defer r.m.Unlock()
	var output []int = r.array[:r.pos+1]
	r.pos = -1
	return output
}

type StageInt func(<-chan bool, <-chan int) <-chan int

type PipeLineInt struct {
	stages []StageInt
	done   <-chan bool
}

func NewPipelineInt(done <-chan bool, stages ...StageInt) *PipeLineInt {
	return &PipeLineInt{done: done, stages: stages}
}

func (p *PipeLineInt) Run(source <-chan int) <-chan int {
	var c <-chan int = source
	for index := range p.stages {
		c = p.runStageInt(p.stages[index], c)
	}
	return c
}

func (p *PipeLineInt) runStageInt(stage StageInt, sourceChan <-chan int) <-chan int {
	return stage(p.done, sourceChan)
}
func main() {
	dataSource := func() (<-chan int, <-chan bool) {
		c := make(chan int)
		done := make(chan bool)
		go func() {
			defer close(done)
			scanner := bufio.NewScanner(os.Stdin)
			var data string
			for {
				scanner.Scan()
				data = scanner.Text()
				if strings.EqualFold(data, "exit") {
					fmt.Println("Программа завершила работу!")
					return
				}
				i, err := strconv.Atoi(data)
				if err != nil {
					fmt.Println("Программа обрабатывает только целые числа!")
					continue
				}
				c <- i
			}
		}()
		return c, done
	}

	negativeFilterStageInt := func(done <-chan bool, c <-chan int) <-chan int {
		convertedIntChan := make(chan int)
		go func() {
			for {
				select {
				case data := <-c:
					if data > 0 {
						select {
						case convertedIntChan <- data:
						case <-done:
							return
						}
					}
				case <-done:
					return
				}
			}
		}()
		return convertedIntChan
	}

	specialFilterStageInt := func(done <-chan bool, c <-chan int) <-chan int {
		filteredIntChan := make(chan int)
		go func() {
			for {
				select {
				case data := <-c:
					if data != 0 && data%3 == 0 {
						select {
						case filteredIntChan <- data:
						case <-done:
							return
						}
					}
				case <-done:
					return
				}
			}
		}()
		return filteredIntChan
	}

	bufferStageInt := func(done <-chan bool, c <-chan int) <-chan int {
		bufferedIntChan := make(chan int)
		buffer := NewRingIntBuffer(bufferSize)
		go func() {
			for {
				select {
				case data := <-c:
					buffer.Push(data)
				case <-done:
					return
				}
			}
		}()

		go func() {
			for {
				select {
				case <-time.After(bufferDrainInterval):
					bufferData := buffer.Get()
					if bufferData != nil {
						for _, data := range bufferData {
							select {
							case bufferedIntChan <- data:
							case <-done:
								return
							}
						}
					}
				case <-done:
					return
				}
			}
		}()
		return bufferedIntChan
	}
	// Потребитель данных от пайплайна
	consumer := func(done <-chan bool, c <-chan int) {
		for {
			select {
			case data := <-c:
				fmt.Printf("Обработаны данные: %d\n", data)
			case <-done:
				return
			}
		}
	}

	source, done := dataSource()

	pipeline := NewPipelineInt(done, negativeFilterStageInt, specialFilterStageInt, bufferStageInt)
	consumer(done, pipeline.Run(source))
}
