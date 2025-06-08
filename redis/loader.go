package main

import (
    "bufio"
    "context"
    "fmt"
    "github.com/go-redis/redis/v8"
    "os"
    "runtime"
    "strconv"
    "strings"
    "sync"
    "time"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: go run improved_loader.go <data_file> [limit] [worker_count]")
        os.Exit(1)
    }

    dataFile := os.Args[1]
    
    var limit uint64 = 0
    if len(os.Args) > 2 {
        var err error
        limit, err = strconv.ParseUint(os.Args[2], 10, 64)
        if err != nil {
            fmt.Printf("Invalid limit: %v\n", err)
            os.Exit(1)
        }
    }
    
    numWorkers := 4 
    if len(os.Args) > 3 {
        var err error
        numWorkers, err = strconv.Atoi(os.Args[3])
        if err != nil {
            fmt.Printf("Invalid worker count: %v\n", err)
            os.Exit(1)
        }
    }
    
    if numWorkers <= 0 {
        numWorkers = runtime.NumCPU() / 2
        if numWorkers < 1 {
            numWorkers = 1
        }
    }

    // Connect to Redis
    client := redis.NewClient(&redis.Options{
        Addr:         "localhost:6379",
        Password:     "",
        DB:           0,
        PoolSize:     numWorkers + 2,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
        DialTimeout:  10 * time.Second,
    })
    defer client.Close()

    ctx := context.Background()

    _, err := client.Ping(ctx).Result()
    if err != nil {
        fmt.Printf("Cannot connect to Redis: %v\n", err)
        os.Exit(1)
    }

    client.FlushDB(ctx)

    file, err := os.Open(dataFile)
    if err != nil {
        fmt.Printf("Cannot open data file: %v\n", err)
        os.Exit(1)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    var count uint64 = 0
    var lineCount uint64 = 0
    var errorCount uint64 = 0
    var successCount uint64 = 0

    markerSizes := []int{1, 10, 100, 1000, 10000, 100000, 1000000}
    markersMutex := &sync.Mutex{}
    markersCreated := make(map[int]bool)

    const maxCapacity = 10 * 1024 * 1024 
    buf := make([]byte, maxCapacity)
    scanner.Buffer(buf, maxCapacity)

    fmt.Printf("Loading data from %s into Redis using %d workers (limit: %d records)...\n", 
        dataFile, numWorkers, limit)
    startTime := time.Now()
    
    if scanner.Scan() {
        lineCount++
    }

    batchSize := 100
    
    lineChan := make(chan string, batchSize*2)
    
    successChan := make(chan int, numWorkers*10)
    
    errorChan := make(chan error, numWorkers*10)
    
    var wg sync.WaitGroup
    
    go func() {
        for err := range errorChan {
            errorCount++
            if errorCount % 100 == 0 {
                fmt.Printf("Encountered %d errors so far. Last error: %v\n", errorCount, err)
            }
        }
    }()
    
    go func() {
        lastSuccess := uint64(0)
        for count := range successChan {
            successCount += uint64(count)
            if successCount % 100000 == 0 {
                elapsed := time.Since(startTime)
                rate := float64(successCount) / elapsed.Seconds()
                fmt.Printf("Successfully loaded %d records in %.2f seconds (%.2f records/sec)\n", 
                    successCount, elapsed.Seconds(), rate)
            }
            markersMutex.Lock()
            for _, size := range markerSizes {
                if uint64(size) > lastSuccess && uint64(size) <= successCount && !markersCreated[size] {
                    markerKey := fmt.Sprintf("dbsize:%d", size)
                    client.Set(ctx, markerKey, fmt.Sprintf("%d", size), 0)
                    markersCreated[size] = true
                    fmt.Printf("Added marker for DB size %d\n", size)
                }
            }
            markersMutex.Unlock()
            lastSuccess = successCount
        }
    }()
    
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerId int) {
            defer wg.Done()
            localCount := 0
            localSuccessCount := 0
            localPipeline := client.Pipeline()
            
            executePipeline := func() {
                if localCount == 0 {
                    return
                }
                
                maxRetries := 3
                retryDelay := 1 * time.Second
                
                for retry := 0; retry < maxRetries; retry++ {
                    _, err := localPipeline.Exec(ctx)
                    if err == nil {
                        successChan <- localSuccessCount
                        localCount = 0
                        localSuccessCount = 0
                        localPipeline = client.Pipeline()
                        return
                    }
                    
                    if retry < maxRetries-1 {
                        errorChan <- fmt.Errorf("worker %d: pipeline error (retry %d): %v", 
                            workerId, retry, err)
                        time.Sleep(retryDelay)
                        retryDelay *= 2 
                    } else {
                        errorChan <- fmt.Errorf("worker %d: pipeline error (gave up): %v", 
                            workerId, err)
                    }
                }
                
                localCount = 0
                localSuccessCount = 0
                localPipeline = client.Pipeline()
            }
            
            for line := range lineChan {
                parts := strings.Split(line, "\t")
                if len(parts) > 0 {
                    rawID := parts[0]
                    cleanID := strings.TrimSpace(rawID)
                    
                    id, err := strconv.ParseUint(cleanID, 10, 64)
                    if err != nil {
                        continue
                    }
                    
                    key := fmt.Sprintf("product:%d", id)
                    localPipeline.Set(ctx, key, line, 0)
                    
                    localCount++
                    localSuccessCount++
                    
                    if localCount >= batchSize {
                        executePipeline()
                    }
                }
            }
            
            if localCount > 0 {
                executePipeline()
            }
        }(i)
    }
    
    for scanner.Scan() && (limit == 0 || count < limit) {
        lineCount++
        line := scanner.Text()
        lineChan <- line
        
        count++
        if count%100000 == 0 {
            elapsed := time.Since(startTime)
            rate := float64(count) / elapsed.Seconds()
            fmt.Printf("Processed %d records in %.2f seconds (%.2f records/sec)\n", 
                count, elapsed.Seconds(), rate)
        }
        
        if limit > 0 && count >= limit {
            break
        }
    }
    
    close(lineChan)
    
    wg.Wait()
    
    close(successChan)
    close(errorChan)

    if err := scanner.Err(); err != nil {
        fmt.Printf("Error reading file: %v\n", err)
        os.Exit(1)
    }

    actualCount, err := client.DBSize(ctx).Result()
    if err != nil {
        fmt.Printf("Error getting DB size: %v\n", err)
    }

    elapsed := time.Since(startTime)
    rate := float64(successCount) / elapsed.Seconds()
    
    fmt.Printf("\nProcessed %d lines and successfully loaded %d keys into Redis in %.2f seconds (%.2f records/sec)\n", 
        lineCount, actualCount, elapsed.Seconds(), rate)
    
    if errorCount > 0 {
        fmt.Printf("Encountered %d errors during loading\n", errorCount)
    }
    
    fmt.Println("\nVerifying DB size markers:")
    for _, size := range markerSizes {
        markerKey := fmt.Sprintf("dbsize:%d", size)
        val, err := client.Get(ctx, markerKey).Result()
        if err != nil {
            fmt.Printf("Marker for size %d: Not found\n", size)
        } else {
            fmt.Printf("Marker for size %d: Found - %s\n", size, val)
        }
    }
    
    fmt.Println("\nVerifying a few records:")
    testIDs := []uint64{54, 100, 1000}
    for _, id := range testIDs {
        key := fmt.Sprintf("product:%d", id)
        val, err := client.Get(ctx, key).Result()
        if err != nil {
            fmt.Printf("Key %s: Not found\n", key)
        } else {
            displayVal := val
            if len(val) > 50 {
                displayVal = val[:50] + "..."
            }
            fmt.Printf("Key %s: Found - %s\n", key, displayVal)
        }
    }
}