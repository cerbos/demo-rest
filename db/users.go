// Copyright 2021 Zenauth Ltd.

package db

import (
	"context"
)

type UserRecord struct {
	PasswordHash []byte
	Roles        []string
	Aisles       []string
}

var users = map[string]*UserRecord{
	"adam": {
		PasswordHash: []byte(`$2y$10$MwXSJvIe8ATJpUnAz0dmgOYLBufw8uqhgMQoxBmGfxzT1hqPVxedK`),
		Roles:        []string{"customer"},
	},

	"bella": {
		PasswordHash: []byte(`$2y$10$T8Bie6zxL9eG2dF.w4sDZORGZ01AheI7WSwkBlOGim7DryKv.FGHq`),
		Roles:        []string{"customer", "employee", "manager"},
	},

	"charlie": {
		PasswordHash: []byte(`$2y$10$GQK9vyCt6iHKHy1xyJd9ne/I.Dzz4qv9FZrsDkzToKF18xFIzNaqG`),
		Roles:        []string{"customer", "employee", "picker"},
	},

	"diana": {
		PasswordHash: []byte(`$2y$10$ZN.44qmBcrPkFPT17MtT5OK2wDPaxJ0XTSSdOwirjeQVZrtCfZWtG`),
		Roles:        []string{"customer", "employee", "dispatcher"},
	},

	"eve": {
		PasswordHash: []byte(`$2y$10$NZfCN94K3.k0ajk60VITieXVoLSvEqYu4zTZMQWVfTiwnAcZyBw9.`),
		Roles:        []string{"customer"},
	},

	"florence": {
		PasswordHash: []byte(`$2y$10$fUIzHVFti/BUKEHGf762EOTyemMuyXHL.zz09XykDMz0461YkUiZC`),
		Roles:        []string{"customer", "employee", "buyer"},
		Aisles:       []string{"bakery"},
	},

	"george": {
		PasswordHash: []byte(`$2y$10$waNQTHas52rXDrqOGhj4Du3VE0u.jdeE9I2orA9LUdVo/UVoQxEn6`),
		Roles:        []string{"customer", "employee", "buyer"},
		Aisles:       []string{"dairy"},
	},

	"harry": {
		PasswordHash: []byte(`$2y$10$RlG.ImP3eXj6f1TBXw8.8.CEsa4Kc5nF3DQZFLQlc0Ecy4bYjlEQu`),
		Roles:        []string{"customer", "employee", "stocker"},
	},

	"jenny": {
		PasswordHash: []byte(`$2y$10$OLQ6pYrxNm4eJJWTmit8wuNq3FpBCWSD82MuzGmDMOozvtU.eXCEa`),
		Roles:        []string{"customer", "employee", "stocker"},
	},
}

// LookupUser retrieves the record for the given username from the database.
func LookupUser(ctx context.Context, userName string) (*UserRecord, error) {
	rec, ok := users[userName]
	if !ok {
		return nil, ErrNotFound
	}

	return rec, nil
}
