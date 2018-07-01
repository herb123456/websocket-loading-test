package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/olekukonko/tablewriter"
)

type Record struct {
	Number      int
	Connections int
	Errors      int
	Duration    float64
}

var open int64
var concurrency int

func main() {
	parseFlags()

	setRLimit()

	wg := &sync.WaitGroup{}

	// 計算要跑幾回測試
	floatSize := float64(open) / float64(concurrency)
	recordSize := int(math.Ceil(floatSize))
	records := make([]*Record, recordSize)

	runLoadtesting(wg, records)

	printResultTable(recordSize, records)
}

func runLoadtesting(wg *sync.WaitGroup, records []*Record) {
	// 跑第幾次
	var index int64
	// 跑第幾回
	var round int64

	u, err := url.Parse(os.Args[len(os.Args)-1])
	if err != nil {
		fmt.Errorf("URL Parse error: " + err.Error())
	}

	log.Printf("connecting to %s", u.String())

	for index < open {
		fmt.Printf("Start %d ...\n", index)

		var number int64
		var success int64
		var fail int64
		startTime := time.Now()

		for j := 0; j < concurrency; j++ {
			if index >= open {
				break
			}

			wg.Add(1)
			atomic.AddInt64(&index, 1)
			atomic.AddInt64(&number, 1)
			go func(id int64) {
				defer wg.Done()
				connectErr := connectAndRead(u.String(), int(id))

				if connectErr != nil {
					atomic.AddInt64(&fail, 1)
				} else {
					atomic.AddInt64(&success, 1)
				}
			}(index)
		}

		wg.Wait()

		endTime := time.Now().Sub(startTime).Seconds()

		// record
		r := &Record{
			Number:      int(index),
			Connections: int(success),
			Errors:      int(fail),
			Duration:    endTime,
		}

		records[round] = r
		atomic.AddInt64(&round, 1)
	}
}

func parseFlags() () {
	flag.Int64Var(&open, "n", 100, "number of connection")
	flag.IntVar(&concurrency, "c", 20, "Concurrent connection.")
	flag.Parse()
}

func printResultTable(recordSize int, records []*Record) {
	data := make([][]string, recordSize+1)
	totalConnections := 0
	totalErrors := 0
	totalDuration := 0.0

	for k := 0; k < recordSize; k++ {
		r := records[k]
		totalConnections += r.Connections
		totalErrors += r.Errors
		totalDuration += r.Duration

		data[k] = []string{
			strconv.Itoa(r.Number),
			strconv.Itoa(r.Connections),
			strconv.Itoa(r.Errors),
			strconv.FormatFloat(r.Duration, 'f', 6, 64),
		}
	}

	data[recordSize] = []string{
		"Total",
		strconv.Itoa(totalConnections),
		strconv.Itoa(totalErrors),
		strconv.FormatFloat(totalDuration, 'f', 6, 64),
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Number", "Connections", "Errors", "Duration(s)"})

	for _, v := range data {
		table.Append(v)
	}

	table.Render()
}

func connectAndRead(u string, index int) error {
	c, _, err := websocket.DefaultDialer.Dial(u+"?id="+strconv.Itoa(index), nil)
	if err != nil {
		return err
	}
	defer c.Close()

	c.WriteMessage(websocket.TextMessage, []byte("test"))

	_, _, readErr := c.ReadMessage()
	if readErr != nil {
		return readErr
	}

	return nil
}

func setRLimit() {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		fmt.Println("Error Getting Rlimit ", err)
	}
	rLimit.Max = 999999
	rLimit.Cur = 999999
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		fmt.Println("Error Setting Rlimit ", err)
	}
	err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		fmt.Println("Error Getting Rlimit ", err)
	}
}
