//   Copyright 2013 Fang Li <surivlee@gmail.com>
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package main

import (
	"flag"
	"github.com/kr/beanstalk"
	"log"
	"time"
)

// Get Parameters from cli
var publishers = flag.Int("p", 1, "number of concurrent publishers, default to 1")
var readers = flag.Int("r", *publishers, "number of concurrent readers, default to number of publishers")
var count = flag.Int("n", 10000, "Count of jobs to be processed, default to 10000")
var host = flag.String("h", "localhost:11300", "Host to beanstalkd, default to localhost:11300")
var size = flag.Int("s", 256, "Size of data, default to 256. in byte")
var drain = flag.Bool("d", false, "Drain the beanstalk before starting test")
var fill = flag.Int("f", 0, "Place <f> jobs on the beanstalk before starting test")

func testPublisher(h string, count int, size int, ch chan int) {
	conn, e := beanstalk.Dial("tcp", h)
	defer conn.Close()
	data := make([]byte, size)
	if e != nil {
		log.Fatal(e)
	}
	for i := 0; i < count; i++ {
		_, err := conn.Put(data, 0, 0, 120*time.Second)
		if err != nil {
			log.Fatal(err)
		}
	}
	ch <- 1
}

func testReader(h string, count int, ch chan int) {
	conn, e := beanstalk.Dial("tcp", h)
	defer conn.Close()
	if e != nil {
		log.Fatal(e)
	}
	for i := 0; i < count; i++ {
		id, _, e := conn.Reserve(250 * time.Millisecond)
		if e != nil {
			log.Println(e)
			continue
		}
		e = conn.Delete(id)
		if e != nil {
			log.Println(e)
		}
	}
	ch <- 1
}

func drainBeanstalk(h string) {
	log.Println("Draining beanstalk")
	conn, e := beanstalk.Dial("tcp", h)
	defer conn.Close()
	if e != nil {
		log.Fatal(e)
	}
	for {
		id, _, e := conn.Reserve(250 * time.Millisecond)
		if e != nil {
			return
		}
		e = conn.Delete(id)
		if e != nil {
			log.Println(e)
		}
	}
}

func fillBeanstalk(h string, count int, size int) {
	log.Println("Filling beanstalk")
	ch := make(chan int)
	go testPublisher(h, count, size, ch)
	<-ch
}

func main() {
	flag.Parse()
	if *drain {
		drainBeanstalk(*host)
	}
	if (*fill) > 0 {
		fillBeanstalk(*host, *fill, *size)
	}

	log.Println("Target host: ", *host)
	log.Println("Starting publishers: ", *publishers)
	log.Println("Starting readers: ", *readers)
	log.Println("Total jobs to be processed: ", *count)
	log.Println("Benchmarking, be patient ...")
	chPublisher := make(chan int)
	chReader := make(chan int)
	t0 := time.Now()

	if (*publishers) > 0 {
		publishCount := *count / *publishers
		for i := 0; i < *publishers; i++ {
			go testPublisher(*host, publishCount, *size, chPublisher)
		}
	}

	if (*readers) > 0 {
		readCount := *count / *readers
		for i := 0; i < *readers; i++ {
			go testReader(*host, readCount, chReader)
		}
	}

	// Wait for return, assume publishers will finish first
	if (*publishers) > 0 {
		for i := 0; i < *publishers; i++ {
			<-chPublisher
		}

		log.Println("---------------")
		delta := time.Now().Sub(t0)
		log.Println("Publishers finished at: ", delta)
		log.Println("Publish rate: ", float64(*count)/delta.Seconds(), " req/s")
	}

	if (*readers) > 0 {
		for i := 0; i < *readers; i++ {
			<-chReader
		}
		delta := time.Now().Sub(t0)
		log.Println("Readers finished at: ", delta)
		log.Println("Read rate: ", float64(*count)/delta.Seconds(), " req/s")
	}
}
