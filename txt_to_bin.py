import struct
import os
import sys

def convert_txt_to_binary(txt_path, bin_path):
    """Convert a text database file to binary format for faster PIR processing."""
    codes = []
    max_uint64 = 2**64 - 1
    
    try:
        print(f"Reading text database from {txt_path}...")
        with open(txt_path, "r", encoding="utf-8") as txtfile:
            for line in txtfile:
                line = line.strip()
                if not line:
                    continue
                    
                parts = line.split(":", 1)
                if parts:
                    try:
                        code = int(parts[0].strip())
                        if 0 <= code <= max_uint64:
                            codes.append(code)
                        else:
                            print(f"Skipping code {code} - out of uint64 range")
                    except ValueError:
                        pass
        
        print(f"Found {len(codes)} valid numeric codes.")
                        
        print(f"Writing binary database to {bin_path}...")
        with open(bin_path, "wb") as binfile:
            binfile.write(struct.pack("<Q", len(codes)))
            
            for i, code in enumerate(codes):
                try:
                    binfile.write(struct.pack("<Q", code))
                    if (i+1) % 100000 == 0 or i+1 == len(codes):
                        print(f"Progress: {i+1}/{len(codes)} entries written ({(i+1)/len(codes)*100:.1f}%)")
                except struct.error as e:
                    print(f"Error packing code {code}: {e}")
                
        total_size = os.path.getsize(bin_path)
        print(f"Conversion complete: {len(codes)} entries written to {bin_path}")
        print(f"Binary file size: {total_size} bytes ({total_size/1024/1024:.2f} MB)")
        
    except Exception as e:
        print(f"Error converting to binary: {e}")
        return False
        
    return True

if __name__ == "__main__":
    default_txt_path = "db/database.txt"
    default_bin_path = "db/database.bin"
    
    txt_path = sys.argv[1] if len(sys.argv) > 1 else default_txt_path
    bin_path = sys.argv[2] if len(sys.argv) > 2 else default_bin_path
    
    os.makedirs(os.path.dirname(bin_path), exist_ok=True)
    
    if not os.path.exists(txt_path):
        print(f"Error: Text file {txt_path} not found")
        sys.exit(1)
        
    print(f"Converting {txt_path} to binary format at {bin_path}")
    if convert_txt_to_binary(txt_path, bin_path):
        print("Conversion successful!")
    else:
        print("Conversion failed.")
        sys.exit(1)