package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/sensu/sensu-go/types"
)

type Authentication struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Expiration   int64  `json:"expires_at"`
}

type RemediationConfig struct {
	Request       string   `json:"request"`
	Occurrences   []int    `json:"occurrences"`
	Severities    []int    `json:"severities"`
	Subscriptions []string `json:subscriptions`
}
type RequestPayload struct {
	Check         string   `json:"check"`
	Subscriptions []string `json:"subscriptions"`
}

var (
	sensuApiUrl      string = getenv("SENSU_API_URL", "http://127.0.0.1:8080")
	sensuApiCertFile string = getenv("SENSU_API_CERT_FILE", "")
	sensuApiUser     string = getenv("SENSU_API_USER", "admin")
	sensuApiPass     string = getenv("SENSU_API_PASS", "P@ssw0rd!")
	sensuApiToken    string
)

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func contains(s []int, i int) bool {
	for _, a := range s {
		if a == i {
			return true
		}
	}
	return false
}

func authenticate(httpClient *http.Client) string {
	var authentication Authentication
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/auth", sensuApiUrl),
		nil,
	)
	if err != nil {
		log.Fatal("ERROR: ", err)
	}
	req.SetBasicAuth(sensuApiUser, sensuApiPass)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("ERROR: %s", err)
	} else if resp.StatusCode == 401 {
		log.Fatalf("ERROR: %v %s (please check your access credentials)", resp.StatusCode, http.StatusText(resp.StatusCode))
	} else if resp.StatusCode >= 300 {
		log.Fatalf("ERROR: %v %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("ERROR: ", err)
	}
	err = json.NewDecoder(bytes.NewReader(b)).Decode(&authentication)
	if err != nil {
		log.Fatal("ERROR: ", err)
	}
	return authentication.AccessToken
}

func LoadCACerts(path string) (*x509.CertPool, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("Error loading system cert pool: %s", err)
	}
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if path != "" {
		certs, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("Error reading CA file (%s): %s", path, err)
		} else {
			rootCAs.AppendCertsFromPEM(certs)
		}
	}
	return rootCAs, nil
}

func initHttpClient() *http.Client {
	certs, err := LoadCACerts(sensuApiCertFile)
	if err != nil {
		log.Fatalf("ERROR: %s\n", err)
	}
	tlsConfig := &tls.Config{
		RootCAs: certs,
	}
	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{
		Transport: tr,
	}
	return client
}

func main() {
	var stdin *os.File
	var event types.Event
	var actions []RemediationConfig
	var annotationConfigKey string = "io.sensu.remediation.config.actions"
	var httpClient *http.Client = initHttpClient()

	stdin = os.Stdin
	err := json.NewDecoder(stdin).Decode(&event)
	if err != nil {
		log.Fatal("ERROR: ", err)
	}

	if event.Check.Annotations[annotationConfigKey] != "" {
		err = json.Unmarshal([]byte(event.Check.Annotations[annotationConfigKey]), &actions)
		if err != nil {
			log.Fatal("ERROR: ", err)
		}
		for _, action := range actions {
			// Only perform the action if the event severity matches
			if !contains(action.Severities, int(event.Check.Status)) {
				// Mismatched severity
				log.Printf("Remediation action \"%s\" configured to trigger on severities: %v (nothing to do for serverity %v).", action.Request, action.Severities, event.Check.Status)
			} else {
				// Only perform the action if the event occurrence matches
				if !contains(action.Occurrences, int(event.Check.Occurrences)) {
					// Mismatched occurrences
					log.Printf("Remediation action \"%s\" configured to trigger on occurrence(s): %v (nothing to do on occurrence #%v).", action.Request, action.Occurrences, event.Check.Occurrences)
				} else {
					// Perform the remediation action!
					if len(action.Subscriptions) == 0 {
						action.Subscriptions = append(action.Subscriptions, fmt.Sprintf("entity:%s", event.Entity.Name))
					}
					log.Printf("Requesting the \"%s\" remediation action on the \"%s\" subscription(s).", action.Request, action.Subscriptions)
					data := RequestPayload{
						Check:         action.Request,
						Subscriptions: action.Subscriptions,
					}
					postBody, err := json.Marshal(data)
					if err != nil {
						log.Fatal("ERROR: ", err)
					}
					body := bytes.NewReader(postBody)
					req, err := http.NewRequest(
						"POST",
						fmt.Sprintf("%s/api/core/v2/namespaces/%s/checks/%s/execute",
							sensuApiUrl,
							event.Entity.Namespace,
							action.Request,
						),
						body,
					)
					if err != nil {
						log.Fatal("ERROR: ", err)
					}
					sensuApiToken = authenticate(httpClient)
					req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", sensuApiToken))
					req.Header.Set("Content-Type", "application/json")
					resp, err := httpClient.Do(req)
					if err != nil {
						log.Fatalf("ERROR: %s\n", err)
					} else if resp.StatusCode == 404 {
						log.Fatalf("ERROR: %v %s (%s); no check named \"%s\" found in namespace \"%s\".\n", resp.StatusCode, http.StatusText(resp.StatusCode), req.URL, action.Request, event.Entity.Namespace)
					} else if resp.StatusCode >= 300 {
						log.Fatalf("ERROR: %v %s", resp.StatusCode, http.StatusText(resp.StatusCode))
					}
					defer resp.Body.Close()
					b, err := ioutil.ReadAll(resp.Body)
					fmt.Println(resp.StatusCode)
					fmt.Println(string(b))
				}
			}
		}
	} else {
		// No configured actions
		log.Printf("No remediation actions configured for check \"%s\"; nothing to do.", event.Check.Name)
		log.Printf("To enable remediation actions, provide remediation configuration using the \"%s\" check annotation.", annotationConfigKey)
	}
}
