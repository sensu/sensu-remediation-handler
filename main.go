package main

import (
  "os"
  "log"
  "fmt"
  "bytes"
  "encoding/json"
  "net/http"
  "io/ioutil"

  "github.com/sensu/sensu-go/types"
)

type Authentication struct {
  AccessToken string `json:"access_token"`
  RefreshToken string `json:"refresh_token"`
  Expiration int64 `json:"expires_at"`
}

type RemediationConfig struct {
  Request string `json:"request"`
  Occurrences []int `json:"occurrences"`
  Severities []int `json:"severities"`
  Subscriptions []string `json:subscriptions`
}
type RequestPayload struct {
  Check string `json:"check"`
  Subscriptions []string `json:"subscriptions"`
}

var (
  sensuApiHost string = getenv("SENSU_BACKEND_HOST","127.0.0.1")
  sensuApiPort string = getenv("SENSU_BACKEND_PORT","8080")
  sensuApiUser string = os.Getenv("SENSU_USER")
  sensuApiPass string = os.Getenv("SENSU_PASS")
  sensuApiToken string
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

func authenticate() string {
  var authentication Authentication
  req, err := http.NewRequest(
    "GET",
    fmt.Sprintf("http://%s:%s/auth",sensuApiHost,sensuApiPort),
    nil,
  )
  if err != nil {
  	log.Fatal(err)
  }
  req.SetBasicAuth(sensuApiUser, sensuApiPass)
  resp, err := http.DefaultClient.Do(req)
  if err != nil {
  	log.Fatal(err)
  }
  defer resp.Body.Close()
  b,err := ioutil.ReadAll(resp.Body)
  if err != nil {
    log.Fatal(err)
  }
  err = json.NewDecoder(bytes.NewReader(b)).Decode(&authentication)
  if err != nil {
    log.Fatal(err)
  }
  return authentication.AccessToken
}


func main() {
  var stdin *os.File
  var event types.Event
  var actions []RemediationConfig
  var annotationConfigKey string = "sensu.io/plugins/remediation/config/actions"

  stdin = os.Stdin
  err := json.NewDecoder(stdin).Decode(&event)
  if err != nil {
    log.Fatal(err)
  }

  if event.Check.Annotations[annotationConfigKey] != "" {
    err = json.Unmarshal([]byte(event.Check.Annotations[annotationConfigKey]), &actions)
    if err != nil {
      log.Fatal(err)
    }
    for _,action := range( actions ) {
      // Only perform the action if the event severity matches
      if !contains(action.Severities, int(event.Check.Status)) {
        // Mismatched severity
        log.Printf("Remediation action \"%s\" configured to trigger on severities: %v (nothing to do for serverity %v).",action.Request,action.Severities,event.Check.Status)
      } else {
        // Only perform the action if the event occurrence matches
        if !contains(action.Occurrences, int(event.Check.Occurrences)) {
          // Mismatched occurrences
          log.Printf("Remediation action \"%s\" configured to trigger on occurrence(s): %v (nothing to do on occurrence #%v).",action.Request,action.Occurrences,event.Check.Occurrences)
        } else {
          // Perform the remediation action!
          if len(action.Subscriptions) == 0 {
            action.Subscriptions = append(action.Subscriptions, fmt.Sprintf("entity:%s",event.Entity.Name))
          }
          log.Printf("Requesting the \"%s\" remediation action on the \"%s\" subscription(s).",action.Request,action.Subscriptions)
          data := RequestPayload{
            Check: action.Request,
            Subscriptions: action.Subscriptions,
          }
          postBody, err := json.Marshal(data)
          if err != nil {
            log.Fatal(err)
          }
          body := bytes.NewReader(postBody)
          req, err := http.NewRequest(
            "POST",
            fmt.Sprintf("http://%s:%s/api/core/v2/namespaces/default/checks/%s/execute",sensuApiHost,sensuApiPort,action.Request),
            body,
          )
          if err != nil {
            log.Fatal(err)
          }
          sensuApiToken = authenticate()
          req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", sensuApiToken))
          req.Header.Set("Content-Type", "application/json")
          resp, err := http.DefaultClient.Do(req)
          if err != nil {
            log.Fatal(err)
          }
          defer resp.Body.Close()
          b,err := ioutil.ReadAll(resp.Body)
          fmt.Println(string(b))
        }
      }
    }
  } else {
    // No configured actions
    log.Printf("No remediation actions configured; nothing to do.")
    log.Printf("To enable remediation actions, configure the \"%s\" check annotation.", annotationConfigKey)
  }
}
