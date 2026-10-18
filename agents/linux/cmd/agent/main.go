package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type event struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	ObservedAt time.Time      `json:"observedAt"`
	ReceivedAt time.Time      `json:"receivedAt"`
	Source     source         `json:"source"`
	Data       map[string]any `json:"data"`
	Privacy    privacy        `json:"privacy"`
}

type source struct {
	ID    string  `json:"id"`
	Kind  string  `json:"kind"`
	Trust float64 `json:"trust"`
}

type privacy struct {
	Level                string `json:"level"`
	ContainsPersonalData bool   `json:"containsPersonalData"`
}

type agentCapabilities struct {
	KernelRelease string
	BTFAvailable  bool
	BPFFSMounted  bool
	EBPFReady     bool
}

func main() {
	endpoint := flag.String(
		"endpoint",
		"http://localhost:8080/v1/events",
		"Gaia ingest endpoint",
	)

	nodeID := flag.String(
		"node-id",
		"",
		"required pseudonymous identifier for this node",
	)

	interval := flag.Duration(
		"interval",
		30*time.Second,
		"how often to emit an aggregated system event",
	)

	flag.Parse()

	if *nodeID == "" {
		log.Fatal("node-id is required")
	}

	if runtime.GOOS != "linux" {
		log.Fatal("the Gaia Linux agent must run on Linux")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	send := func() {
		outgoing, err := systemEvent(*nodeID)
		if err != nil {
			log.Printf("collect system metrics: %v", err)
			return
		}

		if err := submitEvent(client, *endpoint, outgoing); err != nil {
			log.Printf("submit agent event: %v", err)
			return
		}

		log.Printf("submitted system event %s", outgoing.ID)
	}

	send()

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	log.Println("Gaia Linux agent started. Press Ctrl+C to stop.")

	for {
		select {
		case <-ticker.C:
			send()

		case <-stop:
			log.Println("Gaia Linux agent stopped.")
			return
		}
	}
}

func systemEvent(nodeID string) (event, error) {
	load1, load5, load15, err := readLoadAverages()
	if err != nil {
		return event{}, err
	}

	uptimeSeconds, err := readUptimeSeconds()
	if err != nil {
		return event{}, err
	}

	capabilities := detectCapabilities()
	now := time.Now().UTC()

	return event{
		ID:         fmt.Sprintf("evt_agent_system_%s_%d", nodeID, now.UnixNano()),
		Type:       "agent.system",
		ObservedAt: now,
		ReceivedAt: time.Now().UTC(),
		Source: source{
			ID:    nodeID,
			Kind:  "agent",
			Trust: 0.85,
		},
		Data: map[string]any{
			"signal":           "linux-host-health",
			"load1":            load1,
			"load5":            load5,
			"load15":           load15,
			"uptimeSeconds":    uptimeSeconds,
			"cpuCount":         runtime.NumCPU(),
			"kernelRelease":    capabilities.KernelRelease,
			"ebpfBTFAvailable": capabilities.BTFAvailable,
			"bpffsMounted":     capabilities.BPFFSMounted,
			"ebpfReady":        capabilities.EBPFReady,
		},
		Privacy: privacy{
			Level:                "aggregated",
			ContainsPersonalData: false,
		},
	}, nil
}

func readLoadAverages() (float64, float64, float64, error) {
	contents, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}

	return parseLoadAverages(string(contents))
}

func parseLoadAverages(
	contents string,
) (float64, float64, float64, error) {
	fields := strings.Fields(contents)

	if len(fields) < 3 {
		return 0, 0, 0, errors.New("unexpected /proc/loadavg format")
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	load5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	load15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	return load1, load5, load15, nil
}

func readUptimeSeconds() (float64, error) {
	contents, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(contents))
	if len(fields) == 0 {
		return 0, errors.New("unexpected /proc/uptime format")
	}

	return strconv.ParseFloat(fields[0], 64)
}

func detectCapabilities() agentCapabilities {
	kernelRelease := "unknown"

	if contents, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		kernelRelease = strings.TrimSpace(string(contents))
	}

	btfAvailable := pathExists("/sys/kernel/btf/vmlinux")
	bpffsMounted := pathExists("/sys/fs/bpf")

	return agentCapabilities{
		KernelRelease: kernelRelease,
		BTFAvailable:  btfAvailable,
		BPFFSMounted:  bpffsMounted,
		EBPFReady:     isEBPFReady(btfAvailable, bpffsMounted),
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isEBPFReady(btfAvailable bool, bpffsMounted bool) bool {
	return btfAvailable && bpffsMounted
}

func submitEvent(
	client *http.Client,
	endpoint string,
	outgoing event,
) error {
	payload, err := json.Marshal(outgoing)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(
		http.MethodPost,
		endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("ingest service returned %s", response.Status)
	}

	return nil
}
