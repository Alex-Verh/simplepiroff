package main

import (
    "encoding/csv"
    "fmt"
    "log"
    "os"
    "strconv"
    "strings"
)

type NetworkScenario struct {
    Name      string
    Bandwidth float64 // Mbps
    Latency   float64 // ms
}

type PIRResult struct {
    DBSize           string
    RecordSize       string
    OfflineDownload  float64 // KB
    OnlineDownload   float64 // KB
    OnlineUpload     float64 // KB
}

func main() {
    // Define network scenarios
    scenarios := []NetworkScenario{
        {"Good Network (300 Mbps, 10ms)", 300.0, 10.0},
        {"Average Network (100 Mbps, 50ms)", 100.0, 50.0},
        {"Poor Network (25 Mbps, 200ms)", 25.0, 200.0},
    }

    // Process both CSV files
    files := []string{
        "results/recordsize_avg_results.csv",
        "results/dbsize_avg_results.csv",
    }

    for _, filename := range files {
        // Read and parse CSV
        results, err := readPIRResults(filename)
        if err != nil {
            log.Printf("Error reading %s: %v", filename, err)
            continue
        }

        // Generate analysis file
        generateDetailedAnalysis(results, scenarios, filename)
    }
}

func readPIRResults(filename string) ([]PIRResult, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {
        return nil, err
    }

    if len(records) < 2 {
        return nil, fmt.Errorf("CSV file must have at least header and one data row")
    }

    // Parse header to find column indices
    header := records[0]
    colMap := make(map[string]int)
    for i, col := range header {
        colMap[col] = i
    }

    var results []PIRResult
    for i := 1; i < len(records); i++ {
        row := records[i]
        
        result := PIRResult{
            DBSize:     row[colMap["db_size"]],
            RecordSize: row[colMap["record_size"]],
        }


        if val, err := strconv.ParseFloat(row[colMap["offline_download"]], 64); err == nil {
            result.OfflineDownload = val
        }
        if val, err := strconv.ParseFloat(row[colMap["online_download"]], 64); err == nil {
            result.OnlineDownload = val
        }
        if val, err := strconv.ParseFloat(row[colMap["online_upload"]], 64); err == nil {
            result.OnlineUpload = val
        }

        results = append(results, result)
    }

    return results, nil
}

func calculateNetworkTime(result PIRResult, scenario NetworkScenario) float64 {
    totalDataKb := (result.OfflineDownload + result.OnlineDownload + result.OnlineUpload) * 8 // KB to Kb
    totalDataMb := totalDataKb / 1000 // Kb to Mb

    latencySeconds := scenario.Latency / 1000 // ms to s

    dataTransferTime := totalDataMb / scenario.Bandwidth
    networkTime := 3*latencySeconds + dataTransferTime

    return networkTime
}

func generateDetailedAnalysis(results []PIRResult, scenarios []NetworkScenario, filename string) {
    baseName := strings.TrimPrefix(filename, "results/")
    baseName = strings.TrimSuffix(baseName, ".csv")
    outputFile := fmt.Sprintf("results/%s_network_analysis.csv", baseName)
    
    file, err := os.Create(outputFile)
    if err != nil {
        log.Printf("Error creating output file: %v", err)
        return
    }
    defer file.Close()

    writer := csv.NewWriter(file)
    defer writer.Flush()

    header := []string{
        "db_size", "record_size",
        "offline_download_kb", "online_download_kb", "online_upload_kb",
        "good_network_time_s", "average_network_time_s", "poor_network_time_s",
    }
    writer.Write(header)

    for _, result := range results {
        networkTimes := make([]float64, len(scenarios))
        
        for i, scenario := range scenarios {
            networkTimes[i] = calculateNetworkTime(result, scenario)
        }

        row := []string{
            result.DBSize,
            result.RecordSize,
            fmt.Sprintf("%.0f", result.OfflineDownload),
            fmt.Sprintf("%.0f", result.OnlineDownload),
            fmt.Sprintf("%.0f", result.OnlineUpload),
            fmt.Sprintf("%.3f", networkTimes[0]),
            fmt.Sprintf("%.3f", networkTimes[1]),
            fmt.Sprintf("%.3f", networkTimes[2]),
        }
        writer.Write(row)
    }
}