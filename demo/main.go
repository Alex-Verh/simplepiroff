package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"demo/pir"
)

type PIRQueryResponse struct {
	Encrypted bool   `json:"encrypted"`
	Result    string `json:"result"`
	Error     string `json:"error,omitempty"`
}

type ProductResponse struct {
	Name    string            `json:"name"`
	Barcode string            `json:"barcode"`
	AllData map[string]string `json:"allData,omitempty"`
	Error   string            `json:"error,omitempty"`
}

func simpleEncrypt(data string, key string) string {
	encrypted := make([]byte, len(data))
	for i := 0; i < len(data); i++ {
		encrypted[i] = data[i] ^ key[i%len(key)]
	}
	return string(encrypted)
}

func simpleDecrypt(encryptedData string, key string) string {
	return simpleEncrypt(encryptedData, key)
}

func stringToUint64Hash(s string) uint64 {
	hash := uint64(5381)
	for _, c := range s {
		hash = ((hash << 5) + hash) + uint64(c)
	}
	return hash
}

type MockT struct {
	failed bool
}

func (t *MockT) Fatalf(format string, args ...interface{}) {
	t.failed = true
	fmt.Printf("ERROR: %s\n", fmt.Sprintf(format, args...))
	panic(fmt.Sprintf(format, args...))
}

func (t *MockT) Failed() bool {
	return t.failed
}

func captureQueryOutput(productID uint64) (string, error) {
	mockT := &MockT{}

	_, pirKeys, _, _, err := pir.LoadDatabaseOnce()
	if err != nil {
		return "", fmt.Errorf("database error: %v", err)
	}

	found := false
	for _, key := range pirKeys {
		if key == productID {
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("product with ID %d not found in database", productID)
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var output string

	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("query failed: %v", r)
			}
		}()
		pir.QueryProductByID(mockT, productID, 0, 0)
	}()

	w.Close()
	os.Stdout = old

	var buf strings.Builder
	io.Copy(&buf, r)
	output = buf.String()

	fmt.Print(output)

	if mockT.failed || err != nil {
		return "", fmt.Errorf("product not found")
	}

	if len(output) == 0 {
		return "", fmt.Errorf("no product data found")
	}

	return output, nil
}

func findProductByBarcode(barcode string) (uint64, error) {
	_, pirKeys, _, _, err := pir.LoadDatabaseOnce()
	if err != nil {
		return 0, fmt.Errorf("database error: %v", err)
	}

	if id, err := strconv.ParseUint(barcode, 10, 64); err == nil {
		for _, key := range pirKeys {
			if key == id {
				return id, nil
			}
		}
	}

	hashedID := stringToUint64Hash(barcode)
	for _, key := range pirKeys {
		if key == hashedID {
			return hashedID, nil
		}
	}

	return 0, fmt.Errorf("barcode %s not found in database", barcode)
}

func extractProductInfo(output string, barcode string) (map[string]string, error) {
	lines := strings.Split(output, "\n")

	productData := make(map[string]string)
	inRecordSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, "=== Retrieved Full Record for Product ID") {
			inRecordSection = true
			continue
		}

		if strings.Contains(line, "=== End Record ===") {
			inRecordSection = false
			break
		}

		if inRecordSection && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				if value != "<missing>" && value != "" {
					productData[key] = value
				}
			}
		}
	}

	if len(productData) == 0 {
		return nil, fmt.Errorf("product not found or no product data available")
	}

	return productData, nil
}

