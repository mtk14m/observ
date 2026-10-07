package main

import (
	"io"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestConfigDefaults(t *testing.T) {
	c, err := parseConfig(nil, env(nil), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.storage != "fs" || c.dataDir != "./data" || c.retention != 7*24*time.Hour ||
		c.otlpGRPCAddr != ":4317" || c.otlpHTTPAddr != ":4318" || c.httpAddr != ":8080" ||
		c.publicURL != "http://localhost:8080" || c.alertInterval != 30*time.Second {
		t.Errorf("defaults = %+v", c)
	}
}

func TestEnvironmentConfiguresUnsetFlags(t *testing.T) {
	c, err := parseConfig([]string{"-retention", "24h"}, env(map[string]string{
		"OBSRV_RETENTION":   "1h", // the flag wins
		"OBSRV_STORAGE":     "s3",
		"OBSRV_S3_BUCKET":   "telemetry",
		"OBSRV_S3_INSECURE": "true",
	}), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.retention != 24*time.Hour || c.storage != "s3" || c.s3.Bucket != "telemetry" || !c.s3.Insecure {
		t.Errorf("config = %+v", c)
	}
}

func TestConfigErrors(t *testing.T) {
	cases := map[string]struct {
		args []string
		env  map[string]string
	}{
		"unknown storage":     {[]string{"-storage", "ftp"}, nil},
		"s3 without bucket":   {[]string{"-storage", "s3"}, nil},
		"invalid env value":   {nil, map[string]string{"OBSRV_RETENTION": "soon"}},
		"unknown flag":        {[]string{"-nope"}, nil},
		"negative cache size": {[]string{"-cache-size-mb", "-1"}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseConfig(tc.args, env(tc.env), io.Discard); err == nil {
				t.Error("parseConfig returned nil error")
			}
		})
	}
}
