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
	statsURL        = "http://srv.msk01.gigacorp.local/_stats"
	maxErrorCount   = 3
	pollInterval    = time.Second // период опроса
	loadAvgLimit    = 30.0
	memoryLimitPct  = 80.0
	diskLimitPct    = 90.0
	networkLimitPct = 90.0
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
			// после успешного запроса счётчик ошибок обнуляем
			errorCount = 0
		}

		if errorCount >= maxErrorCount {
			fmt.Println("Unable to fetch server statistic")
			return
		}
	}
}

// pollOnce выполняет один запрос к серверу и выводит
// предупреждающие сообщения при превышении порогов.
// Возвращает true, если данные были корректно получены и разобраны.
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

	// Парсим все значения как float64 — нам нужны деления и проценты.
	vals := make([]float64, 7)
	for i, p := range parts {
		p = strings.TrimSpace(p)
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return false
		}
		vals[i] = v
	}

	loadAvg := vals[0]

	memTotal := vals[1]
	memUsed := vals[2]

	diskTotal := vals[3]
	diskUsed := vals[4]

	netTotal := vals[5]
	netUsed := vals[6]

	// Проверка на нулевые/отрицательные ресурсы — это тоже ошибка данных
	if memTotal <= 0 || diskTotal <= 0 || netTotal <= 0 {
		return false
	}

	// 1. Load Average
	if loadAvg > loadAvgLimit {
		fmt.Printf("Load Average is too high: %g\n", loadAvg)
	}

	// 2. Использование оперативной памяти (в процентах)
	if memTotal > 0 {
		// считаем процент использования памяти
		memUsagePct := int(memUsed * 100.0 / memTotal)

		// по условию: при превышении 80%
		if memUsagePct > 80 {
			fmt.Printf("Memory usage too high: %d%%\n", memUsagePct)
		}
	}

	// 3. Свободное дисковое пространство
	diskUsagePct := diskUsed * 100.0 / diskTotal
	if diskUsagePct > diskLimitPct {
		freeBytes := diskTotal - diskUsed
		if freeBytes < 0 {
			freeBytes = 0
		}
		freeMb := int(freeBytes / (1024 * 1024)) // мегабайты
		fmt.Printf("Free disk space is too low: %d Mb left\n", freeMb)
	}

	// 4. Загруженность сети
	netUsagePct := netUsed * 100.0 / netTotal
	if netUsagePct > networkLimitPct {
		freeBytesPerSec := netTotal - netUsed
		if freeBytesPerSec < 0 {
			freeBytesPerSec = 0
		}
		// Переводим байты/с -> мегабиты/с:
		// bytes * 8 / (1024*1024) ≈ Mbit/s
		freeMbitPerSec := int((freeBytesPerSec * 8.0) / (1024.0 * 1024.0))
		fmt.Printf("Network bandwidth usage high: %d Mbit/s available\n", freeMbitPerSec)
	}

	return true
}
