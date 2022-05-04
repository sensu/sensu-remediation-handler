package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/sensu/sensu-plugin-sdk/sensu"
	"github.com/sensu/sensu-go/types"
)

// Config represents the handler plugin config.
type Config struct {
	sensu.PluginConfig
	Example            string
	Annotation         string
	SensuApiUrl        string
	SensuApiKey        string
	SensuTrustedCaFile string
}

// Remediation action configuration
type RemediationAction struct {
	Request       string   `json:"request"`
	Occurrences   []int    `json:"occurrences"`
	Severities    []int    `json:"severities"`
	Subscriptions []string `json:"subscriptions"`
}

// Remediation action request (i.e. "POST /check/:check/execute" request object)
type RemediationRequest struct {
	Check         string   `json:"check"`
	Subscriptions []string `json:"subscriptions"`
}

var (
	config = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-remediation-handler",
			Short:    "Sensu Go handler for triggering automated remediations (playbooks)",
			Keyspace: "sensu.io/plugins/sensu-remediation-handler/config",
		},
	}

	options = []*sensu.PluginConfigOption{
		&sensu.PluginConfigOption{
			Path:      "annotation",
			Env:       "SENSU_REMEDIATION_ANNOTATION",
			Argument:  "annotation",
			Shorthand: "a",
			Default:   "io.sensu.remediation.config.actions",
			Usage:     "Remediation actions annotation",
			Value:     &config.Annotation,
		},
		&sensu.PluginConfigOption{
			Path:      "sensu-api-url",
			Env:       "SENSU_API_URL",
			Argument:  "sensu-api-url",
			Shorthand: "",
			Default:   "http://127.0.0.1:8080",
			Usage:     "Sensu API URL (defaults to $SENSU_API_URL)",
			Value:     &config.SensuApiUrl,
		},
		&sensu.PluginConfigOption{
			Path:      "sensu-api-key",
			Env:       "SENSU_API_KEY",
			Argument:  "sensu-api-key",
			Shorthand: "",
			Default:   "",
			Usage:     "Sensu API Key (defaults to $SENSU_API_KEY)",
			Value:     &config.SensuApiKey,
		},
		&sensu.PluginConfigOption{
			Path:      "sensu-trusted-ca-file",
			Env:       "SENSU_TRUSTED_CA_FILE",
			Argument:  "sensu-trusted-ca-file",
			Shorthand: "",
			Default:   "",
			Usage:     "Sensu API Trusted Certificate Authority File (defaults to $SENSU_TRUSTED_CA_FILE)",
			Value:     &config.SensuTrustedCaFile,
		},
	}
)

func main() {
	handler := sensu.NewGoHandler(&config.PluginConfig, options, checkArgs, executeHandler)
	handler.Execute()
}

func checkArgs(event *types.Event) error {
	if len(config.SensuApiUrl) == 0 {
		return errors.New("--sensu-api-url flag or $SENSU_API_URL environment variable must be set")
	} else if len(config.SensuApiKey) == 0 {
		return errors.New("--sensu-api-key flag or $SENSU_API_KEY environment variable must be set")
	} else {
		return nil
	}
}

func executeHandler(event *types.Event) error {
	actions,err := parseRemediationActions(event)
	if err != nil {
		log.Fatalf("ERROR: %s\n",err)
		return err
	}
	action,proceed := matchRemediationAction(actions,event)
	if proceed {
		err = processRemediationAction(action,event)
		if err != nil {
			log.Fatalf("ERROR: %s\n",err)
			return err
		}
	}
	return nil
}

// helper function to look for items in a slice
func contains(s []int, i int) bool {
	for _, a := range s {
		if a == i {
			return true
		}
	}
	return false
}

// helper function to load custom CA certs
func loadCaCerts(path string) (*x509.CertPool, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		log.Fatalf("ERROR: failed to load system cert pool: %s", err)
		return nil, err
	}
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if path != "" {
		certs, err := ioutil.ReadFile(path)
		if err != nil {
			log.Fatalf("ERROR: failed to read CA file (%s): %s", path, err)
			return nil, err
		} else {
			rootCAs.AppendCertsFromPEM(certs)
		}
	}
	return rootCAs, nil
}

// HTTP client initializer
func initHttpClient() *http.Client {
	certs, err := loadCaCerts(config.SensuTrustedCaFile)
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

// Parse Remediation Actions Annotation
func parseRemediationActions(event *types.Event) ([]RemediationAction, error) {
	var actions []RemediationAction
	// Parse remediation actions annotation
	err := json.Unmarshal([]byte(event.Check.Annotations[config.Annotation]), &actions)
	if err != nil {
		log.Fatalf("ERROR: %s\n", err)
		return actions, err
	}
	return actions, nil
}

// Evaluate remediation actions, determine if a remediation action is required
func matchRemediationAction(actions []RemediationAction, event *types.Event) (*RemediationAction, bool) {
	for _, action := range actions {
		// Only perform the action if the event severity matches
		if !contains(action.Severities, int(event.Check.Status)) {
			// Mismatched severity... nothing to do
			log.Printf("Remediation action \"%s\" configured to trigger on severities: %v (nothing to do for serverity %v).", action.Request, action.Severities, event.Check.Status)
			return nil, false
		} else if !contains(action.Occurrences, int(event.Check.Occurrences)) {
			// Mismatched occurrences... nothing to do
			log.Printf("Remediation action \"%s\" configured to trigger on occurrence(s): %v (nothing to do on occurrence #%v).", action.Request, action.Occurrences, event.Check.Occurrences)
			return nil, false
		} else {
			// Matching severity & occurrences... let's process this job!
			return &action,true
		}
	}
	return nil,false
}

// Process remediation action
func processRemediationAction(action *RemediationAction, event *types.Event) error {
	var httpClient *http.Client = initHttpClient()
	if len(action.Subscriptions) == 0 {
		action.Subscriptions = append(action.Subscriptions, fmt.Sprintf("entity:%s", event.Entity.Name))
	}
	log.Printf("Requesting the \"%s\" remediation action on the \"%s\" subscription(s).", action.Request, action.Subscriptions)
	data := RemediationRequest{
		Check:         action.Request,
		Subscriptions: action.Subscriptions,
	}
	postBody, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("ERROR: %s\n", err)
		return err
	}
	body := bytes.NewReader(postBody)
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/api/core/v2/namespaces/%s/checks/%s/execute",
			config.SensuApiUrl,
			event.Entity.Namespace,
			action.Request,
		),
		body,
	)
	if err != nil {
		log.Fatal("ERROR: ", err)
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Key %s", config.SensuApiKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("ERROR: %s\n", err)
		return err
	} else if resp.StatusCode == 404 {
		log.Fatalf("ERROR: %v %s (%s); no check named \"%s\" found in namespace \"%s\".\n", resp.StatusCode, http.StatusText(resp.StatusCode), req.URL, action.Request, event.Entity.Namespace)
		return err
	} else if resp.StatusCode >= 300 {
		log.Fatalf("ERROR: %v %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		return err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body")
		log.Fatalf("ERROR: %s\n", err)
		return err
	}
	fmt.Println(resp.StatusCode)
	fmt.Println(string(b))
	return nil
}
