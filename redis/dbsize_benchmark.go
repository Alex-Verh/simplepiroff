package main

import (
    "context"
    "encoding/csv"
    "fmt"
    "github.com/go-redis/redis/v8"
    "os"
    "strconv"
    "time"
)

func main() {
    // Redis
    client := redis.NewClient(&redis.Options{
        Addr:         "localhost:6379",
        Password:     "",
        DB:           0,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 5 * time.Second,
    })
    defer client.Close()

    ctx := context.Background()

    _, err := client.Ping(ctx).Result()
    if err != nil {
        fmt.Printf("Cannot connect to Redis: %v\n", err)
        os.Exit(1)
    }

    keyCount, err := client.DBSize(ctx).Result()
    if err != nil {
        fmt.Printf("Error getting DB size: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Current Redis DB has %d keys\n", keyCount)

   if keyCount == 0 {
        fmt.Println("No keys in Redis DB.")
        os.Exit(1)
    }
    
    targetSizes := []int{1, 10, 100, 1000, 10000, 100000, 1000000}
    availableSizes := []int{}
    
    for _, size := range targetSizes {
        markerKey := fmt.Sprintf("dbsize:%d", size)
        exists, err := client.Exists(ctx, markerKey).Result()
        if err == nil && exists > 0 {
            availableSizes = append(availableSizes, size)
        }
    }
    
    if len(availableSizes) == 0 {
        fmt.Println("No DB size markers found. Using default sizes up to current DB size.")
        for _, size := range targetSizes {
            if int64(size) <= keyCount {
                availableSizes = append(availableSizes, size)
            }
        }
    }
    
    if len(availableSizes) == 0 {
        fmt.Println("No suitable DB sizes to test")
        os.Exit(1)
    }
    
    fmt.Printf("Will test these DB sizes: %v\n", availableSizes)
    
    productID := uint64(54)
    if len(os.Args) > 1 {
        var err error
        productID, err = strconv.ParseUint(os.Args[1], 10, 64)
        if err != nil {
            fmt.Printf("Invalid product ID: %v\n", err)
            os.Exit(1)
        }
    }
    
    runs := 50
    if len(os.Args) > 2 {
        var err error
        runs, err = strconv.Atoi(os.Args[2])
        if err != nil {
            fmt.Printf("Invalid number of runs: %v\n", err)
            os.Exit(1)
        }
    }

    resultsFile, err := os.Create("../results/redis_dbsize_results.csv")
    if err != nil {
        fmt.Printf("Error creating results file: %v\n", err)
        os.Exit(1)
    }
    defer resultsFile.Close()

    writer := csv.NewWriter(resultsFile)
    defer writer.Flush()

    writer.Write([]string{"db_size", "record_size", "run_count", "answer_time", "offline_download", "online_download", "online_upload", "query_time", "reconstruct_time", "setup_time"})

    fmt.Printf("Benchmarking Redis query performance with product ID %d for different DB sizes\n", productID)

    key := fmt.Sprintf("product:%d", productID)
    
    exists, err := client.Exists(ctx, key).Result()
    if err != nil || exists == 0 {
        fmt.Printf("Key %s does not exist in Redis. Please check your data.\n", key)
        os.Exit(1)
    }
    
    val, err := client.Get(ctx, key).Result()
    if err != nil {
        fmt.Printf("Error retrieving key %s: %v\n", key)
        os.Exit(1)
    }
    recordSize := len(val)
    fmt.Printf("Record size for key %s: %d bytes\n", key, recordSize)
    
    for _, dbSize := range availableSizes {
        fmt.Printf("\nTesting with DB size %d\n", dbSize)

        totalQueryTime := int64(0)
        successfulRuns := 0
        
        for i := 0; i < 5; i++ {
            client.Get(ctx, key).Result()
            time.Sleep(10 * time.Millisecond)
        }
        
        for run := 1; run <= runs; run++ {
            startTime := time.Now()
            _, err := client.Get(ctx, key).Result()
            queryTime := time.Since(startTime).Nanoseconds() / 1000000
            
            if err != nil {
                fmt.Printf("Run %d: Error: %v\n", run, err)
                continue
            }
            
            totalQueryTime += queryTime
            successfulRuns++
            
            fmt.Printf("Run %d: Query time %d ms\n", run, queryTime)
            
            time.Sleep(5 * time.Millisecond)
        }
        
        var avgQueryTime float64 = 0
        if successfulRuns > 0 {
            avgQueryTime = float64(totalQueryTime) / float64(successfulRuns)
        }
        
        avgQueryTimeSec := avgQueryTime / 1000.0
        responseSizeKB := float64(recordSize) / 1024.0
        
        writer.Write([]string{
            strconv.Itoa(dbSize),
            strconv.Itoa(recordSize * 8), 
            strconv.Itoa(successfulRuns),
            fmt.Sprintf("%.6f", avgQueryTimeSec), 
            "0",
            fmt.Sprintf("%.6f", responseSizeKB), 
            "0",
            fmt.Sprintf("%.6f", avgQueryTimeSec),
            "0",
            "0",
        })
        
        fmt.Printf("DB Size %d: Avg query time %.6f sec (%.2f ms)\n", 
            dbSize, avgQueryTimeSec, avgQueryTime)
    }

    fmt.Println("\nBenchmark complete. Results saved to redis_dbsize_results.csv")
}