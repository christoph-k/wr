package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/christoph-k/go-fsevents"
)

var debug = false

func main() {
	var pids []int
	var pidslock sync.Mutex
	var command []string
	var exclude []string

	for i, arg := range os.Args {
		switch arg {
		case "--":
			command = os.Args[i+1:]
			break
		case "-exclude":
			excludestr := os.Args[i+1]
			exclude = strings.Split(excludestr, ",")
			for idx, e := range exclude {
				exclude[idx] = strings.Trim(e, " ")
			}
		case "-debug":
			debug = true
		}
		exclude = append(exclude, "/.git/")
		exclude = append(exclude, "/.svn/")
		exclude = append(exclude, "/.tmp/")
		exclude = append(exclude, "/.swp/")
	}
	if len(command) == 0 {
		usage()
		os.Exit(1)
	}

	logDebugf("running in debug mode\n")
	// intercept SIGINT and kill childprocesses
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		kill(&pids, &pidslock)
		os.Exit(0)
	}()

	w, err := fsevents.NewWatcher(".", time.Millisecond*200)
	if err != nil {
		panic(err)
	}
	go run(&pids, &pidslock, command)
	for {
		e := <-w.EventChan
		excluded := false
		for _, excl := range exclude {
			if strings.Contains(e.Path, excl) {
				excluded = true
			}
		}
		if e.FileInfo == nil || e.FileInfo.IsDir() || excluded {
			continue
		}
		fmt.Printf("%v has changed\n", e.Path)
		kill(&pids, &pidslock)
		logDebugf("stopping watcher\n")
		//w.Stop()
		w, err = fsevents.NewWatcher(".", time.Millisecond*200)
		if err != nil {
			panic(err)
		}
		go run(&pids, &pidslock, command)
	}
}

func run(pids *[]int, pidslock *sync.Mutex, command []string) {
	fmt.Printf("running %v\n", command)
	cmd := exec.Command(command[0], command[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()

	p := cmd.Process
	if p != nil {
		pidslock.Lock()
		*pids = append(*pids, p.Pid)
		pidslock.Unlock()
	}
	cmd.Wait()
}

func kill(pids *[]int, pidslock *sync.Mutex) {
	logDebugf("locking pidslock\n")
	pidslock.Lock()
	defer pidslock.Unlock()
	logDebugf("locked pidslock\n")
	for _, pid := range *pids {
		fmt.Printf("killing %v\n", pid)
		logDebugf("sending SIGTERM to %v\n", -pid)
		syscall.Kill(-pid, syscall.SIGTERM)
		time.Sleep(time.Millisecond * 100)
		logDebugf("sending SIGKILL to %v\n", -pid)
		syscall.Kill(-pid, syscall.SIGKILL)
	}
	*pids = make([]int, 0)
}

func usage() {
	fmt.Printf("usage: wr [-exclude \"file1, file2\"] -- <command with args>\n")
}

func logDebugf(str string, v ...interface{}) {
	if debug {
		fmt.Printf(str, v...)
	}
}
