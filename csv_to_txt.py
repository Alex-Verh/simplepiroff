import pandas as pd

def convert_csv_to_txt_format(csv_path, output_path, chunk_size=100000):
    products = []
    total = 0
    valid = 0

    try:
        with open(output_path, "w", encoding="utf-8") as outfile:
            for chunk in pd.read_csv(csv_path, sep="\t", chunksize=chunk_size, low_memory=False, on_bad_lines="skip"):
                total += len(chunk)
                for _, row in chunk.iterrows():
                    code = str(row.get("code", "")).strip()
                    name = str(row.get("product_name", "")).strip()
                    if code and name:
                        line = f"{code}: {name}"
                        outfile.write(line + "\n")
                        valid += 1
        print(f"Done! Processed {total} rows, wrote {valid} valid entries to {output_path}")
    except Exception as e:
        print(f"Error during processing: {e}")

if __name__ == "__main__":
    input_csv = "db/en.openfoodfacts.org.products.csv"
    output_txt = "db/database.txt"
    convert_csv_to_txt_format(input_csv, output_txt)