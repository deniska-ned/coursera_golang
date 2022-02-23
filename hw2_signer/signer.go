package main

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

// сюда писать код

func ExecutePipeline(jobs ...job) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()

	in := make(chan interface{})

	for _, j := range jobs {
		out := make(chan interface{})

		wg.Add(1)
		go jobWorker(j, in, out, wg)

		in = out
	}
}

func jobWorker(j job, in, out chan interface{}, wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(out)

	j(in, out)
}

func SingleHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()

	for v := range in {
		data := strconv.Itoa(v.(int))

		wg.Add(1)
		go singleHash(out, data, wg)
	}
}

var mu = &sync.Mutex{}

func DataSignerMd5Wrapper(data string) string {
	defer mu.Unlock()
	mu.Lock()
	return DataSignerMd5(data)
}

func crc32ll(data string, out chan<- string) {
	out <- DataSignerCrc32(data)
}

func singleHash(out chan interface{}, data string, wg *sync.WaitGroup) {
	defer wg.Done()

	leftChan := make(chan string)
	go crc32ll(data, leftChan)

	right := DataSignerCrc32(DataSignerMd5Wrapper(data))

	left := <-leftChan

	out <- left + "~" + right
}

func MultiHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()

	for v := range in {
		data := v.(string)

		wg.Add(1)
		go multiHashWorker(out, data, wg)
	}
}

const th = 5

func crc32ToArr(arr *[th + 1]string, i int, data string, wg *sync.WaitGroup) {
	defer wg.Done()

	arr[i] = DataSignerCrc32(data)
}

func multiHashWorker(out chan interface{}, data string, wg *sync.WaitGroup) {
	defer wg.Done()

	var arr [th + 1]string

	wgn := &sync.WaitGroup{}

	for i := 0; i <= th; i++ {
		wgn.Add(1)
		go crc32ToArr(&arr, i, strconv.Itoa(i)+data, wgn)
	}

	wgn.Wait()

	res := strings.Join(arr[:], "")
	out <- res
}

func CombineResults(in, out chan interface{}) {
	var input []string

	for v := range in {
		input = append(input, v.(string))
	}

	sort.Strings(input)

	res := strings.Join(input, "_")

	out <- res
}
