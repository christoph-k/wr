package main

import (
	"fmt"
	"github.com/christoph-k/go-fsevents"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	var pids []int
	var pidslock sync.Mutex
	var command []string
	var exclude []string

	for i, arg := range os.Args {
		if arg == "--" {
			command = os.Args[i+1:]
		}
		if arg == "-exclude" {
			excludestr := os.Args[i+1]
			exclude = strings.Split(excludestr, ",")
			for idx, e := range exclude {
				exclude[idx] = strings.Trim(e, " ")
			}
		}
	}
	if len(command) == 0 {
		usage()
		os.Exit(1)
	}

	w, err := fsevents.NewWatcher(".", 200)
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
		w, err = fsevents.NewWatcher(".", 200)
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
	pidslock.Lock()
	defer pidslock.Unlock()
	for _, pid := range *pids {
		fmt.Printf("killing %v\n", pid)
		syscall.Kill(-pid, syscall.SIGTERM)
		time.Sleep(time.Millisecond * 100)
		syscall.Kill(-pid, syscall.SIGKILL)
	}
	*pids = make([]int, 0)
}

func usage() {
	fmt.Printf("usage: wr [-exclude \"file1, file2\"] -- <command with args>\n")
}
