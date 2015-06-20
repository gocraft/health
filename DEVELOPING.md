# Developing gocraft/health

Assumes you're in the health path:
```
cd $GOPATH/src/github.com/gocraft/health
```

## Testing

```
go test .
go test ./healthd
go test ./stack
go test ./sinks/bugsnag
go test ./sinks/prometheus
```

## Running healthd

```
HEALTHD_MONITORED_HOSTPORTS=:5020,:5021,:5022 go run ./cmd/healthd/main.go
```

This will monitor health instances on :5020, :5021, and :5022.

The JSON API and web pages are exposed on :5031 by default:
```
http://localhost:5031/healthd/aggregations
http://localhost:5031/healthd/aggregations/overall
http://localhost:5031/healthd/jobs
http://localhost:5031/healthd/hosts
```

Health metrics of healthd itself are exposed on :5030

## Assets

```
go get -u github.com/jteeuwen/go-bindata/...
cd healthd
go-bindata -o ui_assets.go -pkg healthd ui
```


## Running some fake servers