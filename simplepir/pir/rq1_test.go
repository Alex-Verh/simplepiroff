package pir

import (
    "bufio"
    "encoding/binary"
    "encoding/csv"
    "fmt"
    "io"
    "math"
    "os"
    "regexp"
    "strconv"
    "strings"
    "testing"
    "sort"
)

func QueryProductByID(t *testing.T, productID uint64, DBSize uint64, recordSize uint64) {
    pir := SimplePIR{}
    txtPath := "../../db/database.txt"
    binPath := "../../db/database.bin"
    
    var limitedVals []uint64
    var actualRecordSize uint64
    var actualDBSize uint64
    var err error
    
    if _, err := os.Stat(binPath); err == nil {
        fmt.Println("Using binary database file for faster loading")
        limitedVals, actualRecordSize, actualDBSize, err = LoadValuesFromBinary(binPath, DBSize)
        if err != nil {
            fmt.Printf("Warning: Failed to load binary database: %v\nFalling back to text file.\n", err)
        }
    }
    
    if limitedVals == nil {
        fmt.Println("Using text database file")
        limitedVals, actualRecordSize, actualDBSize, err = LoadValuesFromText(txtPath, DBSize)
        if err != nil {
            t.Fatalf("Failed to load database: %v", err)
        }
    }
    
    fmt.Printf("Database has %d total entries, using %d entries for this test\n", 
        actualDBSize, len(limitedVals))

    recordSizeToUse := actualRecordSize
    if recordSize > 0 {
        fmt.Printf("Using specified record size: %d bits (auto-detected: %d bits)\n",
                   recordSize, actualRecordSize)
        recordSizeToUse = recordSize
        
        if recordSize < actualRecordSize {
            maxValueForRecordSize := uint64((1 << recordSizeToUse) - 1)
            
            for i := range limitedVals {
                limitedVals[i] = limitedVals[i] % (maxValueForRecordSize + 1)
            }
            
            fmt.Printf("Values adjusted to fit in %d bits (max possible value: %d)\n",
                      recordSizeToUse, maxValueForRecordSize)
        }
    }
    
    dbSizeToUse := uint64(len(limitedVals))
    
    p := pir.PickParams(dbSizeToUse, recordSizeToUse, SEC_PARAM, LOGQ)
    DB := MakeDB(dbSizeToUse, recordSizeToUse, &p, limitedVals)
    
    var queryIndex uint64
    var found bool = false
    
    for i := uint64(0); i < dbSizeToUse; i++ {
        v := DB.GetElem(i)
        if v == productID {
            queryIndex = i
            found = true
            fmt.Printf("Found product ID %d at index %d\n", productID, i)
            break
        }
    }
    
    if !found {
        fmt.Printf("Product ID %d not found in the database\n", productID)
        return
    }
    
    fmt.Printf("Running PIR query for product ID %d at index %d...\n", productID, queryIndex)
    RunPIR(&pir, DB, p, []uint64{queryIndex})
    fmt.Printf("Successfully retrieved product ID %d\n", productID)
}

func AutoDetectRowLength(dbPath string) (uint64, uint64) {
    file, err := os.Open(dbPath)
    if err != nil {
        panic(fmt.Sprintf("Error opening database file: %v", err))
    }
    defer file.Close()
    
    scanner := bufio.NewScanner(file)
    var maxVal uint64 = 0
	var entryCount uint64 = 0    

    for scanner.Scan() {
        entryCount++
        line := strings.TrimSpace(scanner.Text())
        if line == "" {
            continue
        }
        
        fields := strings.FieldsFunc(line, func(r rune) bool {
            return r == ':' || r == '\t' || r == ' '
        })
        
        if len(fields) > 0 {
            if val, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
                if val > maxVal {
                    maxVal = val
                }
            }
        }
    }
    
    var rowLength uint64 = 32
    if maxVal > 0 {
        rowLength = uint64(math.Ceil(math.Log2(float64(maxVal + 1))))
        rowLength = ((rowLength + 7) / 8) * 8
    }
    
    fmt.Printf("Auto-detected row length: %d bits (max value: %d, entries: %d)\n", 
               rowLength, maxVal, entryCount)
    
    return rowLength, entryCount
}

