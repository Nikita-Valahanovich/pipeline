package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	bufferDrainInterval = 30 * time.Second
	bufferSize          = 10
)

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
		copy(r.array, r.array[1:])
		r.array[r.pos] = el
	} else {
		r.pos++
		r.array[r.pos] = el
	}
	log.Printf("[BUFFER] Добавлено число: %d\n", el)
}

func (r *RingIntBuffer) Get() []int {
	r.m.Lock()
	defer r.m.Unlock()
	if r.pos < 0 {
		return nil
	}
	output := append([]int(nil), r.array[:r.pos+1]...)
	r.pos = -1
	log.Printf("[BUFFER] Буфер очищен. Отдано: %v\n", output)
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
	c := source
	for _, stage := range p.stages {
		c = stage(p.done, c)
	}
	return c
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	dataSource := func() (<-chan int, <-chan bool) {
		c := make(chan int)
		done := make(chan bool)

		go func() {
			defer close(done)
			scanner := bufio.NewScanner(os.Stdin)
			for {
				fmt.Print("Введите число (или 'exit' для выхода): ")
				scanner.Scan()
				data := scanner.Text()
				if strings.EqualFold(data, "exit") {
					log.Println("[SOURCE] Завершение программы.")
					return
				}
				i, err := strconv.Atoi(data)
				if err != nil {
					log.Println("[SOURCE] Ошибка: введено не число.")
					continue
				}
				log.Printf("[SOURCE] Прочитано число: %d\n", i)
				c <- i
			}
		}()
		return c, done
	}

	negativeFilterStageInt := func(done <-chan bool, c <-chan int) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			for data := range c {
				if data > 0 {
					log.Printf("[STAGE 1] Пропущено положительное число: %d\n", data)
					out <- data
				} else {
					log.Printf("[STAGE 1] Отфильтровано отрицательное число: %d\n", data)
				}
			}
		}()
		return out
	}

	specialFilterStageInt := func(done <-chan bool, c <-chan int) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			for data := range c {
				if data%3 == 0 {
					log.Printf("[STAGE 2] Число %d проходит фильтр (делится на 3)\n", data)
					out <- data
				} else {
					log.Printf("[STAGE 2] Число %d не прошло фильтр (не делится на 3)\n", data)
				}
			}
		}()
		return out
	}

	bufferStageInt := func(done <-chan bool, c <-chan int) <-chan int {
		out := make(chan int)
		buffer := NewRingIntBuffer(bufferSize)

		go func() {
			defer close(out)
			ticker := time.NewTicker(bufferDrainInterval)
			defer ticker.Stop()

			for {
				select {
				case data, ok := <-c:
					if !ok {
						return
					}
					buffer.Push(data)
				case <-ticker.C:
					log.Println("[BUFFER] Время очистки буфера.")
					for _, data := range buffer.Get() {
						out <- data
					}
				case <-done:
					log.Println("[BUFFER] Завершение работы.")
					return
				}
			}
		}()
		return out
	}

	consumer := func(done <-chan bool, c <-chan int) {
		for data := range c {
			log.Printf("[CONSUMER] Получено число: %d\n", data)
		}
		log.Println("[CONSUMER] Завершение работы.")
	}

	source, done := dataSource()
	pipeline := NewPipelineInt(done, negativeFilterStageInt, specialFilterStageInt, bufferStageInt)
	consumer(done, pipeline.Run(source))
}
