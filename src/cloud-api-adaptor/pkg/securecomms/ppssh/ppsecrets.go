package ppssh

import (
	"time"
)

type PpSecrets struct {
	keys      []string
	secrets   map[string][]byte
	getSecret GetSecret
}

type GetSecret func(name string) ([]byte, error)

func NewPpSecrets(getSecret GetSecret) *PpSecrets {
	return &PpSecrets{
		keys:      []string{},
		secrets:   make(map[string][]byte),
		getSecret: getSecret,
	}
}

func (fs *PpSecrets) AddKey(key string) {
	fs.keys = append(fs.keys, key)
}

func (fs *PpSecrets) GetKey(key string) []byte {
	return fs.secrets[key]
}

func (fs *PpSecrets) Go() {
	sleeptime := time.Duration(1)

	for len(fs.keys) > 0 {
		key := fs.keys[0]
		logger.Printf("PpSecrets obtaining key %s", key)

		data, err := fs.getSecret(key)
		if err == nil && len(data) > 0 {
			logger.Printf("PpSecrets %s success", key)
			fs.secrets[key] = data
			fs.keys = fs.keys[1:]
			continue
		}
		if err != nil {
			logger.Printf("PpSecrets %s getSecret err: %v", key, err)
		} else {
			logger.Printf("PpSecrets %s getSecret returned an empty secret", key)
		}

		time.Sleep(sleeptime * time.Second)
		sleeptime *= 2
		if sleeptime > 30 {
			sleeptime = 30
		}
	}
}