// PRODUCT_ID=63 go test -run=TestQueryProduct
// test using whole DB size find product ID
func TestQueryProduct(t *testing.T) {
    productID := uint64(0)

 	if idStr := os.Getenv("PRODUCT_ID"); idStr != "" {
        if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
            productID = id
        } else {
            fmt.Printf("Warning: Invalid PRODUCT_ID value '%s', using default 54\n", idStr)
        }
    }
    
    fmt.Printf("Querying for product ID: %d\n", productID)
    QueryProductByID(t, productID, 0, 0)
}

// go test -run=TestPIRWithDifferentDBSizes
// test using different DB sizes
func TestPIRWithDifferentDBSizes(t *testing.T) {
    dbSizes := []uint64{1, 10, 100, 1000, 10000, 100000, 1000000}
    productID := uint64(54) // first product ID in the database
    
    for _, dbSize := range dbSizes {
        fmt.Printf("\n\n==== Testing PIR with %d entries ====\n", dbSize)
        
        output := CaptureOutput(func() {
            QueryProductByID(t, productID, dbSize, 0)
        })
        
        metrics := ExtractPIRMetrics(output)
        
        params := map[string]string{
            "db_size": fmt.Sprintf("%d", dbSize),
            "record_size": "auto",
        }
        
        LogTestResults("dbsize", params, metrics)
    }
}

// go test -run=TestPIRWithDifferentRecordSizes
// test using different record sizes
func TestPIRWithDifferentRecordSizes(t *testing.T) {
    recordSizes := []uint64{8, 16, 32, 64, 128, 256, 512, 1024}
    productID := uint64(54) // first product ID in the database
    
    for _, recordSize := range recordSizes {
        fmt.Printf("\n\n==== Testing PIR with record size %d bits ====\n", recordSize)

        output := CaptureOutput(func() {
            QueryProductByID(t, productID, 0, recordSize)
        })
        
        metrics := ExtractPIRMetrics(output)
        
        params := map[string]string{
            "db_size": "auto", 
            "record_size": fmt.Sprintf("%d", recordSize),
        }

        LogTestResults("recordsize", params, metrics)
    }
}

// go test -run=TestPIRWithSizeCombinations
// test using different record sizes and DB sizes
func TestPIRWithSizeCombinations(t *testing.T) {
    dbSizes := []uint64{1, 10, 100, 1000, 10000, 100000, 1000000}
    recordSizes := []uint64{8, 16, 32, 64, 128, 256, 512, 1024}
    productID := uint64(54) // first product ID in the database

    for _, dbSize := range dbSizes {
        for _, recordSize := range recordSizes {
            fmt.Printf("\n\n==== Testing PIR with DB size %d and record size %d bits ====\n", dbSize, recordSize)

            output := CaptureOutput(func() {
                QueryProductByID(t, productID, dbSize, recordSize)
            })
            
            metrics := ExtractPIRMetrics(output)
            
            params := map[string]string{
                "db_size": fmt.Sprintf("%d", dbSize),
                "record_size": fmt.Sprintf("%d", recordSize),
            }

            LogTestResults("db_recordsize", params, metrics)
        }
    }
}

//  LOADING DATABASE FUNCTIONS -----------------------------------------------------------------------------------------------

func LoadValuesFromText(txtPath string, limit uint64) ([]uint64, uint64, uint64, error) {
    rowLength, totalCount := AutoDetectRowLength(txtPath)
    
    readCount := totalCount
    if limit > 0 && limit < totalCount {
        readCount = limit
        fmt.Printf("Testing with reduced DB size: %d entries (of %d total)\n", 
                  readCount, totalCount)
    }
    
    file, err := os.Open(txtPath)
    if err != nil {
        return nil, 0, 0, fmt.Errorf("error opening text file: %v", err)
    }
    defer file.Close()
    
    scanner := bufio.NewScanner(file)
    var values []uint64
    count := uint64(0)
    
    for scanner.Scan() && count < readCount {
        line := strings.TrimSpace(scanner.Text())
        if line == "" {
            continue
        }
        
        fields := strings.FieldsFunc(line, func(r rune) bool {
            return r == ':' || r == '\t' || r == ' '
        })
        
        if len(fields) > 0 {
            if val, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
                values = append(values, val)
                count++
            }
        }
    }
    
    fmt.Printf("Text loading completed: %d values\n", len(values))
    return values, rowLength, totalCount, nil
}

