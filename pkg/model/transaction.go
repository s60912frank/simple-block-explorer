package model

import (
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
)

type Transaction struct {
	Hash      common.Hash     `json:"tx_hash" gorm:"type:varchar(64); primarykey"`
	BlockHash common.Hash     `json:"block_hash" gorm:"type:varchar(64); not null"`
	From      common.Address  `json:"from" gorm:"type:varchar(40); not null"`
	To        common.Address  `json:"to" gorm:"type:varchar(40); not null"`
	Nonce     uint64          `json:"nonce" gorm:"type:numeric; not null"`
	Data      []byte          `json:"data" gorm:"type:text"`
	Value     uint64          `json:"value" gorm:"type:numeric; not null"` // TODO: bigint or unit64 ok?
	Logs      []*ethTypes.Log `json:"logs" gorm:"type:jsonb"`
}

func (t *Transaction) TableName() string {
	return "transaction"
}
