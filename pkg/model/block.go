package model

type Block struct {
	Hash            string   `json:"block_hash" gorm:"type:varchar(66); primarykey"`
	Number          uint64   `json:"block_num" gorm:"type:numeric; not null"`
	Timestamp       uint64   `json:"block_time" gorm:"type:numeric; not null"`
	ParentHash      string   `json:"parent_hash" gorm:"type:varchar(66); not null"`
	TransactionHash []string `json:"transactions,omitempty" gorm:"-"`
}

func (b *Block) TableName() string {
	return "block"
}
