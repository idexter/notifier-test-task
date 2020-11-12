// Package main provides example executable application.
//
// Requirements described in "The Executable" section of "docs/go01_notifier.pdf".
//
// A program that uses the "notification" library.
// It reads stdin and send new messages every interval (which is configurable).
// Each line are interpreted as a new message that needs to be notified about.
// It also implements graceful shutdown on SIGINT.
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime/trace"
	"strings"
	"syscall"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/idexter/notifier-test-task/pkg/notifier"
)

var (
	urlFlag      string
	intervalFlag time.Duration
	traceFlag    bool
)

func main() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)

	kingpin.Flag("url", "URL").Required().StringVar(&urlFlag)
	kingpin.Flag("interval", "Notification interval\n").Short('i').Default("5s").DurationVar(&intervalFlag)
	kingpin.Flag("trace", "Trace an application\n").Short('t').BoolVar(&traceFlag)
	kingpin.Parse()

	if traceFlag {
		f, terr := os.Create("trace.out")
		if terr != nil {
			log.Fatalf("Unable to open trace.out file, reason: %v", terr)
		}
		defer f.Close() //nolint: errcheck
		if terr = trace.Start(f); terr != nil {
			log.Fatalf("Unable to start tracing, reason: %v", terr)
		}
		defer trace.Stop()
	}

	interrupt := make(chan struct{})
	notify := notifier.New(urlFlag, nil)
	notify.OnError(func(message []byte, err error) {
		log.Printf("Unable to send message \"%s\": %v", message, err)
	})

	go handleSignals(signals, func() {
		notify.Stop()
		close(interrupt)
	})

	reader := bufio.NewReader(os.Stdin)
	var err error
	var nextLine string
readLoop:
	for err != io.EOF {
		select {
		case <-interrupt:
			break readLoop
		default:
			nextLine, err = reader.ReadString('\n')
			if err == nil {
				msg := strings.TrimSuffix(nextLine, "\n")
				n, err := notify.Notify([]byte(msg))
				if err != nil {
					log.Printf("Unable to handle message #%d: %s, reason: %v", n, msg, err)
				}
				time.Sleep(intervalFlag)
			}
		}
	}

	notify.Wait()
	log.Printf("Done\n")
}

func handleSignals(sig <-chan os.Signal, stop func()) {
	fmt.Printf("%s received, canceling notifier context\n", <-sig)
	stop()
}
