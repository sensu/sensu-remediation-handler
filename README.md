# Sensu Remediation Handler

[![Bonsai Asset Badge](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/sensu/sensu-remediation-handler) [![Build Status](https://travis-ci.org/sensu/sensu-remediation-handler.svg?branch=master)](https://travis-ci.org/sensu/sensu-remediation-handler)

- [Overview](#overview)
- [Configuration](#configuration)
  - [Environment Variables](#environment-variables)
  - [Annotations](#annotations)
    - [Annotation Specification](#annotation-specification)
    - [Remediation Action Specification](#remediation-action-specification)
- [Setup](#setup)
- [Examples](#examples)
  - [Example "Unscheduled" Check (Remediation Action)](#example-unscheduled-check-remediation-action)
  - [Example Check Definition and Remediation Request Configuration](#example-check-definition-and-remediation-request-configuration)
- [Acknowledgements](#acknowledgements)

## Overview

The Sensu Remediation Handler is a [Sensu Go event handler][1]
that enables you to build self-healing workflows in Sensu. 

The Sensu Remediation Handler &ndash; and other similar self-healing
workflows in Sensu &ndash; combine a few Sensu features:

- An "unscheduled check" configuration (i.e. a Sensu check with the `"publish":
  false` attribute set).
- The Sensu agent's built-in entity subscriptions (e.g. `entity:web-server-01`),
  which make it possible to target a single agent with a check execution
  request.
- The Sensu Checks API `POST /checks/:check/execute` endpoint, which allows
 this handler to issue ad hoc check requests.

## Configuration

### Environment Variables

The Sensu Remediation Handler does not honor any command line flags. Instead, it requires environment variables, either in the handler definition or in the Sensu backend service environment.

SENSU_API_URL        | |
---------------------|-------------------------------
description          | URL for Sensu backend, including scheme, hostname or IP address, and port.
required             | false
type                 | String
default              | http://127.0.0.1:8080
example              | `SENSU_API_URL=http://sensu.example.com:8080`

SENSU_USER           | |
---------------------|-------------------------------
description          | Username for the Sensu API.
required             | true
type                 | String
example              | `SENSU_API_USER=remediation-handler`

SENSU_PASS           | |
---------------------|-------------------------------
description          | Password for the Sensu API.
required             | true
type                 | String
example              | `SENSU_API_PASS=setecastronomy`

SENSU_API_CERT_FILE  | |
---------------------|-------------------------------
description          | Filesystem path to certificate authority (CA) certificate used to validate https Sensu API connections.
required             | false
type                 | String
example              | `SENSU_API_CERT_FILE=/etc/sensu/cacert.pem`

### Annotations

Although environment variables provide connection details for the Sensu API, you'll use the `io.sensu.remediation.config.actions` check annotation to provide the configuration that defines remediation activities.

#### Annotation Specification

The Sensu Remediation Handler uses the string value of the `io.sensu.remediation.config.actions` check annotation to determine which remediation actions, if any, should be scheduled for a given event.

When present, the value of the `io.sensu.remediation.config.actions` check annotation must be a array of objects containing key/value pairs. Each object element in the array must conform to the remediation action specification.

#### Remediation Action Specification

description   | |
--------------|------------------------------------------------------------
description   | A human-readable representation of the remediation action.
required      | false
type          | String
example       | `"description": "restart failed ntpd service"`

request       | |
--------------|-----------------------------------------------------------------
description   | The name of the check to be scheduled by the remediation action.
required      | true
type          | String
example       | `"request": "remediate-ntpd-service"`

occurrences   | |
--------------|-------------------------------
description   | A list of occurrence counts at which the remediation action is triggered.
required      | true
type          | Array of integers
example       | `"occurrences": [4,14,42]`

severities    | |
--------------|-------------------------------
description   | A list of [check status severities][2] that are allowed for the remediation action.
required      | true
type          | Array of integers
example       | `"severities": [1]`

subscriptions | |
--------------|-------------------------------
description   | A list of agent subscriptions for targeting remediation actions.
required      | true
type          | Array of strings
example       | `"subscriptions": ["ntpd"]`

## Setup

1. Create a dedicated Sensu user and role for the remediation handler.

   ```shell
   sensuctl role create remediation-handler --namespace=default --verb=create,update --resource checks
   sensuctl role-binding create remediation-handler --role=remediation-handler --user=remediation-handler
   sensuctl user create remediation-handler --password REPLACEME
   ```

2. Register the remediation handler asset.

   ```shell
   sensuctl asset add sensu/sensu-remediation-handler --rename sensu-remediation-handler
   ```

3. Configure the remediation handler.

   ```yaml
   ---
   type: Handler
   api_version: core/v2
   metadata:
     name: remediation
     namespace: default
   spec:
     type: pipe
     command: sensu-remediation-handler
     timeout: 10
     runtime_assets:
     - sensu-remediation-handler
     env_vars:
     - "SENSU_API_URL=http://127.0.0.1:8080"
     - "SENSU_API_CERT_FILE="
     - "SENSU_API_USER=remediation-handler"
     - "SENSU_API_PASS=REPLACEME"
   ```

   Save this definition to a file named `sensu-remediation-handler.yaml` and
   run:

   ```shell
   sensuctl create -f sensu-remediation-handler.yaml
   ```

## Examples

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
  interval: 10 # interval is required but not used
  subscriptions: []
```

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
    io.sensu.remediation.config.actions: |
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
  publish: true
  interval: 10
  handlers:
  - remediation
  subscriptions:
  - nginx
```

## Acknowledgements

This handler implements a pattern first implemented in [Nick Stielau's Sensu Remediator][3] circa 2012. Thanks, Nick!

[1]: https://docs.sensu.io/sensu-go/latest/reference/handlers/
[2]: https://docs.sensu.io/sensu-go/latest/reference/checks/#check-result-specification
[3]: https://github.com/sensu-plugins/sensu-plugins-sensu/blob/master/bin/handler-sensu.rb
