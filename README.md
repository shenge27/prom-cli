# prom-cli
---

The `prom-cli` command provides cli interfaces to [Prometheus's Remote Read](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_read) server. The tool was created to aid debugging servers that implement remote_read api.

### Installation

```
go get github.com/shenge27/prom-cli/cmd/prom-cli
```

### Usage

```
prom-cli --url http://127.0.0.1:1234/api/v1/read --token bearertoken --input /tmp/request.json
```

### Debug

The `prom-cli debug` can be used to digest and display as json recorder http request/response pairs.

```
prom-cli debug -i /tmp/service/service_namespace/trace_id.json
```
