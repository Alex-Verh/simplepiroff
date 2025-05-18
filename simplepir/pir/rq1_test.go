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

func QueryProductByID(t *testing.T, productID uint64, testDBSize uint64) {
    pir := SimplePIR{}
    dbPath := "../../db/database.txt"

    maxRecordSize, actualDBSize := AutoDetectRowLength(dbPath)

    dbSizeToUse := actualDBSize
    if testDBSize > 0 && testDBSize < actualDBSize {
        dbSizeToUse = testDBSize
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

    p := pir.PickParams(dbSizeToUse, maxRecordSize, SEC_PARAM, LOGQ)
    DB := MakeDB(dbSizeToUse, maxRecordSize, &p, limitedVals)

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
    QueryProductByID(t, productID, 0)
}

// go test -run=TestPIRWithDifferentDBSizes
// test using different DB sizes
func TestPIRWithDifferentDBSizes(t *testing.T) {
    sizes := []uint64{10, 100, 1000, 10000, 100000, 1000000}
    productID := uint64(54) // first product ID in the database
    
    for _, size := range sizes {
        fmt.Printf("\n\n==== Testing PIR with %d entries ====\n", size)
        QueryProductByID(t, productID, size)
    }
}
