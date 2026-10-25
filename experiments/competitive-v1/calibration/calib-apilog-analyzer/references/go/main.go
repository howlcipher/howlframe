package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

var allowedMethods = map[string]bool{
	"GET":     true,
	"POST":    true,
	"PUT":     true,
	"DELETE":  true,
	"PATCH":   true,
	"HEAD":    true,
	"OPTIONS": true,
}

type Summary struct {
	TotalRecords          int    `json:"total_records"`
	ValidRecords          int    `json:"valid_records"`
	InvalidRecords        int    `json:"invalid_records"`
	EarliestTimestamp     *int64 `json:"earliest_timestamp"`
	LatestTimestamp       *int64 `json:"latest_timestamp"`
	TotalBytesSent        int64  `json:"total_bytes_sent"`
	TotalServices         int    `json:"total_services"`
	ServicesWithSlaBreach int    `json:"services_with_sla_breach"`
}

type ServiceMetric struct {
	Service             string  `json:"service"`
	TotalRequests       int     `json:"total_requests"`
	SuccessfulRequests  int     `json:"successful_requests"`
	ClientErrorRequests int     `json:"client_error_requests"`
	ServerErrorRequests int     `json:"server_error_requests"`
	ErrorRate           float64 `json:"error_rate"`
	MinLatencyMs        int64   `json:"min_latency_ms"`
	MaxLatencyMs        int64   `json:"max_latency_ms"`
	AvgLatencyMs        float64 `json:"avg_latency_ms"`
	TotalBytesSent      int64   `json:"total_bytes_sent"`
	SlaBreach           bool    `json:"sla_breach"`
}

type ClientMetric struct {
	ClientID     string `json:"client_id"`
	RequestCount int    `json:"request_count"`
	TotalBytes   int64  `json:"total_bytes"`
	ErrorCount   int    `json:"error_count"`
}

