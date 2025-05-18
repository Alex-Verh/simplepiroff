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

func QueryProductByID(t *testing.T, productID uint64) {
    pir := SimplePIR{}
    dbPath := "../../db/database.txt"

    maxRecordSize, maxDBSize := AutoDetectRowLength(dbPath)

    p := pir.PickParams(maxDBSize, maxRecordSize, SEC_PARAM, LOGQ)
    DB := LoadDBFromFile(dbPath, maxRecordSize, &p)

    var queryIndex uint64
    var found bool = false
    
    // for i := uint64(0); i < uint64(math.Min(float64(100), float64(maxDBSize))); i++ { // scan only first 100 entries
	for i := uint64(0); i < maxDBSize; i++ {
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


// PRODUCT_ID=63 go test -run=TestQueryProduct
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
    QueryProductByID(t, productID)
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