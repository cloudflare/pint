package promapi

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type Prometheus struct {
	name    string
	uri     string
	api     v1.API
	timeout time.Duration
	cache   *lru.Cache
	lock    *partitionLocker
}

func NewPrometheus(name, uri string, timeout time.Duration) *Prometheus {
	client, err := api.NewClient(api.Config{Address: uri})
	if err != nil {
		// config validation should prevent this from ever happening
		// panic so we don't need to return an error and it's easier to
		// use this code in tests
		panic(err)
	}
	cache, _ := lru.New(1000)
	return &Prometheus{
		name:    name,
		uri:     uri,
		api:     v1.NewAPI(client),
		timeout: timeout,
		cache:   cache,
		lock:    newPartitionLocker((&sync.Mutex{})),
	}
}
