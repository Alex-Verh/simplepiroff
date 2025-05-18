package pir

import (
    "bufio"
    "fmt"
    "math"
    "os"
    "strconv"
    "strings"
    "testing"
)

func QueryProductByID(t *testing.T, productID uint64, DBSize uint64, recordSize uint64) {
    pir := SimplePIR{}
    dbPath := "../../db/database.txt"

    actualRecordSize, actualDBSize := AutoDetectRowLength(dbPath)

    dbSizeToUse := actualDBSize
    if DBSize > 0 && DBSize < actualDBSize {
        dbSizeToUse = DBSize
        fmt.Printf("Testing with reduced DB size: %d entries (of %d total)\n", 
        dbSizeToUse, actualDBSize)
    }

    file, _ := os.Open(dbPath)
    defer file.Close()
    scanner := bufio.NewScanner(file)
    
    var limitedVals []uint64
    count := uint64(0)
    
    for scanner.Scan() && count < dbSizeToUse {
        line := strings.TrimSpace(scanner.Text())
        if line == "" {
            continue
        }
        
        fields := strings.FieldsFunc(line, func(r rune) bool {
            return r == ':' || r == '\t' || r == ' '
        })
        
        if len(fields) > 0 {
            if val, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
                limitedVals = append(limitedVals, val)
                count++
            }
        }
    }

    // if recordSize is 0, use the max record size from the db
    recordSizeToUse := actualRecordSize
    if recordSize > 0 && recordSize < actualRecordSize {
        fmt.Printf("Testing with reduced record size: %d bits (of %d total)\n",
                   recordSize, actualRecordSize)
        recordSizeToUse = recordSize

        maxValueForRecordSize := uint64((1 << recordSizeToUse) - 1)

        for i := range limitedVals {
            limitedVals[i] = limitedVals[i] % (maxValueForRecordSize + 1)
        }

        fmt.Printf("Values adjusted to fit exactly in %d bits (max possible value: %d)\n",
            recordSizeToUse, maxValueForRecordSize)
    }

    // adjust the database size to match the number of values read (cause: duplicates)
    dbSizeToUse = uint64(len(limitedVals))

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