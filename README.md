# Sensu Go Remediation Handler

The Sensu Go Remediation Handler is a simple [Sensu Go event handler][handler]
for configuring self-healing workflows using Sensu. This handler was heavily
inspired by [Nick Stielau's "Sensu Remediator"][remediator] (circa 2012).

The Sensu Go Remediation Handler &ndash; and other similar "self healing"
workflows in Sensu &ndash; use a few simple Sensu features:

- An "unscheduled check" configuration (i.e. a Sensu Check with the `"publish":
  false` attribute set).
- The Sensu Checks API `POST /checks/:check/execute` endpoint, which provides
  the ability to make ad hoc check requests.
- The Sensu Agent's built-in entity subscriptions (e.g. `entity:web-server-01`)
  which make it possible to target a single agent with a check execution
  request.  

[handler]: https://docs.sensu.io/sensu-go/latest/reference/handlers/
[remediator]: https://github.com/sensu-plugins/sensu-plugins-sensu/blob/master/bin/handler-sensu.rb

## Example Usage

### Example "Unscheduled" Check (Remediation Action)

```yaml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: systemd-start-nginx
  namespace: default
spec:
  command: sudo systemctl start nginx
  publish: false
  interval: 10 # not used
  subscriptions:
```

_NOTE: the `interval` attribute provided here is not actually used and shouldn't
be required; see the [Sensu Go issue #2623][2623] for more information._

[2623]: https://github.com/sensu/sensu-go/issues/2623

### Example Check Definition and Remediation Request Configuration

```yaml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: check-nginx
  namespace: default
  labels:
    foo: bar
  annotations:
    sensu.io/plugins/remediation/config/actions: |
      [
        {
          "description": "Perform this action once after Nginx has been down for 30 seconds.",
          "request": "systemd-start-nginx",
          "occurrences": [ 3 ],
          "severities": [ 1,2 ]
        },
        {
          "description": "Perform this action once after Nginx has been down for 120 seconds.",
          "request": "systemd-restart-nginx",
          "occurrences": [ 12 ],
          "severities": [ 1,2 ]
        }
      ]
spec:
  command: check_http -H 127.0.0.1 -P 80 -N
  publish: false
  interval: 10
  handlers:
  - remediation
  subscriptions:
  - nginx
```

## Configuration

### Environment Variables

The Sensu Go Remediation Handler requires the following environment variables to
be set in the `sensu-backend` environment:

- `SENSU_BACKEND_HOST`: The `sensu-backend` hostname or IP address (default: 127.0.0.1)
- `SENSU_BACKEND_PORT`: The `sensu-backend` API port (default: 8080)
- `SENSU_USER`: The Sensu user the handler will authenticate with
- `SENSU_PASS`: The Sensu password the handler will authenticate with  

### Setup

1. Create a dedicated Sensu user & role for the remediation handler

   ```
   $ sensuctl role create remediation-handler --namespace=default --verbs=create,update --resources checks
   $ sensuctl role-binding create remediation-handler --role=remediation-handler --user=remediation-handler
   $ sensuctl user create remediation-handler --password REPLACEME
   ```

2. Register the remediation handler asset

   ```yaml
   ---
   type: Asset
   api_version: core/v2
   metadata:
     name: sensu-go-remediation-handler
     namespace: default
   spec:
     url: https://github.com/calebhailey/sensu-go-remediation-handler/...
     sha512: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
   ```

   Save this definition to a file named `sensu-go-remediation-handler-asset.yaml`
   and run:

   ```
   $ sensuctl create -f sensu-go-remediation-handler-asset.yaml
   ```

3. Configure the handler

   ```yaml
   ---
   type: Handler
   api_version: core/v2
   metadata:
     name: remediation
     namespace: default
   spec:
     type: pipe
     command: sensu-go-remediation-handler
     timeout: 10
     runtime_assets:
     - sensu-go-remediation-handler
     env_vars:
     - "SENSU_BACKEND_HOST=127.0.0.1"
     - "SENSU_BACKEND_PORT=8080"
     - "SENSU_USER=remediation-handler"
     - "SENSU_PASS=REPLACEME"
   ```

   Save this definition to a file named `sensu-go-remediation-handler.yaml` and
   run:

   ```
   $ sensuctl create -f sensu-go-remediation-handler.yaml
   ```
