package pir

import (
    "bufio"
    "encoding/binary"
    "fmt"
    "math"
    "os"
    "strconv"
    "strings"
    "testing"
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
    dbSizes := []uint64{10, 100, 1000, 10000, 100000, 1000000}
    productID := uint64(54) // first product ID in the database
    
    for _, dbSize := range dbSizes {
        fmt.Printf("\n\n==== Testing PIR with %d entries ====\n", dbSize)
        QueryProductByID(t, productID, dbSize, 0)
    }
}

// go test -run=TestPIRWithDifferentRecordSizes
// test using different record sizes
func TestPIRWithDifferentRecordSizes(t *testing.T) {
    recordSizes := []uint64{8, 16, 32, 64, 128, 256}
    productID := uint64(54) // first product ID in the database
    
    for _, recordSize := range recordSizes {
        fmt.Printf("\n\n==== Testing PIR with record size %d bits ====\n", recordSize)
        QueryProductByID(t, productID, 0, recordSize)
    }
}

// go test -run=TestPIRWithSizeCombinations
// test using different record sizes and DB sizes
func TestPIRWithSizeCombinations(t *testing.T) {
    dbSizes := []uint64{10, 100, 1000, 10000, 100000, 1000000}
    recordSizes := []uint64{8, 16, 32, 64, 128, 256}
    productID := uint64(54) // first product ID in the database

    for _, dbSize := range dbSizes {
        for _, recordSize := range recordSizes {
            fmt.Printf("\n\n==== Testing PIR with DB size %d and record size %d bits ====\n", dbSize, recordSize)
            QueryProductByID(t, productID, dbSize, recordSize)
        }
    }
}

//  LOADING DATABASE FUNCTIONS -----------------------------------------------------------------------------

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