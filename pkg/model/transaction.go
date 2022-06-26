package model

import (
	ethTypes "github.com/ethereum/go-ethereum/core/types"
)

type Transaction struct {
	Hash         string          `json:"tx_hash" gorm:"type:varchar(66); primarykey"`
	RefBlockHash string          `json:"block_hash" gorm:"type:varchar(66); index:idx_tx_ref_block_hash; not null"`
	From         string          `json:"from" gorm:"type:varchar(42); not null"`
	To           *string         `json:"to" gorm:"type:varchar(42);"`
	Nonce        uint64          `json:"nonce" gorm:"type:numeric; not null"`
	Data         string          `json:"data" gorm:"type:text"`
	Value        uint64          `json:"value" gorm:"type:numeric; not null"` // TODO: bigint or unit64 ok?
	Logs         []*ethTypes.Log `json:"logs" gorm:"type:jsonb"`
	ReceiptReady bool            `json:"receipt_ready" grom:"type:boolean"`
}

func (t *Transaction) TableName() string {
	return "transaction"
}
