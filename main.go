package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	winlog "github.com/ofcoursedude/gowinlog"
	"github.com/subosito/gotenv"

	"github.com/ofcoursedude/winlogstream/colors"
)

func init() {
	gotenv.Load()
	/*
		Valid env:
		FORMAT=simple/rfc5424
		MSGOUT=full/singleLine/singleLineTrim
		LOGNAME=[name of log to hook into, defaults to Application]
		USECOLOR=true/false
	*/
}

var useColor bool

func main() {
	fmt.Println("Starting...")
	outputFormat := strings.ToLower(os.Getenv("FORMAT"))
	msgOut := strings.ToLower(os.Getenv("MSGOUT"))
	logName := os.Getenv("LOGNAME")
	useColor = strings.ToLower(os.Getenv("USECOLOR")) == "true"
	if logName == "" {
		logName = "Application"
	}

	var outputFormatFunc func(evt *winlog.WinLogEvent, msgFormat func(msg string) string) string
	var msgOutFunc func(msg string) string

	switch msgOut {
	case "full":
		msgOutFunc = func(msg string) string {
			return msg
		}
	case "singleline":
		msgOutFunc = singleLine
	case "singlelinetrim":
		msgOutFunc = singleLineTrim
	default:
		msgOutFunc = singleLine
	}

	switch outputFormat {
	case "simple":
		outputFormatFunc = toSimple
	case "rfc5424":
		outputFormatFunc = toRfc5424
	default:
		outputFormatFunc = toSimple
	}

	shutdowner := make(chan bool)
	go func(sig chan bool) {
		//when we exit, signal it's done
		defer func() {
			sig <- true
		}()
		watcher, err := winlog.NewWinLogWatcher()
		if err != nil {
			fmt.Printf("Couldn't create watcher: %v\n", err)
			return
		}

		// Recieve any future messages on the Application channel
		// "*" doesn't filter by any fields of the event
		watcher.SubscribeFromNow(logName, "*")
		defer watcher.Shutdown()
	EventCollectionLoop:
		for {
			select {
			case evt := <-watcher.Event():
				fmt.Println(outputFormatFunc(evt, msgOutFunc))
			case err := <-watcher.Error():
				fmt.Printf("\nError: %v\n\n", err)
				//Waiting for graceful shutdown signal is good enough to omit
				//the 'default' block
			case <-sig:
				break EventCollectionLoop
				/* default:
				// If no event is waiting, need to wait or do something else, otherwise
				// the the app fails on deadlock.
				<-time.After(1 * time.Millisecond) */
			}
		}
	}(shutdowner)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
	fmt.Println("Attempting graceful shutdown")
	signal.Stop(ch)
	shutdowner <- true
	<-shutdowner
	fmt.Println("Finished")
}

func singleLine(msg string) string {
	return replaceMulti(msg, []string{"\r", "\n"}, " ")
	// return strings.ReplaceAll(msg, "\r\n", " ")
}

func singleLineTrim(msg string) string {
	return strings.Split(strings.Replace(msg, "\r", "", 1), "\r\n")[0]
}

func parse(
	evt *winlog.WinLogEvent,
	formatFunc func(evt *winlog.WinLogEvent, msgFormat func(msg string) string) []string,
	msgFormat func(msg string) string) string {
	output := formatFunc(evt, msgFormat)
	return strings.Join(output, " ")
}

func toSimple(evt *winlog.WinLogEvent, msgFormat func(msg string) string) string {
	level := eventLevel(evt.Level)
	var levelMsg string
	if useColor {
		levelMsg = fmt.Sprint(level.Color(), "[", level.String(), "]", colors.Reset)
	} else {
		levelMsg = fmt.Sprint("[", eventLevel(evt.Level).String(), "]")
	}
	output := []string{
		evt.Created.Format(time.RFC3339),
		levelMsg,
		evt.ProviderName,
		msgFormat(evt.Msg),
	}
	return strings.Join(output, " ")
}

func toRfc5424(evt *winlog.WinLogEvent, msgFormat func(msg string) string) string {
	output := []string{
		"<34>1",
		evt.Created.Format(time.RFC3339),
		fmt.Sprint("[", eventLevel(evt.Level).String(), "]"),
		evt.ComputerName,
		evt.ProviderName,
		strconv.FormatInt(int64(evt.ProcessId), 10),
		strconv.FormatInt(int64(evt.EventId), 10),
		msgFormat(evt.Msg),
	}
	return strings.Join(output, " ")
}

func replaceMulti(source string, toReplace []string, replacement string) string {
	toReturn := source
	for _, item := range toReplace {
		toReturn = strings.ReplaceAll(toReturn, item, replacement)
	}
	return toReturn
}
