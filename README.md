# SimplePIR on Open Food Facts

This project runs controlled experiments using the **SimplePIR** protocol on the **Open Food Facts** dataset.

---

## Dataset Setup

### 1. Download the Dataset (~0.9 GB)

Download the compressed CSV file from the Open Food Facts website:

**[Download Link](https://static.openfoodfacts.org/data/en.openfoodfacts.org.products.csv.gz)**  
_(Compressed: ~0.9 GB, Uncompressed: ~9 GB)_

More info at: https://world.openfoodfacts.org/data

---

### 2. Unzip the `.gz` File

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

### 3. Convert CSV to PIR Format

Install all the dependencies:

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
