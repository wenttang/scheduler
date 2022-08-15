package config

import (
	"github.com/kelseyhightower/envconfig"
)

// Config janitor config
type Config struct {
	// Addr optionally specifies the TCP address for the server to listen on,
	// in the form "host:port". If empty, ":http" (port 80) is used.
	// The service names are defined in RFC 6335 and assigned by IANA.
	// See net.Dial for details of the address format.
	Addr string `envconfig:"ADDR" default:":80"`

	// log level debug、info、warn、error. default error
	LogLevel string `envconfig:"LOG_LEVEL" default:"debug"`

	// Middleware pre or post chain
	Middleware *Middleware `envconfig:"MIDDLEWARE" default:"debug"`
}

type Middleware struct {
	Pre  []string `envconfig:"PRE"`
	Post []string `envconfig:"POST"`
}

func Get() (Config, error) {
	var config Config
	err := envconfig.Process("", &config)
	if err != nil {
		return config, err
	}
	return config, nil
}
