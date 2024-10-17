package ppssh

import (
	"time"
)

type PpSecrets struct {
	secrets   map[string][]byte
	getSecret GetSecret
}

type GetSecret func(name string) ([]byte, error)

func NewPpSecrets(getSecret GetSecret) *PpSecrets {
	return &PpSecrets{
		secrets:   make(map[string][]byte),
		getSecret: getSecret,
	}
}

func (sec *PpSecrets) AddKey(key string) {
	if _, ok := sec.secrets[key]; ok {
		return
	}
	sec.secrets[key] = nil
}

func (sec *PpSecrets) GetKey(key string) []byte {
	return sec.secrets[key]
}

func (sec *PpSecrets) SetKey(key string, keydata []byte) {
	sec.secrets[key] = keydata
}

func (sec *PpSecrets) Go() {
	sleeptime := time.Duration(1)

	for key, keydata := range sec.secrets {
		if keydata != nil {
			continue
		}
		logger.Printf("PpSecrets obtaining key %s", key)

		// loop until we get a valid key
		for {
			keydata, err := sec.getSecret(key)
			if err == nil && len(keydata) > 0 {
				logger.Printf("PpSecrets %s success", key)
				sec.secrets[key] = keydata
				break
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
}