type WindowMetric struct {
	WindowStart   int64   `json:"window_start"`
	WindowEnd     int64   `json:"window_end"`
	TotalRequests int     `json:"total_requests"`
	ErrorRequests int     `json:"error_requests"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
}

type Report struct {
	Summary    Summary         `json:"summary"`
	Services   []ServiceMetric `json:"services"`
	TopClients []ClientMetric  `json:"top_clients"`
	Windows    []WindowMetric  `json:"windows"`
}

type serviceAcc struct {
	total      int
	success    int
	clientErr  int
	serverErr  int
	sumLatency int64
	minLatency int64
	maxLatency int64
	totalBytes int64
}

type clientAcc struct {
	count      int
	totalBytes int64
	errors     int
}

type windowAcc struct {
	total      int
	errors     int
	sumLatency int64
}

func roundTo(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}

func main() {
	var inputPath string
	var outputPath string
	slaLatencyMs := int64(500)
	errorRateThreshold := 0.05
	windowSeconds := int64(300)

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		var key, val string

		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			key = parts[0]
			val = parts[1]
		} else if strings.HasPrefix(arg, "--") {
			key = arg
			if i+1 < len(args) {
				i++
				val = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "Error: Missing value for option %s\n", key)
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: Unrecognized argument: %s\n", arg)
			os.Exit(1)
		}

		switch key {
		case "--input":
			inputPath = val
		case "--output":
			outputPath = val
		case "--sla-latency-ms":
			n, err := strconv.ParseInt(val, 10, 64)
			if err != nil || n < 0 {
				fmt.Fprintf(os.Stderr, "Error: --sla-latency-ms must be a non-negative integer, got '%s'\n", val)
				os.Exit(1)
			}
			slaLatencyMs = n
		case "--error-rate-threshold":
			f, err := strconv.ParseFloat(val, 64)
			if err != nil || math.IsNaN(f) || f < 0.0 || f > 1.0 {
				fmt.Fprintf(os.Stderr, "Error: --error-rate-threshold must be between 0.0 and 1.0, got '%s'\n", val)
				os.Exit(1)
			}
			errorRateThreshold = f
		case "--window-seconds":
			n, err := strconv.ParseInt(val, 10, 64)
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "Error: --window-seconds must be a positive integer, got '%s'\n", val)
				os.Exit(1)
			}
			windowSeconds = n
		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown option %s\n", key)
			os.Exit(1)
		}
	}

	var inReader io.Reader = os.Stdin
	if inputPath != "" && inputPath != "-" {
		fi, err := os.Stat(inputPath)
		if err != nil || fi.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: Cannot open input file '%s': does not exist, is a directory, or cannot be read\n", inputPath)
			os.Exit(2)
		}
		file, err := os.Open(inputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Cannot open input file '%s': %v\n", inputPath, err)
			os.Exit(2)
		}
		defer file.Close()
		inReader = file
	}

	var outWriter io.Writer = os.Stdout
	if outputPath != "" && outputPath != "-" {
		file, err := os.Create(outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Cannot open output file '%s': %v\n", outputPath, err)
			os.Exit(1)
		}
		defer file.Close()
		outWriter = file
	}

	scanner := bufio.NewScanner(inReader)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	totalRecords := 0
	validRecords := 0
	invalidRecords := 0
	var earliestTimestamp *int64
	var latestTimestamp *int64
	var totalBytesSent int64

	services := make(map[string]*serviceAcc)
	clients := make(map[string]*clientAcc)
	windows := make(map[int64]*windowAcc)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		totalRecords++

		var rawMap map[string]interface{}
		err := json.Unmarshal([]byte(trimmed), &rawMap)
		if err != nil || rawMap == nil {
			invalidRecords++
			continue
		}

		tsF, okTs := rawMap["timestamp"].(float64)
		svsS, okSvc := rawMap["service"].(string)
		cidS, okCid := rawMap["client_id"].(string)
		mthS, okMth := rawMap["method"].(string)
		pthS, okPth := rawMap["path"].(string)
		stF, okSt := rawMap["status"].(float64)
		latF, okLat := rawMap["latency_ms"].(float64)
		bytF, okByt := rawMap["bytes_sent"].(float64)

		const maxSafeInteger = float64(1<<53 - 1)

		if !okTs || math.IsNaN(tsF) || math.IsInf(tsF, 0) || tsF < 0 || tsF > maxSafeInteger || tsF != math.Floor(tsF) ||
			!okSvc || len(svsS) == 0 ||
			!okCid || len(cidS) == 0 ||
			!okMth || !allowedMethods[mthS] ||
			!okPth || !strings.HasPrefix(pthS, "/") ||
			!okSt || math.IsNaN(stF) || math.IsInf(stF, 0) || stF < 100 || stF > 599 || stF != math.Floor(stF) ||
			!okLat || math.IsNaN(latF) || math.IsInf(latF, 0) || latF < 0 || latF > maxSafeInteger || latF != math.Floor(latF) ||
			!okByt || math.IsNaN(bytF) || math.IsInf(bytF, 0) || bytF < 0 || bytF > maxSafeInteger || bytF != math.Floor(bytF) {
			invalidRecords++
			continue
		}

		ts := int64(tsF)
		status := int(stF)
		lat := int64(latF)
		bytesSent := int64(bytF)

		validRecords++

		if earliestTimestamp == nil || ts < *earliestTimestamp {
			earliestCopy := ts
			earliestTimestamp = &earliestCopy
		}
		if latestTimestamp == nil || ts > *latestTimestamp {
			latestCopy := ts
			latestTimestamp = &latestCopy
		}
		totalBytesSent += bytesSent

		// Service aggregation
		s, ok := services[svsS]
		if !ok {
			s = &serviceAcc{minLatency: lat, maxLatency: lat}
			services[svsS] = s
		}
		s.total++
		if status >= 200 && status < 400 {
			s.success++
		} else if status >= 400 && status < 500 {
			s.clientErr++
		} else if status >= 500 {
			s.serverErr++
		}
		s.sumLatency += lat
		if lat < s.minLatency {
			s.minLatency = lat
		}
		if lat > s.maxLatency {
			s.maxLatency = lat
		}
		s.totalBytes += bytesSent

		// Client aggregation
		c, ok := clients[cidS]
		if !ok {
			c = &clientAcc{}
			clients[cidS] = c
		}
		c.count++
		c.totalBytes += bytesSent
		if status >= 400 {
			c.errors++
		}

		// Window aggregation
		wStart := ts - (ts % windowSeconds)
		w, ok := windows[wStart]
		if !ok {
			w = &windowAcc{}
			windows[wStart] = w
		}
		w.total++
		if status >= 400 {
			w.errors++
		}
		w.sumLatency += lat
	}

	// Service metrics
	var serviceNames []string
	for name := range services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)

	servicesList := make([]ServiceMetric, 0, len(serviceNames))
	slaBreachCount := 0
	for _, name := range serviceNames {
		s := services[name]
		errCount := s.clientErr + s.serverErr
		errRate := 0.0
		if s.total > 0 {
			errRate = roundTo(float64(errCount)/float64(s.total), 4)
		}
		avgLat := 0.0
		if s.total > 0 {
			avgLat = roundTo(float64(s.sumLatency)/float64(s.total), 2)
		}
		breach := (errRate > errorRateThreshold) || (avgLat > float64(slaLatencyMs))
		if breach {
			slaBreachCount++
		}
		servicesList = append(servicesList, ServiceMetric{
			Service:             name,
			TotalRequests:       s.total,
			SuccessfulRequests:  s.success,
			ClientErrorRequests: s.clientErr,
			ServerErrorRequests: s.serverErr,
			ErrorRate:           errRate,
			MinLatencyMs:        s.minLatency,
			MaxLatencyMs:        s.maxLatency,
			AvgLatencyMs:        avgLat,
			TotalBytesSent:      s.totalBytes,
			SlaBreach:           breach,
		})
	}

	// Top clients
	var clientMetrics []ClientMetric
	for cid, c := range clients {
		clientMetrics = append(clientMetrics, ClientMetric{
			ClientID:     cid,
			RequestCount: c.count,
			TotalBytes:   c.totalBytes,
			ErrorCount:   c.errors,
		})
	}
	sort.Slice(clientMetrics, func(i, j int) bool {
		if clientMetrics[i].RequestCount != clientMetrics[j].RequestCount {
			return clientMetrics[i].RequestCount > clientMetrics[j].RequestCount
		}
		return clientMetrics[i].ClientID < clientMetrics[j].ClientID
	})
	topClients := make([]ClientMetric, 0)
	for i := 0; i < len(clientMetrics) && i < 10; i++ {
		topClients = append(topClients, clientMetrics[i])
	}

	// Windows
	var windowStarts []int64
	for start := range windows {
		windowStarts = append(windowStarts, start)
	}
	sort.Slice(windowStarts, func(i, j int) bool {
		return windowStarts[i] < windowStarts[j]
	})
	windowList := make([]WindowMetric, 0, len(windowStarts))
	for _, start := range windowStarts {
		w := windows[start]
		avgLat := 0.0
		if w.total > 0 {
			avgLat = roundTo(float64(w.sumLatency)/float64(w.total), 2)
		}
		windowList = append(windowList, WindowMetric{
			WindowStart:   start,
			WindowEnd:     start + windowSeconds,
			TotalRequests: w.total,
			ErrorRequests: w.errors,
			AvgLatencyMs:  avgLat,
		})
	}

	report := Report{
		Summary: Summary{
			TotalRecords:          totalRecords,
			ValidRecords:          validRecords,
			InvalidRecords:        invalidRecords,
			EarliestTimestamp:     earliestTimestamp,
			LatestTimestamp:       latestTimestamp,
			TotalBytesSent:        totalBytesSent,
			TotalServices:         len(services),
			ServicesWithSlaBreach: slaBreachCount,
		},
		Services:   servicesList,
		TopClients: topClients,
		Windows:    windowList,
	}

	enc := json.NewEncoder(outWriter)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}
