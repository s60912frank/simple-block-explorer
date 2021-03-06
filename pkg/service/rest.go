package service

import (
	"context"
	"fmt"
	"net/http"
	"portto-explorer/pkg/config"
	"portto-explorer/pkg/database"
	"portto-explorer/pkg/model"
	"portto-explorer/pkg/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type WebServer struct {
	db  *database.Database
	srv *http.Server
}

func NewWebServer(db *database.Database) *WebServer {
	s := &WebServer{
		db: db,
	}
	r := gin.Default()
	w := utils.WrapperErr
	r.GET("/blocks", w(s.GetBlocksHandler))
	r.GET("/blocks/:id", w(s.GetBlockByIDHandler))
	r.GET("/transaction/:txHash", w(s.GetTransactionByHashHandler))

	serverConf := config.GetConfig().Server
	s.srv = &http.Server{
		Addr:    fmt.Sprintf("%s:%s", serverConf.Host, serverConf.Port),
		Handler: r,
	}

	return s
}

func (s *WebServer) Start() error {
	return s.srv.ListenAndServe()
}

func (s *WebServer) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

type GetBlocksQuery struct {
	Limit int `form:"limit,default=1"`
}

func (s *WebServer) GetBlocksHandler(c *gin.Context) (err error) {
	var q GetBlocksQuery
	err = c.ShouldBindQuery(&q)
	if err != nil {
		return
	}

	var blocks []*model.Block
	err = s.db.Tx(func(tx *gorm.DB) error {
		return tx.Order("number DESC").Limit(q.Limit).Find(&blocks).Error
	})
	if err != nil {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"blocks": blocks,
	})

	return
}

func (s *WebServer) GetBlockByIDHandler(c *gin.Context) (err error) {
	numOrHash := c.Param("id")
	if numOrHash == "" {
		err = fmt.Errorf("please specify block number or hash")
		return
	}

	var block *model.Block
	err = s.db.Tx(func(tx *gorm.DB) error {
		condition := "number = ?"
		_, parseIntErr := strconv.ParseUint(numOrHash, 10, 64)
		if parseIntErr != nil {
			condition = "hash = ?"
		}
		err := tx.Where(condition, numOrHash).First(&block).Error
		if err != nil {
			return err
		}
		return tx.
			Model(&model.Transaction{}).
			Select("hash").
			Where("ref_block_hash = ?", block.Hash).Find(&block.TransactionHash).Error
	})
	if err != nil {
		return
	}
	c.JSON(http.StatusOK, block)

	return
}

func (s *WebServer) GetTransactionByHashHandler(c *gin.Context) (err error) {
	hash := c.Param("txHash")
	if hash == "" {
		err = fmt.Errorf("please specify tx hash")
		return
	}

	var transaction *model.Transaction
	err = s.db.Tx(func(tx *gorm.DB) error {
		return tx.Where("hash = ?", hash).First(&transaction).Error
	})
	if err != nil {
		return
	}
	c.JSON(http.StatusOK, transaction)

	return
}
