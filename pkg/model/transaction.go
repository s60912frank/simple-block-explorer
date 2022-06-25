package model

import (
	ethTypes "github.com/ethereum/go-ethereum/core/types"
)

type Transaction struct {
	Hash      string          `json:"tx_hash" gorm:"type:varchar(66); primarykey"`
	BlockHash string          `json:"block_hash" gorm:"type:varchar(66); not null"`
	From      string          `json:"from" gorm:"type:varchar(42); not null"`
	To        string          `json:"to" gorm:"type:varchar(42); not null"`
	Nonce     uint64          `json:"nonce" gorm:"type:numeric; not null"`
	Data      []byte          `json:"data" gorm:"type:text"`
	Value     uint64          `json:"value" gorm:"type:numeric; not null"` // TODO: bigint or unit64 ok?
	Logs      []*ethTypes.Log `json:"logs" gorm:"type:jsonb"`
}

func (t *Transaction) TableName() string {
	return "transaction"
}
