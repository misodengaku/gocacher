package main

import "time"

type Config struct {
	FsRoot             string `yaml:"fs_root"`
	CacheTTL           int    `yaml:"cache_ttl"`
	maxWorkers         int    `yaml:"max_workers"`
	maxQueues          int    `yaml:"max_queues"`
	ListenAddr         string `yaml:"listen_addr"`
	PromHTTPListenAddr string `yaml:"promhttp_listen_addr"`
	RedisAddr          string `yaml:"redis_addr"`
}

type NginxCompatibleFileInfo struct {
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	ModifiedTime    string    `json:"mtime"`
	ModifiedTimeRaw time.Time `json:"-"`
	Size            int64     `json:"size,omitempty"`
}
