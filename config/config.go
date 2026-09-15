package config

import (
	"os"
	"strings"

	"github.com/pkg/errors"
)

type Config struct {
	Dir  string
	Addr string

	Secret Secret
}

func Prod() (Config, error) {
	secret, err := readSecret("/home/ec2-user/var/line/secret")
	if err != nil {
		return Config{}, errors.Wrap(err, "")
	}
	cfg := Config{
		Dir:    "/home/ec2-user/var/line/data",
		Addr:   ":34282",
		Secret: secret,
	}
	return cfg, nil
}

func Dev() (Config, error) {
	secret, err := readSecret("./secret")
	if err != nil {
		return Config{}, errors.Wrap(err, "")
	}
	cfg := Config{
		Dir:    "data",
		Addr:   ":34282",
		Secret: secret,
	}
	return cfg, nil
}

type Secret struct {
	Password               string
	LineChannelSecret      string
	LineChannelAccessToken string
}

// readSecret reads an env-file style secret file of "KEY=VALUE" lines.
func readSecret(path string) (Secret, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Secret{}, errors.Wrap(err, "")
	}

	secret := Secret{}
	for line := range strings.SplitSeq(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "PASSWORD":
			secret.Password = v
		case "LINE_CHANNEL_SECRET":
			secret.LineChannelSecret = v
		case "LINE_CHANNEL_ACCESS_TOKEN":
			secret.LineChannelAccessToken = v
		}
	}
	return secret, nil
}
