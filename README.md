# SimplePIR on Open Food Facts

This project runs controlled experiments using the **SimplePIR** protocol on the **Open Food Facts** dataset.

---

## Dataset Setup

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

### 3. Convert CSV to PIR Format

Create a virtual environment:

#### ▸ macOS or Linux

```bash
python3 -m venv venv

source venv/bin/activate
```

#### ▸ Windows

```bash
python -m venv venv

venv\Scripts\activate
```

Install all the dependencies:

#### ▸ macOS or Linux

```bash
pip3 install -r requirements.txt
```

#### ▸ Windows

```bash
pip install -r requirements.txt
```

From the project root directory, run:

#### ▸ macOS or Linux

```bash
python3 csv_to_txt.py

python3 txt_to_bin.py
```

#### ▸ Windows

```bash
py csv_to_txt.py

py txt_to_bin.py
```

---

## Run protocol over dataset

### 1. Clone SimplePIR repository

In the root directory run:

```bash
git clone https://github.com/ahenzinger/simplepir.git
```

---

### 2. Setup SimplePIR protocol

Follow up setup indications in `README.md` inside the `./simplepir`

Run correctness tests:

```bash
cd simplepir/pir/
go test
cd ../..
```

---

### 3. Research Question 1

#### TestQueryProduct: Queries a specific product ID from the database.

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

#### TestPIRWithDifferentDBSizes: Tests PIR performance with varying database sizes

1. Tests with 10, 100, 1,000, 10,000, 100,000, and 1,000,000 entries
2. Helps measure how PIR scales with more data

#### TestPIRWithDifferentRecordSizes: Tests PIR with different record sizes

1. Tests with 8, 16, 32, 64, 128, and 256-bit records
2. Shows how PIR performs with larger record sizes

#### TestPIRWithSizeCombinations: Tests all combinations of database sizes and record sizes

1. Runs the most comprehensive benchmark
2. Takes the longest to complete [~1min]

```bash
# Navigate to the test directory
cd simplepir/pir/

# Test PIR with different database sizes (10 to 1,000,000 entries)
go test -run=TestPIRWithDifferentDBSizes

# Test PIR with different record sizes (8 to 256 bits)
go test -run=TestPIRWithDifferentRecordSizes

# Test PIR with combinations of different DB sizes and record sizes
# This will run multiple tests with all combinations
go test -run=TestPIRWithSizeCombinations
```

---
