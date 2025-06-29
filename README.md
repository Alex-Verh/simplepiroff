# SimplePIR on Open Food Facts

This project runs controlled experiments using the **SimplePIR** protocol on the **Open Food Facts** dataset.

---

## Dataset and Protocol Setup

### 1. Download the Dataset (~0.9 GB)

Download the compressed CSV file in the `./db` directory from the Open Food Facts website:

**[Download Link](https://static.openfoodfacts.org/data/en.openfoodfacts.org.products.csv.gz)**  
_(Compressed: ~0.9 GB, Uncompressed: ~9 GB)_

More info at: https://world.openfoodfacts.org/data

---

### 2. Unzip the `.gz` File

```bash
cd db
```

#### ▸ macOS or Linux

```bash
gunzip en.openfoodfacts.org.products.csv.gz
```

Or to keep the original .gz file:

```bash
gunzip -k en.openfoodfacts.org.products.csv.gz
```

#### ▸ Windows

Use a tool like 7-Zip:

1. Download and install 7-Zip.

2. Right-click the .gz file.

3. Select 7-Zip > Extract Here to get the .csv file.

Alternatively, use WSL:

```bash
gunzip en.openfoodfacts.org.products.csv.gz
```

---


### 3. Setup SimplePIR protocol

Follow up setup indications in `README.md` inside the `./simplepir`

Run correctness tests:

```bash
cd simplepir/pir/
go test
cd ../..
```

---

### 4. Convert CSV to PIR Format

Convert CSV format into Binary. [~15min]

```bash

go test -timeout 0 -run=TestConvertCSVToBinary

```

Setup database (load key-only file for faster run-time). [~5min]

```bash

SKIP_AUTOLOAD=true go test -timeout 0 -run=TestSetupDatabase

```
---

## Product Query

### TestQueryProduct: Queries a specific product ID from the database.

1. Set the ID using the PRODUCT_ID environment variable
2. Uses the full database by default


```bash
# Navigate to the test directory
cd simplepir/pir/

# Test querying a specific product ID
# Replace 63 with any product ID you want to search for
PRODUCT_ID=63 go test -run=TestQueryProduct
```

---


## Answering Research Question

### Sub-question I 

- All the controlled experiments related to RQ 1 can be found in the ```simplepir/pir/research_test.go``` file. 

#### TestPIRWithDifferentDBSizes: Tests PIR performance with varying database sizes

1. Tests with 10, 100, 1,000, 10,000, 100,000, and 1,000,000 entries
2. Helps measure how PIR scales with more data

#### TestPIRWithDifferentRecordSizes: Tests PIR with different record sizes

1. Tests with 8, 16, 32, 64, 128, 256 and 512-bit records
2. Shows how PIR performs with larger record sizes

#### TestPIRWithSizeCombinations: Tests all combinations of database sizes and record sizes

1. Runs the most comprehensive benchmark
2. Takes the longest to complete depending on count of runs

```bash
# Navigate to the test directory
cd simplepir/pir/

# Test PIR with different database sizes (10 to 1,000,000 entries)
RUNS=1 go test -run=TestPIRWithDifferentDBSizes

# Test PIR with different record sizes (8 to 256 bits)
RUNS=1 go test -run=TestPIRWithDifferentRecordSizes

# Test PIR with combinations of different DB sizes and record sizes
# This will run multiple tests with all combinations
RUNS=1 go test -run=TestPIRWithSizeCombinations
```

To get more reliable performance metrics, you can run each test multiple times and generate plots from the averaged results.

```bash
# Navigate to the test directory
cd simplepir/pir/

# Run database size tests with 10 iterations per size
RUNS=10 go test -run=TestPIRWithDifferentDBSizes

# Run record size tests with 10 iterations per size
RUNS=10 go test -run=TestPIRWithDifferentRecordSizes

# Run combinations with 3 iterations per combination
RUNS=5 go test -run=TestPIRWithSizeCombinations
```

#### Generating Plots from Results

Create a virtual environment:

##### ▸ macOS or Linux

```bash
python3 -m venv venv

source venv/bin/activate
```

##### ▸ Windows

```bash
python -m venv venv

venv\Scripts\activate
```

Install all the dependencies:

##### ▸ macOS or Linux

```bash
pip3 install -r requirements.txt
```

##### ▸ Windows

```bash
pip install -r requirements.txt
```

```bash
# Plot all average result files
python plot_results.py --avg

# Plot a specific results file
python plot_results.py results/dbsize_avg_results.csv

# Plot all results files (including individual runs)
python plot_results.py --all
```

---

### Sub-question II

- All the controlled experiments related to RQ 2 can be found in the ```./network_conditions.go``` file. 

To compute network time costs of SimplePIR under different bandwidth and latency conditions:

1. Good Network - 100 Mbps bandwidth and 10ms latency
2. Average Network - 25 Mbps bandwidth and 50ms latency
3. Poor Network - 5 Mbps bandwidth and 200ms latency

```bash
# in the ./simplepiroff directory
go run network_conditions.go
```

---

### Sub-question III

- All the controlled experiments related to RQ 3 can be found in the ```./redis``` directory. 

#### Start Redis

```bash
brew services start redis
```

#### Load dataset into Redis

```bash
go run loader.go ../db/en.openfoodfacts.org.products.csv
```

#### Run the benchmarks

Different database sizes benchmark:

```bash
go run dbsize_benchmark.go [ID_NUMBER] [RUNS_COUNT]
```

Different record sizes benchmark:

```bash
go run recordsize_benchmark.go [ID_NUMBER] [RUNS_COUNT]
```

#### Stop Redis

```bash
brew services stop redis
```
---

### Sub-question IV

- All the controlled experiments related to RQ 4 can be found in the ```./demo``` directory. 

#### Start Redis
```bash
brew services start redis
```

#### Clean wasm javascript file (if present)
```bash
rm -f pir.wasm wasm_exec.js
```

#### Compute fresh wasm_exec.js
```bash
cp /opt/homebrew/Cellar/go/1.24.2/libexec/lib/wasm/wasm_exec.js .
```

#### Build fresh WASM
```bash
cd wasm
go clean -cache
GOOS=js GOARCH=wasm go build -o ../pir.wasm .
cd ..
```

#### Start demo server
```bash
go run main.go
```

#### Test query barcodes

##### Use ```test_barcodes.txt``` for barcodes examples.
---