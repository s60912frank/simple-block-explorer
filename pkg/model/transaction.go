package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/shopspring/decimal"
)

type DBTxLogs struct {
	Logs []*ethTypes.Log
}

func (l *DBTxLogs) Value() (driver.Value, error) {
	return json.Marshal(l.Logs)
}

func (l *DBTxLogs) Scan(value interface{}) error {
	// if err := json.Unmarshal(value.([]byte), &l.Logs); err != nil {
	// 	return err
	// }
	if v, ok := value.([]*types.Log); !ok {
		l.Logs = v
		return nil
	}
	return fmt.Errorf("value is not log")
}

func (l *DBTxLogs) MarshalJSON() ([]byte, error) {
	return json.Marshal(&l.Logs)
}

func (l *DBTxLogs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &l.Logs)
}

type Transaction struct {
	Hash         string          `json:"tx_hash" gorm:"type:varchar(66); primarykey"`
	RefBlockHash string          `json:"block_hash" gorm:"type:varchar(66); index:idx_tx_ref_block_hash; not null"`
	From         string          `json:"from" gorm:"type:varchar(42); not null"`
	To           *string         `json:"to" gorm:"type:varchar(42);"`
	Nonce        uint64          `json:"nonce" gorm:"type:numeric; not null"`
	Data         string          `json:"data" gorm:"type:text"`
	Value        decimal.Decimal `json:"value" gorm:"type:decimal; not null"`
	Logs         DBTxLogs        `json:"logs" gorm:"type:jsonb"`
	ReceiptReady bool            `json:"receipt_ready" gorm:"type:boolean"`
}

func (t *Transaction) TableName() string {
	return "transaction"
}