func LoadValuesFromBinary(binPath string, limit uint64) ([]uint64, uint64, uint64, error) {
    file, err := os.Open(binPath)
    if err != nil {
        return nil, 0, 0, fmt.Errorf("error opening binary file: %v", err)
    }
    defer file.Close()
    
    var totalCount uint64
    err = binary.Read(file, binary.LittleEndian, &totalCount)
    if err != nil {
        return nil, 0, 0, fmt.Errorf("error reading count: %v", err)
    }
    
    readCount := totalCount
    if limit > 0 && limit < totalCount {
        readCount = limit
        fmt.Printf("Testing with reduced DB size: %d entries (of %d total)\n", 
                  readCount, totalCount)
    }
    
    values := make([]uint64, readCount)
    
    for i := uint64(0); i < readCount; i++ {
        err = binary.Read(file, binary.LittleEndian, &values[i])
        if err != nil {
            return nil, 0, 0, fmt.Errorf("error reading value at index %d: %v", i, err)
        }
        
        if i > 0 && i%1000000 == 0 {
            fmt.Printf("Loaded %d/%d values (%.1f%%)...\n", i, readCount, float64(i)/float64(readCount)*100)
        }
    }

    var maxVal uint64 = 0
    for _, val := range values {
        if val > maxVal {
            maxVal = val
        }
    }
    
    var rowLength uint64 = 32
    if maxVal > 0 {
        rowLength = uint64(math.Ceil(math.Log2(float64(maxVal + 1))))
        rowLength = ((rowLength + 7) / 8) * 8 
    }
    
    fmt.Printf("Binary loading completed: %d values (max value: %d, row length: %d bits)\n", 
              len(values), maxVal, rowLength)
    
    return values, rowLength, totalCount, nil
}


// CAPTURING AND EXTRACTION OUTPUT FUNCTIONS -----------------------------------------------------------------------------

func CaptureOutput(f func()) string {
    old := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w
    
    f() 
    
    w.Close()
    os.Stdout = old
    
    var buf strings.Builder
    io.Copy(&buf, r)

    fmt.Print(buf.String())

    return buf.String()
}

func ExtractPIRMetrics(output string) map[string]float64 {
    // Initialize all expected metrics with zero values
    metrics := map[string]float64{
        "setup_time":       0.0,
        "query_time":       0.0,
        "answer_time":      0.0,
        "reconstruct_time": 0.0,
        "offline_download": 0.0,
        "online_upload":    0.0,
        "online_download":  0.0,
    }
    
    // Time metrics regex patterns
    timePatterns := map[string]string{
        "setup_time":       `Setup\.\.\.\s+Elapsed: ([\d\.]+)(µs|ms|s)`,
        "query_time":       `Building query\.\.\.\s+Elapsed: ([\d\.]+)(µs|ms|s)`,
        "answer_time":      `Answering query\.\.\.\s+Elapsed: ([\d\.]+)(µs|ms|s)`,
        "reconstruct_time": `Reconstructing\.\.\.\s+Success!\s+Elapsed: ([\d\.]+)(µs|ms|s)`,
    }
    
    // Data transfer metrics regex patterns
    dataPatterns := map[string]string{
        "offline_download": `Offline download: ([\d\.]+) KB`,
        "online_upload":    `Online upload: ([\d\.]+) KB`,
        "online_download":  `Online download: ([\d\.]+) KB`,
    }
    
    convertToMs := func(value float64, unit string) float64 {
        switch unit {
        case "µs": return value / 1000
        case "ms": return value
        case "s":  return value * 1000
        default:   return value
        }
    }
    
    // Keep track of which metrics were found
    metricsFound := make(map[string]bool)
    
    // Extract time metrics (they have units like µs, ms, s)
    for name, pattern := range timePatterns {
        re := regexp.MustCompile(pattern)
        if matches := re.FindStringSubmatch(output); len(matches) >= 3 {
            value, _ := strconv.ParseFloat(matches[1], 64)
            metrics[name] = convertToMs(value, matches[2])
            metricsFound[name] = true
        }
    }
    
    // Extract data transfer metrics (they are in KB)
    for name, pattern := range dataPatterns {
        re := regexp.MustCompile(pattern)
        if matches := re.FindStringSubmatch(output); len(matches) >= 2 {
            value, _ := strconv.ParseFloat(matches[1], 64)
            metrics[name] = value
            metricsFound[name] = true
        }
    }
    
    // Debug: print warning for missing metrics
    for name := range metrics {
        if !metricsFound[name] {
            fmt.Printf("Warning: Metric '%s' not found in output\n", name)
        }
    }
    
    return metrics
}

