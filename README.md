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
python3 csv_formatter.py
```

#### ▸ Windows

```bash
py csv_formatter.py
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

### 3. Run a basic test query

```bash

```

---
