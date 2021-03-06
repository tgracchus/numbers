package numbers

import (
	"bufio"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const reportPeriod = 10
const numberLogFileName = "numbers.log"

// StartNumberServer start the number server tcp application with
// number of concurrent server connections and at the given address.
func StartNumberServer(concurrentConnections int, address string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if concurrentConnections < 0 {
		log.Panicf("concurrency level should be more than 0, not %d", concurrentConnections)
	}
	terminate := make(chan int)
	listeners := make([]ConnectionListener, concurrentConnections)
	numbersOuts := make([]chan int, concurrentConnections)
	for i := 0; i < concurrentConnections; i++ {
		cnnListener, numbers := NewSingleConnectionListener(DefaultTCPController, terminate)
		listeners[i] = cnnListener
		numbersOuts[i] = numbers
	}
	deDuplicatedNumbers := NumberStore(reportPeriod, numbersOuts, terminate)
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	done := FileWriter(deDuplicatedNumbers, dir+"/"+numberLogFileName)
	multipleListener := NewMultipleConnectionListener(listeners)

	cancelContextWhenTerminateSignal(cancel, terminate, done)
	err = StartServer(ctx, multipleListener, address, done)
	if err != nil {
		log.Printf("%v", err)
	}
}

func cancelContextWhenTerminateSignal(cancel context.CancelFunc,
	terminate chan int, done chan int) chan int {
	go func() {
		for {
			select {
			case <-terminate:
				cancel()
			case <-done:
				return
			}
		}
	}()
	return terminate
}

const readDeadline = 30 * time.Second

// DefaultTCPController handles the parsing protocol defined in the requirements.
// Accepts a channel terminate to send termination signal and return a channel from where the numbers will be issued
// once they are parsed.
func DefaultTCPController(ctx context.Context, c net.Conn, numbers chan int, terminate chan int) error {
	reader := bufio.NewReader(c)
	for {
		err := c.SetReadDeadline(time.Now().Add(readDeadline))
		if err != nil {
			return errors.Wrap(err, "SetReadDeadline")
		}
		data, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return errors.Wrap(err, "ReadString")
		}

		data = strings.TrimSuffix(data, "\n")
		if len(data) != 9 {
			return errors.Wrap(fmt.Errorf("client: %s, no 9 char length string %s", c.RemoteAddr().String(), data), "check for 9 digits")
		}
		if data == "terminate" {
			select {
			case <-terminate:
				return TERMINATED
			default:
				close(terminate)
			}
			return TERMINATED
		}
		number, err := strconv.Atoi(data)
		if err != nil {
			return errors.Wrap(err, "strconv.Atoi")
		}

		select {
		case <-terminate:
			return TERMINATED
		default:
			numbers <- number
		}
	}
}

// NumberStore given a list of channels it listens to all of them and deduplicated the numbers received.
// If the number is not duplicated handles it to the returned channel for further processing.
// It also keeps track of total and unique numbers and the current 10s windows of unique and duplicated numbers.
func NumberStore(reportPeriod int, ins []chan int, terminate chan int) chan int {
	out := make(chan int)
	in := fanIn(ins, terminate)
	numbers := make(map[int]bool)
	var total int64 = 0
	var currentUnique int64 = 0
	var currentDuplicated int64 = 0
	ticker := time.NewTicker(time.Duration(reportPeriod) * time.Second)
	go func() {
		defer ticker.Stop()
		defer close(out)
		for {
			select {
			case number, more := <-in:
				if more {
					total++
					if _, ok := numbers[number]; ok {
						currentDuplicated++
					} else {
						currentUnique++
						numbers[number] = true
						out <- number
					}
				} else {
					return
				}
			case tick := <-ticker.C:
				log.Printf("Report %v Received %d unique numbers, %d duplicates. Unique total: %d. Total: %d",
					tick, currentUnique, currentDuplicated, len(numbers), total)
				currentUnique = 0
				currentDuplicated = 0
			}
		}
	}()
	return out
}

func fanIn(ins []chan int, terminate chan int) chan int {
	var wg sync.WaitGroup
	wg.Add(len(ins))
	out := make(chan int)
	go func() {
		for _, ch := range ins {
			go func(in chan int) {
				defer wg.Done()
				for {
					select {
					case element, more := <-in:
						if more {
							out <- element
						} else {
							return
						}
					case <-terminate:
						return
					}
				}
			}(ch)
		}
		wg.Wait()
		close(out)
	}()
	return out
}

// FileWriter writes al the numbers received at in channel and writes them to filePath.
// Returns a done channel when it is terminated
func FileWriter(in chan int, filePath string) chan int {
	done := make(chan int)
	f, err := os.Create(filePath)
	if err != nil {
		log.Fatal(err)
	}
	b := bufio.NewWriter(f)
	ticker := time.NewTicker(time.Duration(reportPeriod) * time.Second)
	go func() {
		defer ticker.Stop()
		defer closeFile(b, f)
		for {
			select {
			case number, more := <-in:
				if more {
					_, err := fmt.Fprintf(b, "%09d\n", number)
					if err != nil {
						log.Printf("%v", errors.Wrap(err, "Fprintf"))
					}
				} else {
					if err := b.Flush(); err != nil {
						log.Printf("%v", err)
					}
					close(done)
					return
				}
			case <-ticker.C:
				if err := b.Flush(); err != nil {
					log.Printf("%v", err)
				}
			}

		}
	}()
	return done
}

func closeFile(b *bufio.Writer, f *os.File) {
	if err := f.Close(); err != nil {
		log.Printf("%v", err)
	}
}
