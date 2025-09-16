package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	setCORSHeaders(w)
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
		}
	}
}

func writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	writeJSONResponse(w, statusCode, map[string]string{"error": message})
}

func handleCIDRs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cidrService, err := NewCIDRService(ctx)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError,
			fmt.Sprintf("failed to initialize CIDR service: %v", err))
		return
	}

	switch r.Method {
	case "OPTIONS":
		setCORSHeaders(w)
		w.WriteHeader(http.StatusOK)

	case "GET":
		if r.URL.Path == "/next" || r.URL.Query().Get("action") == "next" {
			nextCIDR, err := cidrService.GetNextAvailableCIDR(ctx)
			if err != nil {
				writeErrorResponse(w, http.StatusInternalServerError,
					fmt.Sprintf("failed to get next available CIDR: %v", err))
				return
			}
			writeJSONResponse(w, http.StatusOK, map[string]string{"cidr": nextCIDR})
			return
		}

		records, err := cidrService.GetAllCIDRs(ctx)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError,
				fmt.Sprintf("failed to get CIDRs: %v", err))
			return
		}

		writeJSONResponse(w, http.StatusOK, map[string]interface{}{
			"records": records,
			"count":   len(records),
		})

	case "POST":
		var requestBody struct {
			Key  string `json:"key"`
			CIDR string `json:"cidr"`
		}

		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if requestBody.Key == "" || requestBody.CIDR == "" {
			writeErrorResponse(w, http.StatusBadRequest,
				"both key and cidr fields are required")
			return
		}

		if err := cidrService.RegisterCIDR(ctx, requestBody.Key, requestBody.CIDR); err != nil {
			writeErrorResponse(w, http.StatusBadRequest,
				fmt.Sprintf("failed to register CIDR: %v", err))
			return
		}

		writeJSONResponse(w, http.StatusCreated, map[string]string{
			"message": "CIDR registered successfully",
			"key":     requestBody.Key,
			"cidr":    requestBody.CIDR,
		})

	case "DELETE":
		key := r.URL.Query().Get("key")
		if key == "" {
			writeErrorResponse(w, http.StatusBadRequest, "key parameter is required")
			return
		}

		if err := cidrService.DeleteCIDR(ctx, key); err != nil {
			writeErrorResponse(w, http.StatusInternalServerError,
				fmt.Sprintf("failed to delete CIDR: %v", err))
			return
		}

		writeJSONResponse(w, http.StatusOK, map[string]string{
			"message": "CIDR deleted successfully",
			"key":     key,
		})

	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", handleCIDRs)
	http.HandleFunc("/next", handleCIDRs)

	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
