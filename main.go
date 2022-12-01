package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rkojedzinszky/zfsziltest/randomizer"
)

type blockStore struct {
	blocks  map[int64]randomizer.RandomID
	written int64
	lock    *sync.Mutex
}

func (p *blockStore) Set(block int64, id randomizer.RandomID) {
	atomic.AddInt64(&p.written, 1)

	p.lock.Lock()
	defer p.lock.Unlock()

	p.blocks[block] = id
}

func (p *blockStore) Written() int64 {
	return atomic.LoadInt64(&p.written)
}

func (p *blockStore) Length() int {
	return len(p.blocks)
}

func (p *blockStore) Iterate(f func(int64, randomizer.RandomID)) {
	for addr, id := range p.blocks {
		f(addr, id)
	}
}

type job struct {
	device string
	blocks int64
	ps     blockStore
	r      *randomizer.Randomizer
}

func newjob(dev string) *job {
	job := &job{
		device: dev,
	}
	file, err := os.Open(dev)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	size, err := file.Seek(0, 2)
	if err != nil {
		log.Panic(err)
	}

	job.blocks = size >> randomizer.Blockshift

	job.ps = blockStore{
		blocks: make(map[int64]randomizer.RandomID),
		lock:   &sync.Mutex{},
	}

	r, err := randomizer.NewRandomizer()
	if err != nil {
		log.Panic(err)
	}
	job.r = r

	return job
}

func (j *job) run(workers int) {
	wg := &sync.WaitGroup{}

	fmt.Println("Destroying the contents of", j.device)

	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go j.worker(wg)
	}

	wg.Wait()
}

func (j *job) worker(wg *sync.WaitGroup) {
	defer wg.Done()

	file, err := os.OpenFile(j.device, os.O_WRONLY|os.O_SYNC, 0)
	if err != nil {
		log.Printf("worker: error opening device: %+v\n", err)
		return
	}
	defer file.Close()

	for {
		rid, data := j.r.GetRandom()

		block := rand.Int63n(j.blocks)

		file.Seek(block<<randomizer.Blockshift, 0)

		n, err := file.Write(data)
		if err != nil {
			return
		}
		if n != randomizer.Blocksize {
			return
		}

		j.ps.Set(block, rid)
	}
}

func (j *job) check() {
	file, err := os.Open(j.device)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	fmt.Println("Checking", j.ps.Length(), "blocks")

	buf := make([]byte, randomizer.Blocksize)
	counter := 1
	errors := 0

	j.ps.Iterate(func(block int64, rid randomizer.RandomID) {
		fmt.Printf("Checking %6d / %6d block=%10d    ", counter, j.ps.Length(), block)

		file.Seek(block<<randomizer.Blockshift, 0)
		n, err := file.Read(buf)
		if err != nil {
			log.Panic(err)
		}
		if n != randomizer.Blocksize {
			log.Panic(fmt.Errorf("check: Short read"))
		}

		stored := j.r.GetByID(rid)
		if bytes.Equal(buf, stored) {
			fmt.Printf("ok    \r")
		} else {
			fmt.Printf("  error\n")
			errors++
		}

		counter++
	})

	fmt.Printf("\n\nTotal of errored blocks: %d\n", errors)
}

func main() {
	device := flag.String("device", "", "Device to check")
	threads := flag.Int("threads", 4, "Parallel threads to run")

	flag.Parse()

	if *device == "" {
		log.Panic("Specify device with -device")
	}

	job := newjob(*device)

	fmt.Println("Starting", *threads, "threads to stress test", *device)

	stop := make(chan struct{})
	go func() {
		c := time.NewTicker(500 * time.Millisecond)
		defer c.Stop()

		startTime := time.Now()
		prevTime := startTime
		var prev int64 = 0

		for {
			select {
			case <-stop:
				fmt.Println("\n")
				return
			case <-c.C:
			}

			now := time.Now()
			total := job.ps.Written()

			last := float64(total-prev) / float64(now.Sub(prevTime)) * float64(time.Second)
			avg := float64(total) / float64(now.Sub(startTime)) * float64(time.Second)
			fmt.Printf("Total written: %7d. Last IOPS=%5.1f  AVG IOPS=%5.1f\r", total, last, avg)

			prevTime = now
			prev = total
		}

	}()

	job.run(*threads)

	close(stop)

	time.Sleep(1 * time.Second)

	fmt.Println("Waiting for", *device, "to become available again")

	for {
		_, err := os.Stat(*device)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	job.check()
}