// LOGGING RESULTS FUNCTION --------------------------------------------------------------------------------------------------

func LogTestResults(testName string, params map[string]string, metrics map[string]float64) {
    os.MkdirAll("../../results", 0755)
    filename := fmt.Sprintf("../../results/%s_results.csv", testName)
    
    fileExists := false
    if _, err := os.Stat(filename); err == nil {
        fileExists = true
    }
    
    file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        fmt.Printf("Warning: Failed to log results: %v\n", err)
        return
    }
    defer file.Close()
    
    writer := csv.NewWriter(file)
    defer writer.Flush()
    
    var paramKeys []string
    var metricKeys []string
    
    for k := range params {
        paramKeys = append(paramKeys, k)
    }
    sort.Strings(paramKeys) 
    
    for k := range metrics {
        metricKeys = append(metricKeys, k)
    }
    sort.Strings(metricKeys)
    
    if !fileExists {
        var header []string
        header = append(header, paramKeys...)
        header = append(header, metricKeys...)
        writer.Write(header)
    }
    
    var row []string
    
    for _, k := range paramKeys {
        row = append(row, params[k])
    }
    
    for _, k := range metricKeys {
        row = append(row, fmt.Sprintf("%.6f", metrics[k]))
    }
    
    writer.Write(row)
}

// RUNNING THE TESTS MULTIPLE TIMES------------------------------------------------------------------------------------------------------------
func RunTestMultipleTimes(t *testing.T, runCount int, testFunc func() (map[string]string, map[string]float64)) {
    if runCount <= 0 {
        runCount = 1
    }
    
    allMetrics := make([]map[string]float64, runCount)
    var params map[string]string
    
    fmt.Printf("\n===== Run 1 of %d =====\n", runCount)
    p, metrics := testFunc()
    allMetrics[0] = metrics
    params = p // Params should be the same for all runs
    
    expectedMetrics := make(map[string]bool)
    for key := range metrics {
        expectedMetrics[key] = true
    }
    
    for i := 1; i < runCount; i++ {
        fmt.Printf("\n===== Run %d of %d =====\n", i+1, runCount)
        _, metrics := testFunc()
        
        completeMetrics := make(map[string]float64)
        for key := range expectedMetrics {
            if value, exists := metrics[key]; exists {
                completeMetrics[key] = value
            } else {
                completeMetrics[key] = 0.0
                fmt.Printf("Warning: Metric '%s' not found in run %d, using 0.0\n", key, i+1)
            }
        }
        
        allMetrics[i] = completeMetrics
    }
    
    avgMetrics := make(map[string]float64)
    for key := range expectedMetrics {
        sum := 0.0
        count := 0
        for i := 0; i < runCount; i++ {
            if value, exists := allMetrics[i][key]; exists {
                sum += value
                count++
            }
        }
        if count > 0 {
            avgMetrics[key] = sum / float64(count)
        }
    }
    
    params["run_count"] = fmt.Sprintf("%d", runCount)
    
    testType := "custom"
    if _, hasDBSize := params["db_size"]; hasDBSize {
        if _, hasRecordSize := params["record_size"]; hasRecordSize {
            if params["record_size"] == "auto" {
                testType = "dbsize"
            } else if params["db_size"] == "auto" {
                testType = "recordsize"
            } else {
                testType = "db_recordsize"
            }
        }
    }
    
    LogTestResults(fmt.Sprintf("%s_avg", testType), params, avgMetrics)
    
    for i := 0; i < runCount; i++ {
        runParams := make(map[string]string)
        for k, v := range params {
            runParams[k] = v
        }
        runParams["run_id"] = fmt.Sprintf("%d", i+1)
        LogTestResults(fmt.Sprintf("%s_runs", testType), runParams, allMetrics[i])
    }
}

