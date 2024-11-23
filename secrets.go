package main

import (
	"fmt"
)

func (a *app) getSecrets() ([]*secret, error) {
	// Unmarshal the secrets into a slice of secret structs
	secrets := make([]*secret, 0)
	if err := a.config.UnmarshalKey("secrets", &secrets); err != nil {
		return nil, fmt.Errorf("error unmarshalling secrets: %w", err)
	}

	return secrets, nil
}
