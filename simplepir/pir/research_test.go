package pir

import (
    "encoding/binary"
    "encoding/gob"
    "encoding/csv"
    "bufio"
    "fmt"
    "io"
    "math"
    "os"
    "regexp"
    "strconv"
    "strings"
    "testing"
    "sort"
    "sync"
    "time"
)

var (
    globalDB *EnhancedDatabase
    globalPirKeys []uint64
    globalColumns []string
    globalActualRecordSize uint64
    globalDBLoaded bool = false
    globalDBMutex sync.Mutex
)

type DatabaseRecord struct {
    Key  uint64
    Data map[string]string
}

type EnhancedDatabase struct {
    Records []DatabaseRecord
    Columns []string
    KeyToIndex map[uint64]int
}

// DATABASE LOADING FUNCTIONS (BOTH CSV AND BINARY) ---------------------------------------------------------------------------------------------
func stringToUint64Hash(s string) uint64 {
    hash := uint64(5381)
    for _, c := range s {
        hash = ((hash << 5) + hash) + uint64(c)
    }
    return hash
}

func LoadEnhancedCSVDatabase(csvPath string, keyColumn string, recordBitLength uint64, limit uint64) (*EnhancedDatabase, []uint64, uint64, error) {
    file, err := os.Open(csvPath)
    if err != nil {
        return nil, nil, 0, fmt.Errorf("error opening CSV file: %v", err)
    }
    defer file.Close()
    
    reader := csv.NewReader(file)
    reader.Comma = '\t'
    reader.LazyQuotes = true
    reader.FieldsPerRecord = -1
    
    header, err := reader.Read()
    if err != nil {
        return nil, nil, 0, fmt.Errorf("error reading header: %v", err)
    }
    
    keyIndex := -1
    for i, col := range header {
        if col == keyColumn {
            keyIndex = i
            break
        }
    }
    
    if keyIndex == -1 {
        return nil, nil, 0, fmt.Errorf("key column '%s' not found", keyColumn)
    }
    
    fmt.Printf("Loading full database with %d columns, using '%s' as PIR key\n", len(header), keyColumn)
    
    db := &EnhancedDatabase{
        Records: make([]DatabaseRecord, 0),
        Columns: header,
        KeyToIndex: make(map[uint64]int),
    }
    
    var pirKeys []uint64
    var maxVal uint64 = 0
    recordCount := uint64(0)
    
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            fmt.Printf("Warning: skipping malformed record at line %d: %v\n", recordCount+2, err)
            continue
        }
        
        if keyIndex >= len(record) {
            continue
        }
        
        var key uint64
        if v, err := strconv.ParseUint(strings.TrimSpace(record[keyIndex]), 10, 64); err == nil {
            key = v
        } else {
            key = stringToUint64Hash(record[keyIndex])
        }
        
        recordData := make(map[string]string)
        for i, value := range record {
            if i < len(header) {
                recordData[header[i]] = value
            }
        }
        
        dbRecord := DatabaseRecord{
            Key:  key,
            Data: recordData,
        }
        
        db.Records = append(db.Records, dbRecord)
        db.KeyToIndex[key] = int(recordCount)
        
        pirKeys = append(pirKeys, key)
        if key > maxVal {
            maxVal = key
        }
        
        recordCount++
        
        if limit > 0 && recordCount >= limit {
            fmt.Printf("Reached limit of %d records\n", limit)
            break
        }
        
        if recordCount%100000 == 0 {
            fmt.Printf("Loaded %d records...\n", recordCount)
        }
    }
    
    var actualRecordSize uint64 = 32
    if maxVal > 0 {
        actualRecordSize = uint64(math.Ceil(math.Log2(float64(maxVal + 1))))
        actualRecordSize = ((actualRecordSize + 7) / 8) * 8
    }
    
    if recordBitLength > 0 {
        actualRecordSize = recordBitLength
        maxValueForRecordSize := uint64((1 << actualRecordSize) - 1)
        
        for i := range pirKeys {
            pirKeys[i] = pirKeys[i] % (maxValueForRecordSize + 1)
        }
    }
    
    fmt.Printf("Enhanced CSV loading completed: %d records, %d columns, record size: %d bits\n", 
               len(db.Records), len(db.Columns), actualRecordSize)
    
    return db, pirKeys, actualRecordSize, nil
}

