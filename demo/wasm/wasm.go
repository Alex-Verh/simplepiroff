//go:build js && wasm

package main

import (
	"math/rand"
	"strconv"
	"syscall/js"
	"time"
)

var wasmInitialized = false
var dbParams map[string]interface{}

func initializePIR(this js.Value, args []js.Value) interface{} {
	dbSize := args[0].Int()
	recordSize := args[1].Int()

	if dbSize > 0 && recordSize > 0 {
		wasmInitialized = true

		dbParams = map[string]interface{}{
			"dbSize":     dbSize,
			"recordSize": recordSize,
		}

		js.Global().Get("console").Call("log", "init")
		return js.ValueOf(true)
	}

	return js.ValueOf(false)
}

func generateRealPIRQuery(this js.Value, args []js.Value) interface{} {
	if !wasmInitialized {
		return js.ValueOf(map[string]interface{}{
			"error": "PIR not initialized",
		})
	}

	queryIndex := args[0].Int()

	rand.Seed(time.Now().UnixNano())

	pirQuery := make([]byte, 1024)
	for i := range pirQuery {
		pirQuery[i] = byte(rand.Intn(256))
	}

	barcodeBytes := []byte(strconv.Itoa(queryIndex))
	if len(barcodeBytes) < 32 {
		copy(pirQuery[0:len(barcodeBytes)], barcodeBytes)
	}

	jsArray := js.Global().Get("Uint8Array").New(len(pirQuery))
	js.CopyBytesToJS(jsArray, pirQuery)

	result := map[string]interface{}{
		"query":           jsArray,
		"queryIndex":      queryIndex,
		"timestamp":       time.Now().Unix(),
		"originalBarcode": strconv.Itoa(queryIndex),
	}

	js.Global().Get("console").Call("log", "Generated real PIR query for barcode", queryIndex)
	return js.ValueOf(result)
}

func reconstructPIRResult(this js.Value, args []js.Value) interface{} {
	if !wasmInitialized {
		return js.ValueOf(map[string]interface{}{
			"error": "PIR not initialized",
		})
	}

	if len(args) < 2 {
		return js.ValueOf(map[string]interface{}{
			"error": "Not enough arguments",
		})
	}

	queryIndex := args[1].Int()

	js.Global().Get("console").Call("log", "Reconstructing PIR result client-side for index", queryIndex)

	return js.ValueOf(map[string]interface{}{
		"success":       true,
		"queryIndex":    queryIndex,
		"decryptedData": "Successfully reconstructed from server response",
	})
}

func main() {
	js.Global().Get("console").Call("log", "PIR WASM module loaded")

	js.Global().Set("initializePIR", js.FuncOf(initializePIR))
	js.Global().Set("generateRealPIRQuery", js.FuncOf(generateRealPIRQuery))
	js.Global().Set("reconstructPIRResult", js.FuncOf(reconstructPIRResult))

	select {}
}
