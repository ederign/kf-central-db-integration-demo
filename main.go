package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
)

type ParamsData struct {
	Params        map[string]interface{}
	ModelRegistry map[string]interface{}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received %s request for %s", r.Method, r.URL.Path)

	log.Println("Incoming Request Headers:")
	for name, values := range r.Header {
		for _, value := range values {
			log.Printf("Header: %s=%s", name, value)
		}
	}

	log.Println("Incoming Cookies:")
	for _, cookie := range r.Cookies() {
		log.Printf("Cookie: %s=%s", cookie.Name, cookie.Value)
	}

	w.Header().Set("X-Frame-Options", "ALLOWALL")
	w.Header().Set("Content-Security-Policy", "frame-ancestors *;")

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var params map[string]interface{}

	if r.Method == http.MethodPost {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading request body: %v", err)
			http.Error(w, "Unable to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if len(body) > 0 {
			if err := json.Unmarshal(body, &params); err != nil {
				log.Printf("Error parsing JSON: %v", err)
				http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
				return
			}
		} else {
			params = map[string]interface{}{}
		}
	} else if r.Method == http.MethodGet {
		r.ParseForm()
		params = make(map[string]interface{})
		for key, values := range r.Form {
			if len(values) == 1 {
				params[key] = values[0]
			} else {
				params[key] = values
			}
		}
	}

	// Read the oauth2_proxy_kubeflow cookie
	oauthCookie, err := r.Cookie("oauth2_proxy_kubeflow")
	if err != nil {
		if err == http.ErrNoCookie {
			log.Println("oauth2_proxy_kubeflow cookie not found")
			params["oauth2_proxy_kubeflow"] = nil
		} else {
			log.Printf("Error reading oauth2_proxy_kubeflow cookie: %v", err)
			http.Error(w, "Error reading cookies", http.StatusBadRequest)
			return
		}
	} else {
		params["oauth2_proxy_kubeflow"] = oauthCookie.Value
	}

	// Read the kubeflow-userid header
	kubeflowUserID := r.Header.Get("kubeflow-userid")
	if kubeflowUserID == "" {
		log.Println("kubeflow-userid header not found")
	} else {
		params["kubeflow-userid"] = kubeflowUserID
	}

	// Read the x-forwarded-access-token header
	xForwardedAccessToken := r.Header.Get("x-forwarded-access-token")
	if xForwardedAccessToken == "" {
		log.Println("x-forwarded-access-token header not found")
	} else {
		params["x-forwarded-access-token"] = xForwardedAccessToken
	}

	// Call the model-registry microservice
	modelRegistryURL := "http://model-registry-bff-service.kubeflow.svc.cluster.local:4000/api/v1/model_registry"
	modelRegistryResp, err := http.Get(modelRegistryURL)
	if err != nil {
		log.Printf("Error calling model registry service: %v", err)
		http.Error(w, "Error calling model registry service", http.StatusInternalServerError)
		return
	}
	defer modelRegistryResp.Body.Close()

	// Print the status code of the model-registry response
	fmt.Println(modelRegistryResp.StatusCode)

	if modelRegistryResp.StatusCode != http.StatusOK {
		log.Printf("Model registry service returned status: %s", modelRegistryResp.Status)
		http.Error(w, "Model registry service error", modelRegistryResp.StatusCode)
		return
	}

	modelRegistryBody, err := ioutil.ReadAll(modelRegistryResp.Body)
	if err != nil {
		log.Printf("Error reading model registry response body: %v", err)
		http.Error(w, "Error reading model registry response", http.StatusInternalServerError)
		return
	}

	var modelRegistryData map[string]interface{}
	if err := json.Unmarshal(modelRegistryBody, &modelRegistryData); err != nil {
		log.Printf("Error parsing model registry JSON: %v", err)
		http.Error(w, "Invalid JSON from model registry service", http.StatusBadRequest)
		return
	}

	tmpl := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Request Parameters and Model Registry Data</title>
		<style>
			body { font-family: Arial, sans-serif; margin: 20px; }
			h1 { color: #333; }
			h2 { color: #444; }
			ul { list-style-type: none; padding: 0; }
			li { background: #f4f4f4; margin: 5px 0; padding: 10px; border-radius: 4px; }
			strong { color: #555; }
		</style>
	</head>
	<body>
		<h1>Request Parameters and Authentication Details</h1>
		<ul>
			{{range $key, $value := .Params}}
				<li><strong>{{$key}}:</strong> {{printf "%v" $value}}</li>
			{{end}}
		</ul>

		<h2>Model Registry Data</h2>
		<ul>
			{{range $key, $value := .ModelRegistry}}
				<li><strong>{{$key}}:</strong> {{printf "%v" $value}}</li>
			{{end}}
		</ul>
	</body>
	</html>
	`

	t, err := template.New("index").Parse(tmpl)
	if err != nil {
		log.Printf("Template parsing error: %v", err)
		http.Error(w, "Error parsing template", http.StatusInternalServerError)
		return
	}

	data := ParamsData{
		Params:        params,
		ModelRegistry: modelRegistryData,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Error executing template", http.StatusInternalServerError)
	}
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", handleRequest)
	mux.HandleFunc("/modelRegistry/", handleRequest)

	port := ":8887"
	log.Printf("Server is running on port %s", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