func SaveEnhancedBinaryDatabase(enhancedDB *EnhancedDatabase, binPath string) error {
    file, err := os.Create(binPath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    encoder := gob.NewEncoder(file)
    return encoder.Encode(enhancedDB)
}

func LoadPIRKeysFromBinary(binPath string, recordBitLength uint64, limit uint64) ([]string, []uint64, uint64, error) {
    file, err := os.Open(binPath)
    if err != nil {
        return nil, nil, 0, err
    }
    defer file.Close()
    
    var numColumns uint32
    binary.Read(file, binary.LittleEndian, &numColumns)
    
    columns := make([]string, numColumns)
    for i := uint32(0); i < numColumns; i++ {
        var colLen uint32
        binary.Read(file, binary.LittleEndian, &colLen)
        colBytes := make([]byte, colLen)
        file.Read(colBytes)
        columns[i] = string(colBytes)
    }
    
    var totalRecords uint64
    binary.Read(file, binary.LittleEndian, &totalRecords)
    
    fmt.Printf("Loading PIR keys from binary: %d total records, %d columns\n", totalRecords, len(columns))
    
    recordsToLoad := totalRecords
    if limit > 0 && limit < totalRecords {
        recordsToLoad = limit
    }
    
    var pirKeys []uint64
    var maxVal uint64 = 0
    
    for i := uint64(0); i < recordsToLoad; i++ {
        var key uint64
        binary.Read(file, binary.LittleEndian, &key)
        
        pirKeys = append(pirKeys, key)
        if key > maxVal {
            maxVal = key
        }
        
        for j := uint32(0); j < numColumns; j++ {
            var valueLen uint32
            binary.Read(file, binary.LittleEndian, &valueLen)
            file.Seek(int64(valueLen), io.SeekCurrent)
        }
        
        if (i+1)%100000 == 0 {
            fmt.Printf("Loaded %d keys...\n", i+1)
        }
    }
    
    var actualRecordSize uint64 = 32
    if maxVal > 0 {
        actualRecordSize = uint64(math.Ceil(math.Log2(float64(maxVal + 1))))
        actualRecordSize = ((actualRecordSize + 7) / 8) * 8
    }
    
    if recordBitLength > 0 {
        actualRecordSize = recordBitLength
        maxValueForRecordSize := uint64((1 << actualRecordSize) - 1)
        
        for i := range pirKeys {
            pirKeys[i] = pirKeys[i] % (maxValueForRecordSize + 1)
        }
    }
    
    fmt.Printf("PIR keys loading completed: %d keys, record size: %d bits\n", 
               len(pirKeys), actualRecordSize)
    
    return columns, pirKeys, actualRecordSize, nil
}

func GetRecordFromBinary(binPath string, columns []string, recordIndex uint64) (map[string]string, error) {
    file, err := os.Open(binPath)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    var numColumns uint32
    binary.Read(file, binary.LittleEndian, &numColumns)
    
    for i := uint32(0); i < numColumns; i++ {
        var colLen uint32
        binary.Read(file, binary.LittleEndian, &colLen)
        file.Seek(int64(colLen), io.SeekCurrent)
    }
    
    file.Seek(8, io.SeekCurrent)
    
    for i := uint64(0); i < recordIndex; i++ {
        file.Seek(8, io.SeekCurrent)
        
        for j := uint32(0); j < numColumns; j++ {
            var valueLen uint32
            binary.Read(file, binary.LittleEndian, &valueLen)
            file.Seek(int64(valueLen), io.SeekCurrent)
        }
    }
    
    var key uint64
    binary.Read(file, binary.LittleEndian, &key)
    
    recordData := make(map[string]string)
    for j, col := range columns {
        var valueLen uint32
        binary.Read(file, binary.LittleEndian, &valueLen)
        valueBytes := make([]byte, valueLen)
        file.Read(valueBytes)
        
        if j < len(columns) {
            recordData[col] = string(valueBytes)
        }
    }
    
    return recordData, nil
}

func LoadDatabaseOnce() (*EnhancedDatabase, []uint64, []string, uint64, error) {
    globalDBMutex.Lock()
    defer globalDBMutex.Unlock()
    
    if globalDBLoaded {
        fmt.Println("Using cached database")
        return globalDB, globalPirKeys, globalColumns, globalActualRecordSize, nil
    }
    
    csvPath := "../../db/en.openfoodfacts.org.products.csv"
    binPath := "../../db/en.openfoodfacts.org.products.bin"
    keysOnlyPath := "../../db/en.openfoodfacts.org.products.keys.bin"
    
    if _, err := os.Stat(keysOnlyPath); err == nil {
        fmt.Println("Loading database from keys-only binary (ultra-fast)")
        globalColumns, globalPirKeys, globalActualRecordSize, err = LoadKeysOnlyBinary(keysOnlyPath, 0, 0)
        if err != nil {
            return nil, nil, nil, 0, fmt.Errorf("failed to load keys-only binary: %v", err)
        }
        
    } else if _, err := os.Stat(binPath); err == nil {
        fmt.Println("Loading database from binary and creating keys-only cache")
        globalColumns, globalPirKeys, globalActualRecordSize, err = LoadPIRKeysFromBinary(binPath, 0, 0)
        if err != nil {
            return nil, nil, nil, 0, fmt.Errorf("failed to load PIR keys from binary: %v", err)
        }
        
        fmt.Println("Creating keys-only cache for future use...")
        if saveErr := CreateKeysOnlyBinary(binPath, keysOnlyPath); saveErr != nil {
            fmt.Printf("Warning: Could not create keys-only cache: %v\n", saveErr)
        }
        
    } else if _, err := os.Stat(csvPath); err == nil {
        fmt.Println("Loading database from CSV (slowest option)")
        globalDB, globalPirKeys, globalActualRecordSize, err = LoadEnhancedCSVDatabase(csvPath, "code", 0, 0)
        if err != nil {
            return nil, nil, nil, 0, fmt.Errorf("failed to load CSV database: %v", err)
        }
        globalColumns = globalDB.Columns
        
        fmt.Println("Saving binary version for future use...")
        if saveErr := SaveEnhancedBinaryDatabase(globalDB, binPath); saveErr != nil {
            fmt.Printf("Warning: Could not save binary version: %v\n", saveErr)
        }
    } else {
        return nil, nil, nil, 0, fmt.Errorf("no database files found")
    }
    
    globalDBLoaded = true
    fmt.Printf("Database loaded once: %d records, %d columns, record size: %d bits\n", 
               len(globalPirKeys), len(globalColumns), globalActualRecordSize)
    
    return globalDB, globalPirKeys, globalColumns, globalActualRecordSize, nil
}

func CreateKeysOnlyBinary(fullBinPath, keysOnlyPath string) error {
    fmt.Println("Creating keys-only binary file...")
    
    columns, pirKeys, recordSize, err := LoadPIRKeysFromBinary(fullBinPath, 0, 0)
    if err != nil {
        return err
    }
    
    file, err := os.Create(keysOnlyPath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    binary.Write(file, binary.LittleEndian, uint32(len(columns)))
    for _, col := range columns {
        binary.Write(file, binary.LittleEndian, uint32(len(col)))
        file.Write([]byte(col))
    }
    
    binary.Write(file, binary.LittleEndian, recordSize)
    binary.Write(file, binary.LittleEndian, uint64(len(pirKeys)))
    
    for _, key := range pirKeys {
        binary.Write(file, binary.LittleEndian, key)
    }
    
    fmt.Printf("Created keys-only binary: %d keys, %d columns\n", len(pirKeys), len(columns))
    return nil
}

func LoadKeysOnlyBinary(keysOnlyPath string, recordBitLength uint64, limit uint64) ([]string, []uint64, uint64, error) {
    file, err := os.Open(keysOnlyPath)
    if err != nil {
        return nil, nil, 0, err
    }
    defer file.Close()
    
    var numColumns uint32
    binary.Read(file, binary.LittleEndian, &numColumns)
    
    columns := make([]string, numColumns)
    for i := uint32(0); i < numColumns; i++ {
        var colLen uint32
        binary.Read(file, binary.LittleEndian, &colLen)
        colBytes := make([]byte, colLen)
        file.Read(colBytes)
        columns[i] = string(colBytes)
    }
    
    var recordSize uint64
    var totalKeys uint64
    binary.Read(file, binary.LittleEndian, &recordSize)
    binary.Read(file, binary.LittleEndian, &totalKeys)
    
    keysToLoad := totalKeys
    if limit > 0 && limit < totalKeys {
        keysToLoad = limit
    }
    
    keys := make([]uint64, keysToLoad)
    for i := uint64(0); i < keysToLoad; i++ {
        binary.Read(file, binary.LittleEndian, &keys[i])
    }
    
    var actualRecordSize uint64 = recordSize
    if recordBitLength > 0 {
        actualRecordSize = recordBitLength
        maxValueForRecordSize := uint64((1 << actualRecordSize) - 1)
        
        for i := range keys {
            keys[i] = keys[i] % (maxValueForRecordSize + 1)
        }
    }
    
    fmt.Printf("Loaded keys-only binary: %d keys, %d columns, record size: %d bits\n", 
               len(keys), len(columns), actualRecordSize)
    
    return columns, keys, actualRecordSize, nil
}

func TestSetupDatabase(t *testing.T) {
    fmt.Println("=== Database Setup ===")
    
    csvPath := "../../db/en.openfoodfacts.org.products.csv"
    binPath := "../../db/en.openfoodfacts.org.products.bin"
    keysOnlyPath := "../../db/en.openfoodfacts.org.products.keys.bin"
    
    if _, err := os.Stat(binPath); os.IsNotExist(err) {
        fmt.Println("Converting CSV to binary...")
        start := time.Now()
        if err := ConvertCSVToBinaryStreamOptimized(csvPath, binPath, 0); err != nil {
            t.Fatalf("Failed to convert CSV: %v", err)
        }
        fmt.Printf("CSV conversion completed in %v\n", time.Since(start))
    } else {
        fmt.Println("Binary file already exists ✓")
    }
    
    if _, err := os.Stat(keysOnlyPath); os.IsNotExist(err) {
        fmt.Println("Creating ultra-fast keys-only cache...")
        start := time.Now()
        if err := CreateKeysOnlyBinary(binPath, keysOnlyPath); err != nil {
            t.Fatalf("Failed to create keys cache: %v", err)
        }
        fmt.Printf("Keys-only cache created in %v\n", time.Since(start))
    } else {
        fmt.Println("Keys-only cache already exists ✓")
    }

    fmt.Println("Testing cache performance...")
    start := time.Now()
    _, _, _, _, err := LoadDatabaseOnce()
    if err != nil {
        t.Fatalf("Failed to load database: %v", err)
    }
    elapsed := time.Since(start)
    
    fmt.Printf("\n=== Setup Complete ===\n")
    fmt.Printf("Database loading time: %v\n", elapsed)
}

// CSV TO BINARY CONVERSION FUNCTION ---------------------------------------------------------------------------------------------
func ConvertCSVToBinaryStreamOptimized(csvPath, binPath string, maxRecords uint64) error {
    fmt.Printf("Converting CSV to binary format (optimized streaming)...\n")
    
    csvFile, err := os.Open(csvPath)
    if err != nil {
        return fmt.Errorf("error opening CSV file: %v", err)
    }
    defer csvFile.Close()
    
    binFile, err := os.Create(binPath)
    if err != nil {
        return fmt.Errorf("error creating binary file: %v", err)
    }
    defer binFile.Close()
    
    reader := csv.NewReader(csvFile)
    reader.Comma = '\t'
    reader.LazyQuotes = true
    reader.FieldsPerRecord = -1
    
    header, err := reader.Read()
    if err != nil {
        return fmt.Errorf("error reading header: %v", err)
    }
    
    keyIndex := -1
    for i, col := range header {
        if col == "code" {
            keyIndex = i
            break
        }
    }
    
    if keyIndex == -1 {
        return fmt.Errorf("key column 'code' not found")
    }
    
    bufWriter := bufio.NewWriter(binFile)
    defer bufWriter.Flush()
    
    binary.Write(bufWriter, binary.LittleEndian, uint32(len(header))) // Number of columns
    for _, col := range header {
        binary.Write(bufWriter, binary.LittleEndian, uint32(len(col)))
        bufWriter.Write([]byte(col))
    }
    
    fmt.Printf("Optimized streaming conversion with %d columns, using 'code' as PIR key\n", len(header))
    if maxRecords > 0 {
        fmt.Printf("Converting up to %d records\n", maxRecords)
    }
    
    recordCount := uint64(0)
    
    bufWriter.Flush()
    recordCountPos, _ := binFile.Seek(0, io.SeekCurrent)
    binary.Write(binFile, binary.LittleEndian, uint64(0))
    
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            fmt.Printf("Warning: skipping malformed record at line %d: %v\n", recordCount+2, err)
            continue
        }
        
        if keyIndex >= len(record) {
            continue
        }
        
        var key uint64
        if v, err := strconv.ParseUint(strings.TrimSpace(record[keyIndex]), 10, 64); err == nil {
            key = v
        } else {
            key = stringToUint64Hash(record[keyIndex])
        }
        
        binary.Write(binFile, binary.LittleEndian, key)
        
        for i, value := range record {
            if i < len(header) {
                binary.Write(binFile, binary.LittleEndian, uint32(len(value)))
                binFile.Write([]byte(value))
            }
        }
        
        recordCount++
        
        if recordCount%50000 == 0 {
            fmt.Printf("Streamed %d records...\n", recordCount)
        }
        
        if maxRecords > 0 && recordCount >= maxRecords {
            fmt.Printf("Reached maximum record limit of %d\n", maxRecords)
            break
        }
    }
    
    binFile.Seek(recordCountPos, io.SeekStart)
    binary.Write(binFile, binary.LittleEndian, recordCount)
    
    fmt.Printf("Optimized streaming conversion completed: %d records saved to %s\n", recordCount, binPath)
    return nil
}

