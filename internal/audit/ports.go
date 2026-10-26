package audit

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

// DefaultPorts are the ports a dev server, database or debugger most often grabs
// and then silently keeps after a crash.
var DefaultPorts = []int{
	3000, 3001, 4200, 5000, 5173, 8000, 8080, 8081, 8888, 9000,
	5432, 3306, 6379, 27017, 9229,
}

// portProbe records whether something accepted a connection.
type portProbe struct {
	Port int
	Open bool
}

// isListening dials loopback instead of reading the socket table: it needs no
// privileges and matches what a client would actually experience.
func isListening(port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func checkPorts(ports []int) []Finding {
	if len(ports) == 0 {
		ports = DefaultPorts
	}

	results := make([]portProbe, len(ports))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, p := range ports {
		wg.Add(1)
		go func(i, p int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = portProbe{Port: p, Open: isListening(p, 150*time.Millisecond)}
		}(i, p)
	}
	wg.Wait()

	var open []int
	for _, r := range results {
		if r.Open {
			open = append(open, r.Port)
		}
	}
	sort.Ints(open)

	if len(open) == 0 {
		return []Finding{{
			ID:       "ports.occupied",
			Title:    "No dev ports occupied",
			Severity: SevOK,
			Detail:   fmt.Sprintf("%s common dev ports are free.", formatInt(len(ports))),
		}}
	}

	list := make([]string, 0, len(open))
	for _, p := range open {
		list = append(list, formatInt(p))
	}
	return []Finding{{
		ID:       "ports.occupied",
		Title:    fmt.Sprintf("%s already in use", plural(len(open), "dev port")),
		Severity: SevInfo,
		Detail:   "Listening on loopback: " + joinComma(list) + ". Expected if you have a dev server running; otherwise a crashed process still holds them.",
		Fix:      "Find the owner with `netstat -ano | findstr :<port>` and stop it with `taskkill /PID <pid> /F`.",
		Weight:   minInt(len(open), 3),
	}}
}