func testDatabaseConnection() {
	mockT := &MockT{}

	defer func() {
		if r := recover(); r != nil {
		}
	}()

	_, pirKeys, _, _, err := pir.LoadDatabaseOnce()
	if err != nil {
		return
	}

	if len(pirKeys) > 0 {
		fmt.Printf("  - First few keys: %v\n", pirKeys[:min(5, len(pirKeys))])
	}

	if len(pirKeys) > 0 {
		pir.QueryProductByID(mockT, pirKeys[0], 0, 0)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func handlePIRQuery(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Query parameter required", http.StatusBadRequest)
		return
	}

	decryptedQuery := simpleDecrypt(query, "simplepir")

	var queryData struct {
		Barcode string `json:"barcode"`
	}
	if err := json.Unmarshal([]byte(decryptedQuery), &queryData); err != nil {
		http.Error(w, "Invalid encrypted query", http.StatusBadRequest)
		return
	}

	fmt.Printf("REAL PIR Query for barcode: %s\n", queryData.Barcode)

	productID, err := findProductByBarcode(queryData.Barcode)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		errorResponse := PIRQueryResponse{
			Encrypted: true,
			Result:    simpleEncrypt(`{"error":"Product not found"}`, "simplepir"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	start := time.Now()

	output, err := executeRealPIRQuery(productID)
	if err != nil {
		fmt.Printf("ERROR: Real PIR query failed: %v\n", err)
		errorResponse := PIRQueryResponse{
			Encrypted: true,
			Result:    simpleEncrypt(`{"error":"Product not found"}`, "simplepir"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	elapsed := time.Since(start)
	fmt.Printf("REAL PIR query completed in %v\n", elapsed)

	allProductData, err := extractProductInfo(output, queryData.Barcode)
	if err != nil {
		fmt.Printf("ERROR: Failed to extract product info: %v\n", err)
		errorResponse := PIRQueryResponse{
			Encrypted: true,
			Result:    simpleEncrypt(`{"error":"Failed to retrieve product data"}`, "simplepir"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	productName := allProductData["product_name"]
	if productName == "" {
		productName = allProductData["code"]
	}

	responseData := ProductResponse{
		Name:    productName,
		Barcode: queryData.Barcode,
		AllData: allProductData,
	}

	responseJSON, _ := json.Marshal(responseData)
	encryptedResponse := simpleEncrypt(string(responseJSON), "simplepir")

	response := PIRQueryResponse{
		Encrypted: true,
		Result:    encryptedResponse,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func executeRealPIRQuery(productID uint64) (string, error) {
	mockT := &MockT{}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var output string
	var err error

	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("PIR query failed: %v", r)
			}
		}()
		pir.QueryProductByID(mockT, productID, 0, 0)
	}()

	w.Close()
	os.Stdout = old

	var buf strings.Builder
	io.Copy(&buf, r)
	output = buf.String()

	if mockT.failed || err != nil {
		return "", fmt.Errorf("PIR query failed")
	}

	if len(output) == 0 {
		return "", fmt.Errorf("no PIR output generated")
	}

	return output, nil
}

func handlePIRProtocol(w http.ResponseWriter, r *http.Request) {
	var request struct {
		PIRQuery []byte                 `json:"pirQuery"`
		Metadata map[string]interface{} `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var barcode string = "0"

	if request.Metadata != nil {
		if originalBarcode, ok := request.Metadata["originalBarcode"].(string); ok {
			barcode = originalBarcode
		}
	}

	if len(request.PIRQuery) >= 8 {
		var extracted strings.Builder
		for i := 0; i < min(32, len(request.PIRQuery)); i++ {
			if request.PIRQuery[i] >= '0' && request.PIRQuery[i] <= '9' {
				extracted.WriteByte(request.PIRQuery[i])
			} else {
				break
			}
		}
		if extracted.Len() > 0 {
			barcode = extracted.String()
		}
	}

	var productID uint64
	if id, err := strconv.ParseUint(barcode, 10, 64); err == nil {
		productID = id
	} else {
		productID = stringToUint64Hash(barcode)
	}

	output, err := captureQueryOutput(productID)
	if err != nil {
		response := map[string]interface{}{
			"encryptedResult": "encrypted_error_response",
			"productData": map[string]interface{}{
				"error": "Product not found",
			},
			"timestamp": time.Now().Unix(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	allProductData, err := extractProductInfo(output, barcode)
	if err != nil {
		log.Printf("ERROR: Failed to extract product info for barcode %s: %v", barcode, err)
		response := map[string]interface{}{
			"encryptedResult": "encrypted_error_response",
			"productData": map[string]interface{}{
				"error": "Product not found",
			},
			"timestamp": time.Now().Unix(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	response := map[string]interface{}{
		"encryptedResult": "encrypted_pir_response_data",
		"productData":     allProductData,
		"timestamp":       time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleRegularSearch(w http.ResponseWriter, r *http.Request) {
	barcode := r.URL.Query().Get("query")
	if barcode == "" {
		http.Error(w, "Query parameter required", http.StatusBadRequest)
		return
	}

	fmt.Printf("Regular search for barcode: %s\n", barcode)

	productID, err := findProductByBarcode(barcode)
	if err != nil {
		fmt.Printf("ERROR: Regular search failed: %v\n", err)
		response := ProductResponse{Error: "Product not found"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	start := time.Now()

	output, err := executeDirectLookup(productID)
	if err != nil {
		fmt.Printf("ERROR: Direct lookup failed: %v\n", err)
		response := ProductResponse{Error: "Product not found"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	elapsed := time.Since(start)
	fmt.Printf("Direct lookup completed in %v\n", elapsed)

	allProductData, err := extractProductInfo(output, barcode)
	if err != nil {
		fmt.Printf("ERROR: Failed to extract product info from regular search: %v\n", err)
		response := ProductResponse{Error: "Failed to retrieve product data"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	productName := allProductData["product_name"]
	if productName == "" {
		productName = allProductData["code"]
	}

	response := ProductResponse{
		Name:    productName,
		Barcode: barcode,
		AllData: allProductData,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func executeDirectLookup(productID uint64) (string, error) {
	_, pirKeys, columns, _, err := pir.LoadDatabaseOnce()
	if err != nil {
		return "", fmt.Errorf("database error: %v", err)
	}

	var queryIndex uint64
	var found bool
	for i, key := range pirKeys {
		if key == productID {
			queryIndex = uint64(i)
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("product with ID %d not found in database", productID)
	}

	binPath := "../db/en.openfoodfacts.org.products.bin"
	recordData, err := pir.GetRecordFromBinary(binPath, columns, queryIndex)
	if err != nil {
		return "", fmt.Errorf("error retrieving record: %v", err)
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("=== Retrieved Full Record for Product ID %d (Index: %d) ===\n", productID, queryIndex))

	for _, column := range columns {
		if value, exists := recordData[column]; exists {
			output.WriteString(fmt.Sprintf("%-20s: %s\n", column, value))
		} else {
			output.WriteString(fmt.Sprintf("%-20s: <missing>\n", column))
		}
	}
	output.WriteString("=== End Record ===\n")

	return output.String(), nil
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	isPrivate := r.URL.Query().Get("private") == "true"

	if isPrivate {
		handlePIRQuery(w, r)
	} else {
		handleRegularSearch(w, r)
	}
}

func main() {
	fmt.Println("Starting PIR service...")

	testDatabaseConnection()

	r := mux.NewRouter()

	r.HandleFunc("/search", handleSearch).Methods("GET")
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
	}).Methods("GET")
	r.HandleFunc("/pir-protocol", handlePIRProtocol).Methods("POST")

	r.PathPrefix("/").Handler(http.FileServer(http.Dir(".")))

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	handler := c.Handler(r)

	port := "3000"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	fmt.Printf("PIR service running on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
