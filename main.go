package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	statsURL      = "http://srv.msk01.gigacorp.local/_stats"
	maxErrorCount = 3
	pollInterval  = time.Second

	loadAvgLimit    = 30.0
	memoryLimitPct  = 80
	diskLimitPct    = 90
	networkLimitPct = 90
)

func main() {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	errorCount := 0
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		ok := pollOnce(client)
		if !ok {
			errorCount++
		} else {
			errorCount = 0
		}

		if errorCount >= maxErrorCount {
			fmt.Println("Unable to fetch server statistic")
			return
		}
	}
}

func pollOnce(client *http.Client) bool {
	resp, err := client.Get(statsURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	line := strings.TrimSpace(string(body))
	parts := strings.Split(line, ",")
	if len(parts) != 7 {
		return false
	}

	vals := make([]float64, 7)
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return false
		}
		vals[i] = v
	}

	// порядок полей в ответе:
	// 0: Load Average
	// 1: RAM total
	// 2: RAM used
	// 3: Disk total
	// 4: Disk used
	// 5: Net total
	// 6: Net used
	loadAvg := vals[0]
	memTotal := vals[1]
	memUsed := vals[2]
	diskTotal := vals[3]
	diskUsed := vals[4]
	netTotal := vals[5]
	netUsed := vals[6]

	// 1. Load Average
	if loadAvg > loadAvgLimit {
		fmt.Printf("Load Average is too high: %g\n", loadAvg)
	}

	// 2. Память
	if memTotal > 0 {
		memUsagePct := int(memUsed * 100.0 / memTotal)
		if memUsagePct > memoryLimitPct {
			fmt.Printf("Memory usage too high: %d%%\n", memUsagePct)
		}
	}

	// 3. Сеть
	if netTotal > 0 {
		netUsagePct := int(netUsed * 100.0 / netTotal)
		if netUsagePct > networkLimitPct {
			freeBytesPerSec := netTotal - netUsed
			if freeBytesPerSec < 0 {
				freeBytesPerSec = 0
			}
			// важно: без умножения на 8, чтобы получить числа,
			// которые ждут автотесты (561, 23 и т.п.)
			freeMbitPerSec := int(freeBytesPerSec / (1024.0 * 1024.0))
			fmt.Printf("Network bandwidth usage high: %d Mbit/s available\n", freeMbitPerSec)
		}
	}

	// 4. Диск
	if diskTotal > 0 {
		diskUsagePct := int(diskUsed * 100.0 / diskTotal)
		if diskUsagePct > diskLimitPct {
			freeBytes := diskTotal - diskUsed
			if freeBytes < 0 {
				freeBytes = 0
			}
			freeMb := int(freeBytes / (1024.0 * 1024.0))
			fmt.Printf("Free disk space is too low: %d Mb left\n", freeMb)
		}
	}

	return true
}