// RUNS=10 go test -run=TestPIRWithDifferentDBSizesMultiRun
func TestPIRWithDifferentDBSizesMultiRun(t *testing.T) {
    dbSizes := []uint64{1, 10, 100, 1000, 10000, 100000, 1000000}
    productID := uint64(54) // first product ID in the database
    
    runCount := 3 // default
    if countStr := os.Getenv("RUNS"); countStr != "" {
        if count, err := strconv.Atoi(countStr); err == nil && count > 0 {
            runCount = count
        }
    }
    
    for _, dbSize := range dbSizes {
        fmt.Printf("\n\n==== Testing PIR with %d entries (%d runs) ====\n", dbSize, runCount)
        
        RunTestMultipleTimes(t, runCount, func() (map[string]string, map[string]float64) {
            output := CaptureOutput(func() {
                QueryProductByID(t, productID, dbSize, 0)
            })
            
            metrics := ExtractPIRMetrics(output)
            
            params := map[string]string{
                "db_size": fmt.Sprintf("%d", dbSize),
                "record_size": "auto",
            }
            
            return params, metrics
        })
    }
}

// RUNS=10 go test -run=TestPIRWithDifferentRecordSizesMultiRun
func TestPIRWithDifferentRecordSizesMultiRun(t *testing.T) {
    recordSizes := []uint64{8, 16, 32, 64, 128, 256, 512}
    productID := uint64(54) // first product ID in the database
    
    runCount := 3 // default
    if countStr := os.Getenv("RUNS"); countStr != "" {
        if count, err := strconv.Atoi(countStr); err == nil && count > 0 {
            runCount = count
        }
    }
    
    for _, recordSize := range recordSizes {
        fmt.Printf("\n\n==== Testing PIR with record size %d bits (%d runs) ====\n", recordSize, runCount)
        
        RunTestMultipleTimes(t, runCount, func() (map[string]string, map[string]float64) {
            output := CaptureOutput(func() {
                QueryProductByID(t, productID, 0, recordSize)
            })
            
            metrics := ExtractPIRMetrics(output)
            
            params := map[string]string{
                "db_size": "auto", 
                "record_size": fmt.Sprintf("%d", recordSize),
            }
            
            return params, metrics
        })
    }
}

// RUNS=5 go test -run=TestPIRWithSizeCombinationsMultiRun
func TestPIRWithSizeCombinationsMultiRun(t *testing.T) {
    dbSizes := []uint64{1, 10, 100, 1000, 10000, 100000, 1000000}
    recordSizes := []uint64{8, 16, 32, 64, 128, 256, 512}
    productID := uint64(54) // first product ID in the database

    runCount := 3 // default
    if countStr := os.Getenv("RUNS"); countStr != "" {
        if count, err := strconv.Atoi(countStr); err == nil && count > 0 {
            runCount = count
        }
    }
    
    for _, dbSize := range dbSizes {
        for _, recordSize := range recordSizes {
            fmt.Printf("\n\n==== Testing PIR with DB size %d and record size %d bits (%d runs) ====\n", 
                      dbSize, recordSize, runCount)
            
            RunTestMultipleTimes(t, runCount, func() (map[string]string, map[string]float64) {
                output := CaptureOutput(func() {
                    QueryProductByID(t, productID, dbSize, recordSize)
                })
                
                metrics := ExtractPIRMetrics(output)
                
                params := map[string]string{
                    "db_size": fmt.Sprintf("%d", dbSize),
                    "record_size": fmt.Sprintf("%d", recordSize),
                }
                
                return params, metrics
            })
        }
    }
}