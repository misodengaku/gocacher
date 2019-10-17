package main

type Config struct {
	FsRoot     string `yaml:"fs_root"`
	CacheTTL   int    `yaml:"cache_ttl"`
	maxWorkers int    `yaml:"max_workers"`
	maxQueues  int    `yaml:"max_queues"`
	ListenAddr string `yaml:"listen_addr"`
	RedisAddr  string `yaml:"redis_addr"`
}