func TestConvertCSVToBinary(t *testing.T) {
    csvPath := "../../db/en.openfoodfacts.org.products.csv"
    binPath := "../../db/en.openfoodfacts.org.products.bin"
    
    if err := ConvertCSVToBinaryStreamOptimized(csvPath, binPath, 0); err != nil {
        t.Fatalf("Streaming conversion failed: %v", err)
    }
}

// CAPTURING AND EXTRACTION OUTPUT FUNCTIONS USED BY LOGGER -----------------------------------------------------------------------------
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

// TESTING QUERY PRODUCT BY ID FUNCTION ---------------------------------------------------------------------------------------------
func QueryProductByID(t *testing.T, productID uint64, DBSize uint64, recordSize uint64) {
    pir := SimplePIR{}
    
    _, allPirKeys, columns, baseRecordSize, err := LoadDatabaseOnce()
    if err != nil {
        t.Fatalf("Failed to load database: %v", err)
    }
    
    var pirKeys []uint64
    var actualRecordSize uint64
    
    // limit DB size
    if DBSize > 0 && DBSize < uint64(len(allPirKeys)) {
        pirKeys = allPirKeys[:DBSize]
    } else {
        pirKeys = allPirKeys
    }
    
    // limit Record size
    if recordSize > 0 {
        actualRecordSize = recordSize
        
        if recordSize < baseRecordSize {
            maxValueForRecordSize := uint64((1 << actualRecordSize) - 1)
            fmt.Printf("Compressing %d-bit keys to %d bits (max value: %d)\n", 
                       baseRecordSize, actualRecordSize, maxValueForRecordSize)
            
            for i := range pirKeys {
                pirKeys[i] = pirKeys[i] % (maxValueForRecordSize + 1)
            }
        } else if recordSize > baseRecordSize {
            fmt.Printf("Expanding %d-bit keys to %d bits for testing\n", 
                       baseRecordSize, actualRecordSize)
            
            expansion_factor := uint64(1 << (recordSize - baseRecordSize))
            for i := range pirKeys {
                pirKeys[i] = pirKeys[i] + (uint64(i)*expansion_factor)
            }
        }
    } else {
        actualRecordSize = baseRecordSize
    }
    
    actualDBSize := uint64(len(pirKeys))
    
    p := pir.PickParams(actualDBSize, actualRecordSize, SEC_PARAM, LOGQ)
    DB := MakeDB(actualDBSize, actualRecordSize, &p, pirKeys)
    
    var queryIndex uint64
    var found bool = false
    
    for i := uint64(0); i < actualDBSize; i++ {
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
    
    // run PIR
    RunPIR(&pir, DB, p, []uint64{queryIndex})
    
    // retrieve full record data
    binPath := "../../db/en.openfoodfacts.org.products.bin"
    fmt.Printf("Retrieving full record data for index %d...\n", queryIndex)
    recordData, err := GetRecordFromBinary(binPath, columns, queryIndex)
    if err != nil {
        fmt.Printf("Error retrieving record: %v\n", err)
        return
    }
    
    fmt.Printf("\n=== Retrieved Full Record for Product ID %d ===\n", productID)
    
    for _, column := range columns {
        if value, exists := recordData[column]; exists {
            displayValue := value
            if len(displayValue) > 100 {
                displayValue = displayValue[:100] + "..."
            }
            fmt.Printf("%-20s: %s\n", column, displayValue)
        } else {
            fmt.Printf("%-20s: <missing>\n", column)
        }
    }
    fmt.Printf("=== End Record ===\n")
    
    fmt.Printf("Successfully retrieved product ID %d\n", productID)
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

// RUNS=10 go test -timeout 0 -run=TestPIRWithDifferentRecordSizesMultiRun
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