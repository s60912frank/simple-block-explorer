package model

import (
	"github.com/ethereum/go-ethereum/common"
)

type Block struct {
	Hash         common.Hash    `json:"block_hash" gorm:"type:varchar(64); primarykey"`
	Number       uint64         `json:"block_num" gorm:"type:numeric; not null"`
	Timestamp    uint64         `json:"block_time" gorm:"type:numeric; not null"`
	ParentHash   common.Hash    `json:"parent_hash" gorm:"type:varchar(64)"` // TODO: nullable ok?
	Transactions []*Transaction `json:"transactions" gorm:"foreignKey:block_hash"`
}

func (b *Block) TableName() string {
	return "block"
}
