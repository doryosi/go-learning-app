package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Address                                    string
	ReadTimeout, WriteTimeout, ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("PORT must be a number between 1 and 65535")
	}
	return Config{Address: ":" + port, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, ShutdownTimeout: 10 * time.Second}, nil
}
